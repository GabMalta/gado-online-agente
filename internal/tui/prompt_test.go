package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GabMalta/gado-online-agente/internal/backend"
	"github.com/GabMalta/gado-online-agente/internal/config"
)

type clienteFalso struct {
	porChave map[string]backend.Leilao
	chamadas []string
}

func (c *clienteFalso) para(chave string) ClienteLeilao {
	c.chamadas = append(c.chamadas, chave)
	return &respostaFalsa{leilao: c.porChave[chave], conhecida: existe(c.porChave, chave)}
}

func existe(m map[string]backend.Leilao, k string) bool {
	_, ok := m[k]
	return ok
}

type respostaFalsa struct {
	leilao    backend.Leilao
	conhecida bool
}

func (r *respostaFalsa) BuscarLeilao() (backend.Leilao, error) {
	if !r.conhecida {
		return backend.Leilao{}, backend.ErrChaveRecusada
	}
	return r.leilao, nil
}

// prompt monta um Prompt com entrada roteirizada e data fixa.
func prompt(t *testing.T, entrada string, c *clienteFalso, hoje string) (*Prompt, *strings.Builder) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)

	var saida strings.Builder
	data, err := time.Parse("2006-01-02", hoje)
	if err != nil {
		t.Fatal(err)
	}

	return &Prompt{
		Entrada:     strings.NewReader(entrada),
		Saida:       &saida,
		NovoCliente: func(_, chave string) ClienteLeilao { return c.para(chave) },
		Hoje:        func() time.Time { return data },
	}, &saida
}

