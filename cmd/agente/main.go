// Agente de transmissão do Gado Online.
//
// Lê o overlay do vMix ou do OBS na máquina do operador e informa ao backend
// qual lote está em pista. Processo separado do software de transmissão de
// propósito: script travado trava o vMix no meio do leilão.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GabMalta/gado-online-agente/internal/backend"
	"github.com/GabMalta/gado-online-agente/internal/config"
	"github.com/GabMalta/gado-online-agente/internal/fonte"
	"github.com/GabMalta/gado-online-agente/internal/laco"
	"github.com/GabMalta/gado-online-agente/internal/tui"
)

// versao e' preenchida no build com -ldflags "-X main.versao=1.0.0".
var versao = "dev"

// O operador abre o agente com dois cliques no .exe, entao nada aqui pode
// depender de flag: login, programa de transmissao e leilao se escolhem no
// console. As flags que sobraram sao de desenvolvimento.
func main() {
	simular := flag.String("simular", "",
		"lê o estado de um arquivo JSON em vez do vMix/OBS (desenvolvimento e demonstração)")
	servidor := flag.String("servidor", "", "URL do backend (sobrescreve a configuração salva)")
	intervalo := flag.Duration("intervalo", laco.IntervaloPadrao,
		"intervalo entre leituras do overlay")
	flag.Parse()

	err := rodar(*simular, *servidor, *intervalo)
	if err == nil || errors.Is(err, tui.ErrCancelado) {
		return
	}

	fmt.Fprintf(os.Stderr, "\nerro: %v\n", err)
	tui.PausarAoSair(os.Stdin, os.Stdout)
	os.Exit(1)
}

func rodar(simular string, servidor string, intervalo time.Duration) error {
	// Antes do `--servidor`: o arquivo nasce com o servidor padrao. Falhar aqui
	// nao impede o agente de rodar com a config em memoria.
	_ = config.CriarSeAusente()

	cfg, err := config.Carregar()
	if err != nil && !errors.Is(err, config.ErrSemConfig) {
		return err
	}
	if servidor != "" {
		cfg.URLBase = servidor
	}

	prompt := &tui.Prompt{
		Entrada: os.Stdin,
		Saida:   os.Stdout,
		NovoCliente: func(urlBase string) tui.ClienteAbertura {
			return backend.NovoClienteUsuario(urlBase)
		},
	}

	cfg, abertura, err := prompt.Resolver(cfg)
	if err != nil {
		return err
	}

	f, err := abrirFonte(cfg, simular, abertura.Software)
	if err != nil {
		return err
	}
	defer f.Fechar()

	status := &tui.Status{
		Saida:   os.Stdout,
		Versao:  versao,
		Fonte:   f.Nome(),
		Detalhe: f.Descricao(),
		Leilao:  abertura.Leilao.Nome,
	}

	fmt.Println()
	cliente := backend.NovoCliente(cfg.URLBase, abertura.Chave)
	ciclo := laco.Novo(f, cliente, intervalo, status.Desenhar)

	// Ctrl+C libera a pista antes de sair: o operador fechando o agente no fim
	// do leilão não deve deixar o último lote anunciado esperando o TTL.
	parar := make(chan struct{})
	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sinais
		close(parar)
	}()

	erroLaco := ciclo.Rodar(parar)

	if errors.Is(erroLaco, backend.ErrChaveRecusada) {
		// Alguem gerou chave nova ou revogou esta no sistema. Reabrir o agente
		// busca a chave atual do leilao.
		return errors.New("a chave de transmissão foi recusada pelo servidor " +
			"(foi revogada no sistema?). Abra o agente de novo")
	}
	if erroLaco != nil {
		return erroLaco
	}

	liberarNaSaida(cliente, status)
	return nil
}

func abrirFonte(cfg config.Config, simular string, software string) (fonte.Fonte, error) {
	switch {
	case simular != "":
		return fonte.NovoSimulado(simular), nil
	case software == config.SoftwareOBS:
		return fonte.NovoOBS(cfg.OBSEndereco, cfg.OBSSenha, cfg.OBSCampos)
	default:
		return fonte.NovoVMix(cfg.VMixTitle, cfg.VMixCampos), nil
	}
}

// liberarNaSaida é cortesia, não garantia: se falhar, o TTL de 120s do backend
// cobre. Por isso o erro só é informado, não propagado.
func liberarNaSaida(cliente *backend.Cliente, status *tui.Status) {
	status.Mensagem("")
	status.Mensagem("  Liberando a pista...")

	if err := cliente.LiberarPista(); err != nil {
		status.Mensagem("  Não consegui liberar a pista (" + err.Error() + ").")
		status.Mensagem("  Ela expira sozinha em até 2 minutos.")
		return
	}
	status.Mensagem("  Pista liberada. Até o próximo leilão.")
}
