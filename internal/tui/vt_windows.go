//go:build windows

package tui

import "os"

// habilitarCores liga o Virtual Terminal Processing no console do Windows.
//
// O `conhost.exe` classico (o "Prompt de Comando" antigo, e o que abre ao dar
// dois cliques num .exe) NAO interpreta ANSI por padrao -- os codigos aparecem
// crus na tela como `←[33m`. O Windows Terminal ja liga sozinho.
func habilitarCores() bool {
	modo, ok := modoConsole(os.Stdout)
	if !ok {
		// Saida redirecionada para arquivo ou pipe: nao e' console, sem cor.
		return false
	}

	if modo&saidaTerminalVT != 0 {
		return true
	}
	return definirModoConsole(os.Stdout, modo|saidaTerminalVT)
}
