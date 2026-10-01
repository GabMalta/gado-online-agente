//go:build !windows

package tui

import "os"

// habilitarCores em Linux/macOS: o ANSI funciona em terminal de verdade, mas em
// saida redirecionada para arquivo os codigos sujariam o log.
//
// Checa `ModeCharDevice` em vez de usar `golang.org/x/term`: terminal e'
// char device, arquivo e pipe nao sao -- e isso dispensa a dependencia.
func habilitarCores() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
