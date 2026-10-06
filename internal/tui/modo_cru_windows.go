//go:build windows

package tui

import (
	"os"
	"syscall"
	"unsafe"
)

// https://learn.microsoft.com/windows/console/setconsolemode
const (
	entradaProcessada = 0x0001 // ENABLE_PROCESSED_INPUT: Ctrl+C vira sinal
	entradaPorLinha   = 0x0002 // ENABLE_LINE_INPUT
	entradaComEco     = 0x0004 // ENABLE_ECHO_INPUT
	entradaTerminalVT = 0x0200 // ENABLE_VIRTUAL_TERMINAL_INPUT
	saidaTerminalVT   = 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING
)

// kernel32 via syscall em vez de `golang.org/x/sys/windows` de proposito: nao
// vale uma dependencia a mais num binario que vai para a maquina do cliente.
var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode      = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode      = kernel32.NewProc("SetConsoleMode")
	procGetConsoleProcesses = kernel32.NewProc("GetConsoleProcessList")
)

func modoConsole(f *os.File) (uint32, bool) {
	var modo uint32
	ret, _, _ := procGetConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&modo)))
	return modo, ret != 0
}

func definirModoConsole(f *os.File, modo uint32) bool {
	ret, _, _ := procSetConsoleMode.Call(f.Fd(), uintptr(modo))
	return ret != 0
}

// entrarModoCru desliga a leitura por linha e o eco, e liga a entrada VT.
//
// A entrada VT nao e' detalhe: sem ela o ReadConsole simplesmente nao entrega
// as setas (elas nao sao caractere). Com ela, chegam como `ESC [ A`, igual ao
// Linux, e o mesmo parser serve aos dois. Windows antigo que recusa o modo cai
// no menu numerado.
func entrarModoCru(f *os.File) (restaurar func(), ok bool) {
	original, ok := modoConsole(f)
	if !ok {
		return nil, false
	}

	cru := original&^(entradaProcessada|entradaPorLinha|entradaComEco) | entradaTerminalVT
	if !definirModoConsole(f, cru) {
		return nil, false
	}
	return func() { definirModoConsole(f, original) }, true
}

// pausarAoSair diz se o agente e' o unico processo no console -- o caso do
// duplo clique no .exe, em que a janela fecha junto com o processo e leva
// embora a mensagem de erro. Aberto de um cmd ou PowerShell, o console tem
// outros processos e continua aberto sozinho.
func pausarAoSair() bool {
	var lista [2]uint32
	n, _, _ := procGetConsoleProcesses.Call(uintptr(unsafe.Pointer(&lista[0])), uintptr(len(lista)))
	return n == 1
}