func TestSemChaveSalvaPedeAChave(t *testing.T) {
	c := &clienteFalso{porChave: map[string]backend.Leilao{
		"nova-chave": {Nome: "LEILAO DE HOJE", DataLeilao: "2026-10-01", TotalLotes: 90},
	}}

	// Cola a chave, depois Enter para confirmar o leilao de hoje.
	p, saida := prompt(t, "nova-chave\n\n", c, "2026-10-01")

	cfg, leilao, err := p.Resolver(config.Config{URLBase: "http://x"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Chave != "nova-chave" {
		t.Errorf("chave nao foi guardada: %q", cfg.Chave)
	}
	if leilao.Nome != "LEILAO DE HOJE" {
		t.Errorf("leilao errado: %+v", leilao)
	}
	if !strings.Contains(saida.String(), "Nenhuma chave configurada") {
		t.Error("nao avisou que faltava chave")
	}
}

func TestChaveSalvaDeHojeEnterConfirma(t *testing.T) {
	c := &clienteFalso{porChave: map[string]backend.Leilao{
		"salva": {Nome: "LEILAO DE HOJE", DataLeilao: "2026-10-01", TotalLotes: 90},
	}}

	p, saida := prompt(t, "\n", c, "2026-10-01")

	cfg, leilao, err := p.Resolver(config.Config{Chave: "salva", URLBase: "http://x"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Chave != "salva" {
		t.Errorf("devia ter mantido a chave salva: %q", cfg.Chave)
	}
	if leilao.Nome != "LEILAO DE HOJE" {
		t.Errorf("leilao errado: %+v", leilao)
	}
	if !strings.Contains(saida.String(), "[Enter] usar este leilão") {
		t.Errorf("com data de hoje o Enter devia ser 'usar'. Saida:\n%s", saida.String())
	}
}

func TestChaveSalvaDeHojeNTrocaAChave(t *testing.T) {
	c := &clienteFalso{porChave: map[string]backend.Leilao{
		"velha": {Nome: "LEILAO A", DataLeilao: "2026-10-01"},
		"nova":  {Nome: "LEILAO B", DataLeilao: "2026-10-01"},
	}}

	p, _ := prompt(t, "n\nnova\n\n", c, "2026-10-01")

	cfg, leilao, err := p.Resolver(config.Config{Chave: "velha", URLBase: "http://x"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Chave != "nova" || leilao.Nome != "LEILAO B" {
		t.Errorf("nao trocou a chave: %q / %+v", cfg.Chave, leilao)
	}
}

// O teste central desta parte: com leilao antigo, Enter PEDE chave nova.
func TestChaveDeLeilaoAntigoEnterPedeNovaChave(t *testing.T) {
	c := &clienteFalso{porChave: map[string]backend.Leilao{
		"agosto": {Nome: "LEILAO DE AGOSTO", DataLeilao: "2026-08-04"},
		"hoje":   {Nome: "LEILAO DE HOJE", DataLeilao: "2026-10-01", TotalLotes: 90},
	}}

	// Enter (default) e depois a chave nova.
	p, saida := prompt(t, "\nhoje\n\n", c, "2026-10-01")

	cfg, leilao, err := p.Resolver(config.Config{Chave: "agosto", URLBase: "http://x"})
	if err != nil {
		t.Fatal(err)
	}

	// Sem isso, o operador com pressa passaria o leilao de hoje alimentando a
	// pagina publica de agosto.
	if cfg.Chave != "hoje" {
		t.Errorf("Enter em leilao antigo devia pedir chave nova, ficou com %q", cfg.Chave)
	}
	if leilao.Nome != "LEILAO DE HOJE" {
		t.Errorf("leilao errado: %+v", leilao)
	}

	texto := saida.String()
	if !strings.Contains(texto, "há 58 dias") {
		t.Errorf("nao disse quantos dias atras. Saida:\n%s", texto)
	}
	if !strings.Contains(texto, "[Enter] colar nova chave") {
		t.Errorf("o default devia ser 'colar nova chave'. Saida:\n%s", texto)
	}
}

func TestChaveDeLeilaoAntigoUsaMesmoAssimComU(t *testing.T) {
	c := &clienteFalso{porChave: map[string]backend.Leilao{
		"agosto": {Nome: "LEILAO DE AGOSTO", DataLeilao: "2026-08-04"},
	}}

	p, _ := prompt(t, "u\n", c, "2026-10-01")

	cfg, leilao, err := p.Resolver(config.Config{Chave: "agosto", URLBase: "http://x"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Chave != "agosto" || leilao.Nome != "LEILAO DE AGOSTO" {
		t.Errorf("U devia usar o leilao antigo: %q / %+v", cfg.Chave, leilao)
	}
}

func TestChaveRevogadaEhApagadaEOPromptPedeOutra(t *testing.T) {
	c := &clienteFalso{porChave: map[string]backend.Leilao{
		"boa": {Nome: "LEILAO DE HOJE", DataLeilao: "2026-10-01"},
	}}

	p, saida := prompt(t, "boa\n\n", c, "2026-10-01")

	// `revogada` nao esta no mapa -> ErrChaveRecusada.
	if err := config.Salvar(config.Config{Chave: "revogada", URLBase: "http://x"}); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := p.Resolver(config.Config{Chave: "revogada", URLBase: "http://x"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Chave != "boa" {
		t.Errorf("devia ter pedido outra chave, ficou com %q", cfg.Chave)
	}
	if !strings.Contains(saida.String(), "não vale mais") {
		t.Errorf("nao explicou o motivo. Saida:\n%s", saida.String())
	}

	// Credencial revogada nao tem motivo para continuar no disco.
	salva, err := config.Carregar()
	if err == nil && salva.Chave == "revogada" {
		t.Error("a chave revogada ficou no disco")
	}
}

func TestLeilaoDeAmanhaTambemAvisa(t *testing.T) {
	// Leilao criado para amanha: nao e' hoje, entao o default continua sendo
	// trocar — mas a mensagem nao pode dizer "ha -1 dias".
	c := &clienteFalso{porChave: map[string]backend.Leilao{
		"amanha": {Nome: "LEILAO DE AMANHA", DataLeilao: "2026-10-02"},
	}}

	p, saida := prompt(t, "u\n", c, "2026-10-01")

	if _, _, err := p.Resolver(config.Config{Chave: "amanha", URLBase: "http://x"}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(saida.String(), "amanhã") {
		t.Errorf("mensagem de data futura ruim. Saida:\n%s", saida.String())
	}
}

func TestDataIlegivelAvisaENaoAssumeHoje(t *testing.T) {
	c := &clienteFalso{porChave: map[string]backend.Leilao{
		"estranha": {Nome: "LEILAO X", DataLeilao: "sem-data"},
	}}

	p, saida := prompt(t, "u\n", c, "2026-10-01")

	if _, _, err := p.Resolver(config.Config{Chave: "estranha", URLBase: "http://x"}); err != nil {
		t.Fatal(err)
	}

	texto := saida.String()
	if !strings.Contains(texto, "Não consegui ler a data") {
		t.Errorf("devia avisar que nao leu a data. Saida:\n%s", texto)
	}
	// Na duvida, o default seguro e' pedir chave nova.
	if !strings.Contains(texto, "[Enter] colar nova chave") {
		t.Errorf("default devia ser trocar. Saida:\n%s", texto)
	}
}

func TestEntradaFechadaSemChaveCancela(t *testing.T) {
	c := &clienteFalso{porChave: map[string]backend.Leilao{}}
	p, _ := prompt(t, "", c, "2026-10-01")

	_, _, err := p.Resolver(config.Config{URLBase: "http://x"})

	if !errors.Is(err, ErrCancelado) {
		t.Fatalf("esperava ErrCancelado, veio %v", err)
	}
}

func TestDiasAtras(t *testing.T) {
	hoje, _ := time.Parse("2006-01-02", "2026-10-01")

	casos := []struct {
		data      string
		dias      int
		conhecida bool
	}{
		{"2026-10-01", 0, true},
		{"2026-09-30", 1, true},
		{"2026-08-04", 58, true},
		{"2026-10-02", -1, true},
		{"lixo", 0, false},
	}

	for _, caso := range casos {
		dias, conhecida := diasAtras(caso.data, hoje)
		if dias != caso.dias || conhecida != caso.conhecida {
			t.Errorf("%s: esperava (%d,%v), veio (%d,%v)",
				caso.data, caso.dias, caso.conhecida, dias, conhecida)
		}
	}
}

// clienteInstavel falha nas primeiras tentativas e depois responde, simulando a
// internet do parque de exposicoes subindo com atraso.
type clienteInstavel struct {
	falhasRestantes int
	leilao          backend.Leilao
	tentativas      int
}

func (c *clienteInstavel) BuscarLeilao() (backend.Leilao, error) {
	c.tentativas++
	if c.falhasRestantes > 0 {
		c.falhasRestantes--
		return backend.Leilao{}, errors.New("dial tcp: connection refused")
	}
	return c.leilao, nil
}

func TestServidorForaDoArNaAberturaInsisteEnaoDesiste(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)

	// Encurta a espera para o teste nao levar 10s.
	original := esperaEntreTentativas
	esperaEntreTentativas = time.Millisecond
	t.Cleanup(func() { esperaEntreTentativas = original })

	instavel := &clienteInstavel{
		falhasRestantes: 2,
		leilao:          backend.Leilao{Nome: "LEILAO DE HOJE", DataLeilao: "2026-10-01"},
	}

	hoje, _ := time.Parse("2006-01-02", "2026-10-01")
	var saida strings.Builder
	p := &Prompt{
		Entrada:     strings.NewReader("\n"),
		Saida:       &saida,
		NovoCliente: func(_, _ string) ClienteLeilao { return instavel },
		Hoje:        func() time.Time { return hoje },
	}

	_, leilao, err := p.Resolver(config.Config{Chave: "k", URLBase: "http://x"})
	if err != nil {
		t.Fatalf("devia ter insistido ate conseguir, veio %v", err)
	}
	if leilao.Nome != "LEILAO DE HOJE" {
		t.Errorf("leilao errado: %+v", leilao)
	}
	if instavel.tentativas != 3 {
		t.Errorf("esperava 3 tentativas, veio %d", instavel.tentativas)
	}
	if !strings.Contains(saida.String(), "Tentando de novo") {
		t.Errorf("nao avisou que estava tentando. Saida:\n%s", saida.String())
	}
}
