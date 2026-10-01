//go:build windows

package tui

import (
	"os"
	"syscall"
	"unsafe"
)

// https://learn.microsoft.com/windows/console/setconsolemode
const habilitaVirtualTerminal = 0x0004

// habilitarCores liga o Virtual Terminal Processing no console do Windows.
//
// O `conhost.exe` classico (o "Prompt de Comando" antigo, e o que abre ao dar
// dois cliques num .exe) NAO interpreta ANSI por padrao -- os codigos aparecem
// crus na tela como `←[33m`. O Windows Terminal ja liga sozinho.
//
// Usa kernel32 via syscall em vez de `golang.org/x/sys/windows` de proposito:
// nao vale uma dependencia a mais num binario que vai para a maquina do cliente.
func habilitarCores() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	handle := syscall.Handle(os.Stdout.Fd())

	var modo uint32
	ret, _, _ := getConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&modo)))
	if ret == 0 {
		// Saida redirecionada para arquivo ou pipe: nao e' console, sem cor.
		return false
	}

	if modo&habilitaVirtualTerminal != 0 {
		return true
	}

	ret, _, _ = setConsoleMode.Call(uintptr(handle), uintptr(modo|habilitaVirtualTerminal))
	return ret != 0
}
