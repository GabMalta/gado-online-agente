//go:build linux

package tui

import (
	"os"
	"syscall"
	"unsafe"
)

// entrarModoCru desliga o modo canonico, o eco e os sinais do terminal: cada
// tecla chega na hora, sem esperar Enter, e o Ctrl+C vira o byte 0x03.
//
// ioctl direto em vez de `golang.org/x/term`, pelo mesmo motivo de
// vt_windows.go: nao vale uma dependencia a mais. Devolve ok=false quando a
// entrada nao e' terminal (pipe, arquivo) -- o chamador cai no modo numerado.
func entrarModoCru(f *os.File) (restaurar func(), ok bool) {
	fd := f.Fd()

	var original syscall.Termios
	if !termios(fd, syscall.TCGETS, &original) {
		return nil, false
	}

	cru := original
	cru.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG
	cru.Cc[syscall.VMIN] = 1
	cru.Cc[syscall.VTIME] = 0

	if !termios(fd, syscall.TCSETS, &cru) {
		return nil, false
	}
	return func() { termios(fd, syscall.TCSETS, &original) }, true
}

func termios(fd uintptr, pedido uintptr, t *syscall.Termios) bool {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, pedido, uintptr(unsafe.Pointer(t)))
	return errno == 0
}

// pausarAoSair nao faz nada fora do Windows: no Linux o agente roda de um
// terminal que continua aberto depois que ele sai.
func pausarAoSair() bool { return false }
