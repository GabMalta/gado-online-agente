//go:build !linux && !windows

package tui

import "os"

// entrarModoCru nao e' suportado aqui: os menus caem no modo numerado.
func entrarModoCru(*os.File) (func(), bool) { return nil, false }

func pausarAoSair() bool { return false }
