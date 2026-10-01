package tui

import "sync"

// Cores ANSI cruas, sem dependencia.
//
// `corAtiva` e' resolvido uma vez, na primeira utilizacao: no Windows e' preciso
// LIGAR o Virtual Terminal Processing, porque o conhost.exe classico -- o que
// abre ao dar dois cliques num .exe -- nao interpreta ANSI por padrao e imprime
// os codigos crus na tela (`←[33m`). Se nao der para ligar, as funcoes abaixo
// devolvem o texto sem enfeite, que e' legivel.
var (
	umaVez   sync.Once
	corAtiva bool
)

const (
	reset    = "\033[0m"
	corVerde = "\033[32m"
	corVerm  = "\033[31m"
	corAmar  = "\033[33m"
	corCinza = "\033[90m"
	negrito  = "\033[1m"
)

// suportaAnsi diz se o console interpreta sequencias de escape. Usado tambem
// pelo painel de status, que move o cursor para redesenhar em lugar -- sem
// suporte, aquelas sequencias virariam lixo na tela a cada ciclo.
func suportaAnsi() bool {
	umaVez.Do(func() { corAtiva = habilitarCores() })
	return corAtiva
}

func pintar(cor, s string) string {
	if !suportaAnsi() {
		return s
	}
	return cor + s + reset
}

func verde(s string) string    { return pintar(corVerde, s) }
func vermelho(s string) string { return pintar(corVerm, s) }
func amarelo(s string) string  { return pintar(corAmar, s) }
func cinza(s string) string    { return pintar(corCinza, s) }
func forte(s string) string    { return pintar(negrito, s) }
