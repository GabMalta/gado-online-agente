package tui

// Cores ANSI cruas, sem dependencia. Windows 10+ interpreta ANSI no console;
// em terminal que nao interpreta, o pior caso e' um codigo visivel, nao uma
// falha -- e nao vale uma dependencia a mais num binario que vai para a maquina
// do cliente.
const (
	reset    = "\033[0m"
	corVerde = "\033[32m"
	corVerm  = "\033[31m"
	corAmar  = "\033[33m"
	corCinza = "\033[90m"
	negrito  = "\033[1m"
)

func verde(s string) string    { return corVerde + s + reset }
func vermelho(s string) string { return corVerm + s + reset }
func amarelo(s string) string  { return corAmar + s + reset }
func cinza(s string) string    { return corCinza + s + reset }
func forte(s string) string    { return negrito + s + reset }
