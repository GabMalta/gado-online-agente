package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/GabMalta/gado-online-agente/internal/laco"
)

// Status desenha o painel do agente, redesenhando em lugar em vez de fazer
// scroll.
//
// Isso e' funcionalidade, nao enfeite: quando a pagina publica nao atualiza, a
// primeira pergunta e' se o agente esta lendo a fonte e se o servidor esta
// aceitando. O painel responde as duas de relance, sem o operador abrir log.
type Status struct {
	Saida   io.Writer
	Versao  string
	Fonte   string
	Detalhe string
	Leilao  string

	linhasAnteriores int

	// Usado so quando o console nao suporta ANSI: evita reimprimir o painel
	// identico a cada ciclo.
	ultimoResumo string
}

const largura = 56

func (s *Status) Desenhar(sit laco.Situacao) {
	var b strings.Builder

	// Sobe o cursor e limpa o que foi desenhado antes. Sem suporte a ANSI
	// (conhost.exe classico com VT desligado), redesenhar em lugar e'
	// impossivel: o painel passa a sair uma vez por mudanca, em vez de a cada
	// ciclo, para nao inundar o console.
	emLugar := suportaAnsi()

	if !emLugar {
		resumo := s.resumir(sit)
		if resumo == s.ultimoResumo {
			return
		}
		s.ultimoResumo = resumo
	} else if s.linhasAnteriores > 0 {
		fmt.Fprintf(&b, "\033[%dA\033[J", s.linhasAnteriores)
	}

	linhas := 0
	escrever := func(formato string, args ...any) {
		fmt.Fprintf(&b, formato+"\n", args...)
		linhas++
	}

	escrever("  %s %s", forte("Gado Online — agente de transmissão"), cinza("v"+s.Versao))
	escrever("  %s", cinza(strings.Repeat("─", largura)))
	escrever("  %-11s %s  %s", s.Fonte, sinal(sit.FonteOK), cinza("("+s.Detalhe+")"))
	escrever("  %-11s %s  %s", "Servidor", sinal(sit.ServidorOK), cinza(desdeUltimoEnvio(sit.UltimoEnvio)))
	escrever("  %-11s %s", "Leilão", s.Leilao)
	escrever("")

	if sit.Estado.Vazia() {
		escrever("  %s", cinza("⏸  pista livre — aguardando próximo lote"))
	} else {
		escrever("  %s", linhaDoLote(sit))
	}

	if sit.Erro != "" {
		escrever("")
		escrever("  %s", vermelho("✗  "+sit.Erro))
	}

	if emLugar {
		s.linhasAnteriores = linhas
	}
	_, _ = io.WriteString(s.Saida, b.String())
}

// resumir reduz a situacao ao que, mudando, merece reimprimir o painel.
func (s *Status) resumir(sit laco.Situacao) string {
	campos := make([]string, 0, len(sit.Estado.Overrides))
	for _, campo := range ordemOverrides {
		if _, tem := sit.Estado.Overrides[campo]; tem {
			campos = append(campos, campo)
		}
	}

	return fmt.Sprintf("%t|%t|%s|%s|%s|%s",
		sit.FonteOK, sit.ServidorOK, sit.Estado.Lote, sit.Estado.Valor,
		strings.Join(campos, ","), sit.Erro)
}

func linhaDoLote(sit laco.Situacao) string {
	partes := []string{verde("▶"), forte("lote " + sit.Estado.Lote)}

	if sit.Estado.Valor != "" {
		partes = append(partes, sit.Estado.Valor)
	}

	// Lista os campos que o operador digitou por cima do catalogo. E' o que
	// explica, sem abrir log, por que a pagina mostra algo diferente do
	// cadastro.
	if len(sit.Estado.Overrides) > 0 {
		campos := make([]string, 0, len(sit.Estado.Overrides))
		for _, campo := range ordemOverrides {
			if _, tem := sit.Estado.Overrides[campo]; tem {
				campos = append(campos, campo)
			}
		}
		if len(campos) > 0 {
			partes = append(partes, cinza("✎ "+strings.Join(campos, ", ")))
		}
	}

	return strings.Join(partes, "   ")
}

// Ordem fixa para o painel nao reordenar a cada ciclo (mapa em Go nao tem ordem).
var ordemOverrides = []string{"qtd_animais", "raca", "sexo", "idade", "peso", "obs"}

func sinal(ok bool) string {
	if ok {
		return verde("● conectado")
	}
	return vermelho("● sem contato")
}

func desdeUltimoEnvio(quando time.Time) string {
	if quando.IsZero() {
		return "nenhum envio ainda"
	}

	segundos := int(time.Since(quando).Seconds())
	if segundos <= 0 {
		return "último envio agora"
	}
	return fmt.Sprintf("último envio há %ds", segundos)
}

// Mensagem escreve uma linha fora do painel, para avisos que devem persistir.
func (s *Status) Mensagem(texto string) {
	s.linhasAnteriores = 0
	fmt.Fprintln(s.Saida, texto)
}
