// Package tui e' o console do agente: fluxo de abertura e status ao vivo.
package tui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/GabMalta/gado-online-agente/internal/backend"
	"github.com/GabMalta/gado-online-agente/internal/config"
)

// ErrCancelado e' devolvido quando o operador fecha o agente no prompt.
var ErrCancelado = errors.New("cancelado pelo operador")

// esperaEntreTentativas: intervalo entre tentativas de alcancar o servidor na
// abertura. Curto o bastante para o operador nao achar que travou.
var esperaEntreTentativas = 5 * time.Second

// Prompt conduz o fluxo de abertura: login, software e leilao.
type Prompt struct {
	Entrada io.Reader
	Saida   io.Writer

	// NovoCliente e' injetado para o teste nao precisar de rede real.
	NovoCliente func(urlBase string) ClienteAbertura

	leitor *bufio.Reader
}

// ClienteAbertura e' o que o prompt precisa do backend.
type ClienteAbertura interface {
	Entrar(usuario, senha string) error
	ListarLeiloesAtivos() ([]backend.Leilao, error)
	ObterChave(leilaoID string) (string, error)
}

// Abertura e' o resultado do prompt: o que o laco precisa para comecar.
type Abertura struct {
	Leilao   backend.Leilao
	Chave    string
	Software string
}

var softwares = []struct{ codigo, nome string }{
	{config.SoftwareVMix, "vMix"},
	{config.SoftwareOBS, "OBS"},
}

// Resolver pede o login, o software e o leilao, e devolve a chave de
// transmissao do leilao escolhido.
//
// O operador abre o agente com dois cliques no .exe: nao ha flag nem chave
// para colar. Tudo se escolhe com as setas e Enter.
func (p *Prompt) Resolver(cfg config.Config) (config.Config, Abertura, error) {
	p.leitor = bufio.NewReader(p.Entrada)
	cliente := p.NovoCliente(cfg.URLBase)

	p.linha("")
	p.linha("  " + forte("Gado Online — agente de transmissão"))

	// O servidor e o arquivo na tela: trocar o dominio (homologacao, servidor
	// novo) e' editar `url_base` nesse arquivo, e o operador nao tem como
	// adivinhar onde fica o %APPDATA%.
	p.linha("  " + cinza("Servidor:      "+cfg.URLBase))
	if caminho, err := config.Caminho(); err == nil {
		p.linha("  " + cinza("Configuração:  "+caminho))
	}

	usuario, err := p.entrar(cliente, cfg.Usuario)
	if err != nil {
		return cfg, Abertura{}, err
	}

	software, err := p.escolherSoftware(cfg.Software)
	if err != nil {
		return cfg, Abertura{}, err
	}

	leilao, err := p.escolherLeilao(cliente)
	if err != nil {
		return cfg, Abertura{}, err
	}

	var chave string
	err = p.insistir(cfg.URLBase, func() (err error) {
		chave, err = cliente.ObterChave(leilao.ID)
		return err
	})
	if err != nil {
		return cfg, Abertura{}, err
	}

	cfg.Usuario = usuario
	cfg.Software = software
	if err := config.Salvar(cfg); err != nil {
		p.aviso("não consegui salvar a configuração: " + err.Error())
	}

	return cfg, Abertura{Leilao: leilao, Chave: chave, Software: software}, nil
}

// entrar pede usuario e senha ate o login passar. Devolve o usuario usado.
func (p *Prompt) entrar(cliente ClienteAbertura, ultimo string) (string, error) {
	p.linha("")
	p.linha("  Entre com o seu usuário do sistema.")

	for {
		digitado, err := p.lerCampo("Usuário", ultimo, false)
		if err != nil {
			return "", err
		}
		usuario := strings.TrimSpace(digitado)
		// Na nova tentativa o usuario vem preenchido: quase sempre o erro foi
		// na senha.
		ultimo = usuario

		senha, err := p.lerCampo("Senha", "", true)
		if err != nil {
			return "", err
		}

		if usuario == "" || senha == "" {
			p.aviso("Preencha o usuário e a senha.")
			continue
		}

		err = p.insistir("", func() error { return cliente.Entrar(usuario, senha) })
		if errors.Is(err, backend.ErrLoginInvalido) {
			p.aviso("Usuário ou senha inválidos. Tente de novo.")
			continue
		}
		if err != nil {
			return "", err
		}
		return usuario, nil
	}
}

func (p *Prompt) escolherSoftware(ultimo string) (string, error) {
	nomes := make([]string, len(softwares))
	inicial := 0
	for i, software := range softwares {
		nomes[i] = software.nome
		if software.codigo == ultimo {
			inicial = i
		}
	}

	i, err := p.selecionar("Qual programa de transmissão?", nomes, inicial)
	if err != nil {
		return "", err
	}
	return softwares[i].codigo, nil
}

// escolherLeilao lista os leiloes aguardando ou em andamento da empresa.
func (p *Prompt) escolherLeilao(cliente ClienteAbertura) (backend.Leilao, error) {
	for {
		var leiloes []backend.Leilao
		err := p.insistir("", func() (err error) {
			leiloes, err = cliente.ListarLeiloesAtivos()
			return err
		})
		if err != nil {
			return backend.Leilao{}, err
		}

		if len(leiloes) == 0 {
			// Lista vazia nao fecha o agente: o operador pode estar criando o
			// leilao no sistema agora, ou reabrindo um que ja tinha encerrado.
			p.linha("")
			p.aviso("Nenhum leilão aguardando ou em andamento.")
			p.linha("     Cadastre o leilão no sistema (ou ajuste o status) e atualize.")

			i, err := p.selecionar("O que fazer?", []string{"Atualizar a lista", "Sair"}, 0)
			if err != nil {
				return backend.Leilao{}, err
			}
			if i == 1 {
				return backend.Leilao{}, ErrCancelado
			}
			continue
		}

		opcoes := make([]string, len(leiloes))
		for i, leilao := range leiloes {
			opcoes[i] = descreverLeilao(leilao)
		}

		i, err := p.selecionar("Qual leilão?", opcoes, 0)
		if err != nil {
			return backend.Leilao{}, err
		}
		return leiloes[i], nil
	}
}

