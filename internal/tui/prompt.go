// Package tui e' o console do agente: fluxo de abertura e status ao vivo.
package tui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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

// Prompt conduz o fluxo de abertura.
type Prompt struct {
	Entrada io.Reader
	Saida   io.Writer

	// NovoCliente e' injetado para o teste nao precisar de rede real.
	NovoCliente func(urlBase, chave string) ClienteLeilao

	// Hoje e' injetavel para testar o aviso de leilao antigo.
	Hoje func() time.Time
}

// ClienteLeilao e' o que o prompt precisa do backend.
type ClienteLeilao interface {
	BuscarLeilao() (backend.Leilao, error)
}

// Resolver devolve a config pronta para uso, com chave validada contra o
// backend e confirmada pelo operador.
//
// Tres caminhos: sem chave salva, chave de leilao de hoje, e chave de leilao
// antigo. O terceiro e' o que existe porque `criar_chave` so revoga as chaves
// do *mesmo* leilao: a credencial do mes passado continua valida, e sem aviso o
// operador passaria o leilao de hoje alimentando a pagina de agosto.
func (p *Prompt) Resolver(cfg config.Config) (config.Config, backend.Leilao, error) {
	leitor := bufio.NewReader(p.Entrada)

	for {
		if !cfg.TemChave() {
			chave, err := p.pedirChave(leitor, "")
			if err != nil {
				return cfg, backend.Leilao{}, err
			}
			cfg.Chave = chave
		}

		leilao, err := p.NovoCliente(cfg.URLBase, cfg.Chave).BuscarLeilao()

		if errors.Is(err, backend.ErrChaveRecusada) {
			// Credencial revogada nao tem motivo para continuar no disco.
			cfg.Chave = ""
			_ = config.EsquecerChave()
			p.linha("")
			p.aviso("A chave salva não vale mais (foi revogada ou o leilão foi encerrado).")
			continue
		}

		if err != nil {
			// Validar a chave antes de comecar continua obrigatorio -- sem saber
			// de que leilao ela e', nao da' para comecar com seguranca. Mas
			// desistir seria pior: na maquina de um parque de exposicoes a
			// internet demora a subir, e o operador ficaria clicando duas vezes
			// no .exe de novo e de novo. Entao insiste, dizendo o que esta
			// acontecendo. Ctrl+C sai.
			p.linha("")
			p.aviso("Não consegui falar com o servidor.")
			p.linha("     " + cinza(err.Error()))
			p.linha("     " + cinza(fmt.Sprintf("Tentando de novo em %s... (Ctrl+C para sair)", esperaEntreTentativas)))
			time.Sleep(esperaEntreTentativas)
			continue
		}

		confirmada, trocar, err := p.confirmar(leitor, leilao)
		if err != nil {
			return cfg, backend.Leilao{}, err
		}

		if trocar {
			chave, err := p.pedirChave(leitor, "Cole a chave do leilão de hoje:")
			if err != nil {
				return cfg, backend.Leilao{}, err
			}
			cfg.Chave = chave
			continue
		}

		if confirmada {
			if err := config.Salvar(cfg); err != nil {
				p.aviso("não consegui salvar a configuração: " + err.Error())
			}
			return cfg, leilao, nil
		}
	}
}

func (p *Prompt) pedirChave(leitor *bufio.Reader, titulo string) (string, error) {
	if titulo == "" {
		titulo = "Nenhuma chave configurada."
		p.linha("")
		p.linha("  " + titulo)
		p.linha("  Pegue a chave no dashboard: card do leilão → botão \"Transmissão\".")
		titulo = "Cole a chave de transmissão e pressione Enter:"
	}

	p.linha("")
	p.linha("  " + titulo)
	p.escrever("  > ")

	linha, err := leitor.ReadString('\n')
	chave := strings.TrimSpace(linha)

	if chave == "" {
		if err != nil {
			return "", ErrCancelado
		}
		return "", errors.New("chave vazia")
	}
	return chave, nil
}

// confirmar mostra o leilao da chave e devolve (usar, trocar, erro).
func (p *Prompt) confirmar(leitor *bufio.Reader, leilao backend.Leilao) (bool, bool, error) {
	dias, conhecida := diasAtras(leilao.DataLeilao, p.agora())

	p.linha("")
	p.linha("  Chave salva aponta para:")
	p.linha("    " + leilao.Nome + " — " + formatarData(leilao.DataLeilao))
	p.linha(fmt.Sprintf("    %d lotes no catálogo", leilao.TotalLotes))

	// Default invertido quando a data nao e' hoje: operador com pressa aperta
	// Enter sem ler, e nos dois casos acerta.
	ehDeHoje := conhecida && dias == 0

	if !ehDeHoje {
		p.linha("")
		if conhecida {
			p.aviso(fmt.Sprintf(
				"Esse leilão é de %s (%s).", formatarData(leilao.DataLeilao), emPortugues(dias)))
		} else {
			p.aviso("Não consegui ler a data desse leilão.")
		}
		p.linha("     Você provavelmente precisa da chave do leilão de hoje.")
		p.linha("")
		p.linha("  [Enter] colar nova chave      [U] usar este leilão mesmo assim")
		p.escrever("  > ")

		resposta, err := ler(leitor)
		if err != nil {
			return false, false, err
		}
		if strings.EqualFold(resposta, "u") {
			return true, false, nil
		}
		return false, true, nil
	}

	p.linha("")
	p.linha("  [Enter] usar este leilão      [N] colar outra chave")
	p.escrever("  > ")

	resposta, err := ler(leitor)
	if err != nil {
		return false, false, err
	}
	if strings.EqualFold(resposta, "n") {
		return false, true, nil
	}
	return true, false, nil
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

func (p *Prompt) agora() time.Time {
	if p.Hoje != nil {
		return p.Hoje()
	}
	return time.Now()
}

func (p *Prompt) linha(texto string) { fmt.Fprintln(p.Saida, texto) }

func (p *Prompt) escrever(texto string) { fmt.Fprint(p.Saida, texto) }

func (p *Prompt) aviso(texto string) {
	fmt.Fprintln(p.Saida, "  "+amarelo("⚠  "+texto))
}

// diasAtras devolve quantos dias atras foi a data ISO, e se deu para ler.
func diasAtras(dataISO string, agora time.Time) (int, bool) {
	data, err := time.Parse("2006-01-02", dataISO)
	if err != nil {
		return 0, false
	}

	hoje := time.Date(agora.Year(), agora.Month(), agora.Day(), 0, 0, 0, 0, time.UTC)
	return int(hoje.Sub(data).Hours() / 24), true
}

func formatarData(dataISO string) string {
	data, err := time.Parse("2006-01-02", dataISO)
	if err != nil {
		return dataISO
	}
	return data.Format("02/01/2006")
}

func emPortugues(dias int) string {
	switch {
	case dias == 1:
		return "há 1 dia"
	case dias > 1:
		return fmt.Sprintf("há %d dias", dias)
	case dias == -1:
		return "amanhã"
	default:
		return fmt.Sprintf("em %d dias", -dias)
	}
}
