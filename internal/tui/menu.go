package tui

import (
	"fmt"
	"strconv"
)

const (
	ocultarCursor = "\033[?25l"
	mostrarCursor = "\033[?25h"

	// larguraOpcao corta o texto da opcao: linha que quebra no console estraga
	// a conta de quantas linhas subir para redesenhar o menu.
	larguraOpcao = 70
)

// selecionar mostra um menu navegavel com as setas e devolve o indice
// escolhido.
//
// Sem terminal (entrada de pipe, testes) ou sem ANSI, cai numa lista numerada:
// o operador digita o numero e Enter. Enter vazio fica com `inicial`.
func (p *Prompt) selecionar(titulo string, opcoes []string, inicial int) (int, error) {
	if inicial < 0 || inicial >= len(opcoes) {
		inicial = 0
	}

	p.linha("")
	p.linha("  " + forte(titulo))

	restaurar, cru := p.modoCru()
	if cru && !suportaAnsi() {
		// Sem ANSI nao da para redesenhar em lugar: o menu viraria uma pilha de
		// copias a cada seta.
		restaurar()
		cru = false
	}
	if !cru {
		return p.selecionarNumerado(opcoes, inicial)
	}
	defer restaurar()

	p.escrever(ocultarCursor)
	defer p.escrever(mostrarCursor)

	p.linha("  " + cinza("↑ ↓ para escolher, Enter para confirmar"))

	atual := inicial
	p.desenharOpcoes(opcoes, atual)

	for {
		ev, err := lerTecla(p.leitor)
		if err != nil {
			return 0, ErrCancelado
		}

		switch ev.tecla {
		case teclaCima:
			atual = (atual - 1 + len(opcoes)) % len(opcoes)
		case teclaBaixo:
			atual = (atual + 1) % len(opcoes)
		case teclaEnter:
			// Recolhe o menu na opcao escolhida: a tela fica com o resumo do
			// que foi decidido, nao com a lista inteira.
			p.escrever(fmt.Sprintf("\033[%dA\033[J", len(opcoes)+1))
			p.linha("  " + verde("✓ ") + limitar(opcoes[atual]))
			return atual, nil
		case teclaCancelar:
			return 0, ErrCancelado
		default:
			continue
		}

		p.escrever(fmt.Sprintf("\033[%dA\033[J", len(opcoes)))
		p.desenharOpcoes(opcoes, atual)
	}
}

func (p *Prompt) desenharOpcoes(opcoes []string, atual int) {
	for i, opcao := range opcoes {
		if i == atual {
			p.linha("  " + verde("▶ ") + forte(limitar(opcao)))
		} else {
			p.linha("    " + limitar(opcao))
		}
	}
}

func (p *Prompt) selecionarNumerado(opcoes []string, inicial int) (int, error) {
	for i, opcao := range opcoes {
		p.linha(fmt.Sprintf("    %d) %s", i+1, opcao))
	}

	for {
		p.escrever(fmt.Sprintf("  Número [%d]: ", inicial+1))

		resposta, err := ler(p.leitor)
		if err != nil {
			return 0, err
		}
		if resposta == "" {
			return inicial, nil
		}

		n, err := strconv.Atoi(resposta)
		if err == nil && n >= 1 && n <= len(opcoes) {
			return n - 1, nil
		}
		p.aviso(fmt.Sprintf("Digite um número de 1 a %d.", len(opcoes)))
	}
}

func limitar(texto string) string {
	letras := []rune(texto)
	if len(letras) <= larguraOpcao {
		return texto
	}
	return string(letras[:larguraOpcao-1]) + "…"
}