func descreverLeilao(leilao backend.Leilao) string {
	status := "Aguardando"
	if leilao.Status == backend.StatusEmAndamento {
		status = "Em andamento"
	}

	lotes := fmt.Sprintf("%d lotes", leilao.TotalLotes)
	if leilao.TotalLotes == 1 {
		lotes = "1 lote"
	}

	return fmt.Sprintf("%s — %s — %s — %s",
		leilao.Nome, formatarData(leilao.DataLeilao), status, lotes)
}

// insistir repete a acao enquanto o servidor nao responde.
//
// Desistir seria pior: na maquina de um parque de exposicoes a internet demora
// a subir, e o operador ficaria clicando duas vezes no .exe de novo e de novo.
// Entao insiste, dizendo o que esta acontecendo. Ctrl+C sai. Erro que o
// servidor devolveu (senha errada, por exemplo) volta na hora.
func (p *Prompt) insistir(urlBase string, acao func() error) error {
	for {
		err := acao()

		if errors.Is(err, backend.ErrRotaDesconhecida) {
			p.linha("")
			p.aviso("O servidor respondeu, mas não conhece as rotas de transmissão.")
			if urlBase != "" {
				p.linha("     " + cinza(urlBase))
			}
			p.linha("     Provavelmente está numa versão anterior a este agente.")
			p.linha("     Avise quem cuida do sistema.")
			return err
		}

		var conexao *backend.ErroConexao
		if !errors.As(err, &conexao) {
			return err
		}

		p.linha("")
		p.aviso("Não consegui falar com o servidor.")
		p.linha("     " + cinza(err.Error()))
		p.linha("     " + cinza(fmt.Sprintf("Tentando de novo em %s... (Ctrl+C para sair)", esperaEntreTentativas)))
		time.Sleep(esperaEntreTentativas)
	}
}

// lerCampo le uma linha de texto. `oculto` mostra asteriscos no lugar das
// letras (senha); `inicial` vem preenchido e pode ser apagado.
func (p *Prompt) lerCampo(rotulo, inicial string, oculto bool) (string, error) {
	restaurar, cru := p.modoCru()
	if !cru {
		// Sem terminal nao da para esconder a senha nem pre-preencher: o padrao
		// aparece entre colchetes e Enter vazio fica com ele.
		if inicial != "" {
			p.escrever(fmt.Sprintf("  %s [%s]: ", rotulo, inicial))
		} else {
			p.escrever("  " + rotulo + ": ")
		}

		texto, err := ler(p.leitor)
		if err != nil {
			return "", err
		}
		if texto == "" {
			texto = inicial
		}
		return texto, nil
	}
	defer restaurar()

	texto := []rune(inicial)
	p.escrever("  " + rotulo + ": " + mascarar(string(texto), oculto))

	for {
		ev, err := lerTecla(p.leitor)
		if err != nil {
			p.linha("")
			return "", ErrCancelado
		}

		switch ev.tecla {
		case teclaEnter:
			p.linha("")
			return string(texto), nil
		case teclaCancelar:
			p.linha("")
			return "", ErrCancelado
		case teclaApagar:
			if len(texto) > 0 {
				texto = texto[:len(texto)-1]
				p.escrever("\b \b")
			}
		case teclaTexto:
			texto = append(texto, ev.letra)
			p.escrever(mascarar(string(ev.letra), oculto))
		}
	}
}

func mascarar(texto string, oculto bool) string {
	if !oculto {
		return texto
	}
	return strings.Repeat("*", len([]rune(texto)))
}

// modoCru liga o modo cru quando a entrada e' o console de verdade. Entrada de
// teste (strings.Reader) ou de pipe devolve ok=false.
func (p *Prompt) modoCru() (func(), bool) {
	arquivo, ok := p.Entrada.(*os.File)
	if !ok {
		return nil, false
	}
	return entrarModoCru(arquivo)
}

func ler(leitor *bufio.Reader) (string, error) {
	linha, err := leitor.ReadString('\n')
	texto := strings.TrimSpace(linha)

	// EOF com texto ainda e' uma resposta valida (entrada vinda de pipe).
	if err != nil && texto == "" {
		return "", ErrCancelado
	}
	return texto, nil
}

func (p *Prompt) linha(texto string) { fmt.Fprintln(p.Saida, texto) }

func (p *Prompt) escrever(texto string) { fmt.Fprint(p.Saida, texto) }

func (p *Prompt) aviso(texto string) {
	fmt.Fprintln(p.Saida, "  "+amarelo("⚠  "+texto))
}

func formatarData(dataISO string) string {
	data, err := time.Parse("2006-01-02", dataISO)
	if err != nil {
		return dataISO
	}
	return data.Format("02/01/2006")
}

// PausarAoSair segura a janela aberta quando o agente foi aberto com dois
// cliques. Sem isso, um erro na abertura fecha o console antes de o operador
// conseguir ler a mensagem.
func PausarAoSair(entrada io.Reader, saida io.Writer) {
	if !pausarAoSair() {
		return
	}
	fmt.Fprintln(saida, "")
	fmt.Fprint(saida, "  Pressione Enter para fechar.")
	_, _ = bufio.NewReader(entrada).ReadString('\n')
}
