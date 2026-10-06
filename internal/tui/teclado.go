package tui

import (
	"bufio"
	"unicode"
)

// tecla e' o que os menus e os campos de texto entendem do teclado.
type tecla int

const (
	teclaOutra tecla = iota
	teclaCima
	teclaBaixo
	teclaEnter
	teclaApagar
	teclaCancelar
	teclaTexto
)

type evento struct {
	tecla tecla
	letra rune
}

// lerTecla le uma tecla do console em modo cru.
//
// As setas chegam como sequencia de escape (`ESC [ A`) tanto no Linux quanto
// no Windows com ENABLE_VIRTUAL_TERMINAL_INPUT -- ver modo_cru_windows.go. O
// Ctrl+C chega como o byte 0x03, porque o modo cru desliga o sinal: quem trata
// e' o chamador, devolvendo ErrCancelado.
func lerTecla(leitor *bufio.Reader) (evento, error) {
	letra, _, err := leitor.ReadRune()
	if err != nil {
		return evento{}, err
	}

	switch letra {
	case '\r', '\n':
		return evento{tecla: teclaEnter}, nil
	case 0x03:
		return evento{tecla: teclaCancelar}, nil
	case 0x7f, 0x08:
		return evento{tecla: teclaApagar}, nil
	case 0x1b:
		return lerEscape(leitor)
	}

	if unicode.IsPrint(letra) {
		return evento{tecla: teclaTexto, letra: letra}, nil
	}
	return evento{tecla: teclaOutra}, nil
}

// lerEscape consome o resto de uma sequencia `ESC [ ... letra` ou `ESC O letra`.
// Sequencias com parametro (`ESC [ 1 ; 5 A`, Ctrl+seta) valem pela letra final.
func lerEscape(leitor *bufio.Reader) (evento, error) {
	introdutor, err := leitor.ReadByte()
	if err != nil {
		return evento{}, err
	}
	if introdutor != '[' && introdutor != 'O' {
		return evento{tecla: teclaOutra}, nil
	}

	for {
		b, err := leitor.ReadByte()
		if err != nil {
			return evento{}, err
		}
		if (b >= '0' && b <= '9') || b == ';' {
			continue
		}

		switch b {
		case 'A':
			return evento{tecla: teclaCima}, nil
		case 'B':
			return evento{tecla: teclaBaixo}, nil
		}
		return evento{tecla: teclaOutra}, nil
	}
}
