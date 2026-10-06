package tui

import (
	"bufio"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/GabMalta/gado-online-agente/internal/backend"
	"github.com/GabMalta/gado-online-agente/internal/config"
)

// clienteFalso responde como o backend, sem rede.
type clienteFalso struct {
	senha   string
	leiloes [][]backend.Leilao // uma resposta por chamada; a ultima se repete
	chaves  map[string]string

	// falhasDeRede: quantas chamadas a Entrar falham antes de responder.
	falhasDeRede int

	logins        []string
	listagens     int
	chavesPedidas []string
	urlBase       string
}

func (c *clienteFalso) Entrar(usuario, senha string) error {
	if c.falhasDeRede > 0 {
		c.falhasDeRede--
		return &backend.ErroConexao{Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}}
	}
	c.logins = append(c.logins, usuario)
	if senha != c.senha {
		return backend.ErrLoginInvalido
	}
	return nil
}

func (c *clienteFalso) ListarLeiloesAtivos() ([]backend.Leilao, error) {
	i := c.listagens
	if i >= len(c.leiloes) {
		i = len(c.leiloes) - 1
	}
	c.listagens++
	return c.leiloes[i], nil
}

func (c *clienteFalso) ObterChave(leilaoID string) (string, error) {
	c.chavesPedidas = append(c.chavesPedidas, leilaoID)
	return c.chaves[leilaoID], nil
}

var (
	leilaoAoVivo = backend.Leilao{
		ID: "a", Nome: "LEILAO AO VIVO", DataLeilao: "2026-10-06", Status: backend.StatusEmAndamento, TotalLotes: 90,
	}
	leilaoAmanha = backend.Leilao{
		ID: "b", Nome: "LEILAO DE AMANHA", DataLeilao: "2026-10-07", Status: backend.StatusAguardando, TotalLotes: 1,
	}
)

func novoClienteFalso() *clienteFalso {
	return &clienteFalso{
		senha:   "certa",
		leiloes: [][]backend.Leilao{{leilaoAoVivo, leilaoAmanha}},
		chaves:  map[string]string{"a": "chave-a", "b": "chave-b"},
	}
}

// prompt monta um Prompt com entrada roteirizada. Sem terminal, os menus caem
// no modo numerado -- e' ele que a entrada abaixo exercita.
func prompt(t *testing.T, entrada string, c *clienteFalso) (*Prompt, *strings.Builder) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)

	var saida strings.Builder
	return &Prompt{
		Entrada: strings.NewReader(entrada),
		Saida:   &saida,
		NovoCliente: func(urlBase string) ClienteAbertura {
			c.urlBase = urlBase
			return c
		},
	}, &saida
}

func TestFluxoCompletoDevolveChaveDoLeilaoEscolhido(t *testing.T) {
	c := novoClienteFalso()
	// usuario, senha, software 2 (OBS), leilao 2
	p, _ := prompt(t, "joao\ncerta\n2\n2\n", c)

	cfg, abertura, err := p.Resolver(config.Padrao())
	if err != nil {
		t.Fatal(err)
	}

	if abertura.Software != config.SoftwareOBS {
		t.Errorf("software errado: %q", abertura.Software)
	}
	if abertura.Leilao.ID != "b" || abertura.Chave != "chave-b" {
		t.Errorf("abertura errada: %+v", abertura)
	}

	// Usuario e software ficam para a proxima abertura; nada de credencial.
	salva, err := config.Carregar()
	if err != nil {
		t.Fatal(err)
	}
	if salva.Usuario != "joao" || salva.Software != config.SoftwareOBS {
		t.Errorf("config salva errada: %+v", salva)
	}
	if cfg.Usuario != "joao" {
		t.Errorf("config devolvida errada: %+v", cfg)
	}
}

func TestEnterVazioUsaOsPadroes(t *testing.T) {
	c := novoClienteFalso()
	// Usuario vem da ultima vez; software e leilao ficam no pre-selecionado.
	p, saida := prompt(t, "\ncerta\n\n\n", c)

	cfg := config.Padrao()
	cfg.Usuario = "maria"
	cfg.Software = config.SoftwareOBS

	_, abertura, err := p.Resolver(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if c.logins[0] != "maria" {
		t.Errorf("devia logar com o usuario salvo, veio %q", c.logins[0])
	}
	if abertura.Software != config.SoftwareOBS {
		t.Errorf("devia manter o software da ultima vez, veio %q", abertura.Software)
	}
	// Em andamento vem primeiro, e e' o pre-selecionado.
	if abertura.Leilao.ID != "a" {
		t.Errorf("devia escolher o primeiro leilao, veio %+v", abertura.Leilao)
	}
	if !strings.Contains(saida.String(), "Usuário [maria]") {
		t.Errorf("o usuario salvo devia aparecer como padrao. Saida:\n%s", saida.String())
	}
}

func TestSenhaErradaPedeDeNovoComOUsuarioPreenchido(t *testing.T) {
	c := novoClienteFalso()
	p, saida := prompt(t, "joao\nerrada\n\ncerta\n1\n1\n", c)

	if _, _, err := p.Resolver(config.Padrao()); err != nil {
		t.Fatal(err)
	}

	if len(c.logins) != 2 || c.logins[1] != "joao" {
		t.Errorf("esperava duas tentativas como joao, veio %v", c.logins)
	}
	if !strings.Contains(saida.String(), "Usuário ou senha inválidos") {
		t.Errorf("nao avisou a senha errada. Saida:\n%s", saida.String())
	}
}

func TestLeilaoMostraDataStatusELotes(t *testing.T) {
	c := novoClienteFalso()
	p, saida := prompt(t, "joao\ncerta\n1\n1\n", c)

	if _, _, err := p.Resolver(config.Padrao()); err != nil {
		t.Fatal(err)
	}

	texto := saida.String()
	for _, esperado := range []string{
		"LEILAO AO VIVO — 06/10/2026 — Em andamento — 90 lotes",
		"LEILAO DE AMANHA — 07/10/2026 — Aguardando — 1 lote",
	} {
		if !strings.Contains(texto, esperado) {
			t.Errorf("faltou %q. Saida:\n%s", esperado, texto)
		}
	}
}

func TestSemLeilaoAtivoDeixaAtualizar(t *testing.T) {
	c := novoClienteFalso()
	c.leiloes = [][]backend.Leilao{{}, {leilaoAoVivo}}
	// Atualizar (1), depois escolhe o leilao que apareceu.
	p, saida := prompt(t, "joao\ncerta\n1\n1\n1\n", c)

	_, abertura, err := p.Resolver(config.Padrao())
	if err != nil {
		t.Fatal(err)
	}

	if c.listagens != 2 {
		t.Errorf("esperava listar duas vezes, veio %d", c.listagens)
	}
	if abertura.Leilao.ID != "a" {
		t.Errorf("leilao errado: %+v", abertura.Leilao)
	}
	if !strings.Contains(saida.String(), "Nenhum leilão aguardando ou em andamento") {
		t.Errorf("nao avisou a lista vazia. Saida:\n%s", saida.String())
	}
}

func TestSemLeilaoAtivoSairCancela(t *testing.T) {
	c := novoClienteFalso()
	c.leiloes = [][]backend.Leilao{{}}
	p, _ := prompt(t, "joao\ncerta\n1\n2\n", c)

	if _, _, err := p.Resolver(config.Padrao()); !errors.Is(err, ErrCancelado) {
		t.Fatalf("esperava ErrCancelado, veio %v", err)
	}
	if len(c.chavesPedidas) != 0 {
		t.Errorf("nao devia pedir chave: %v", c.chavesPedidas)
	}
}

func TestNumeroInvalidoNoMenuPedeDeNovo(t *testing.T) {
	c := novoClienteFalso()
	p, saida := prompt(t, "joao\ncerta\n7\nobs\n2\n1\n", c)

	_, abertura, err := p.Resolver(config.Padrao())
	if err != nil {
		t.Fatal(err)
	}
	if abertura.Software != config.SoftwareOBS {
		t.Errorf("software errado: %q", abertura.Software)
	}
	if strings.Count(saida.String(), "Digite um número de 1 a 2") != 2 {
		t.Errorf("devia avisar duas vezes. Saida:\n%s", saida.String())
	}
}

func TestEntradaFechadaCancela(t *testing.T) {
	c := novoClienteFalso()
	p, _ := prompt(t, "", c)

	if _, _, err := p.Resolver(config.Padrao()); !errors.Is(err, ErrCancelado) {
		t.Fatalf("esperava ErrCancelado, veio %v", err)
	}
}

func TestServidorForaDoArNaAberturaInsisteEnaoDesiste(t *testing.T) {
	// Encurta a espera para o teste nao levar 10s.
	original := esperaEntreTentativas
	esperaEntreTentativas = time.Millisecond
	t.Cleanup(func() { esperaEntreTentativas = original })

	c := novoClienteFalso()
	c.falhasDeRede = 2
	p, saida := prompt(t, "joao\ncerta\n1\n1\n", c)

	if _, _, err := p.Resolver(config.Padrao()); err != nil {
		t.Fatalf("devia ter insistido ate conseguir, veio %v", err)
	}
	// A senha nao e' pedida de novo: a falha foi de rede, nao de login.
	if len(c.logins) != 1 {
		t.Errorf("esperava um login depois das falhas, veio %v", c.logins)
	}
	if !strings.Contains(saida.String(), "Tentando de novo") {
		t.Errorf("nao avisou que estava tentando. Saida:\n%s", saida.String())
	}
}

// TestPrimeiraExecucaoNaoMontaURLVazia cobre o bug que foi para producao: a
// config da primeira execucao vinha sem URLBase e o agente fazia requisicao
// sem host -- "unsupported protocol scheme".
func TestPrimeiraExecucaoNaoMontaURLVazia(t *testing.T) {
	c := novoClienteFalso()
	p, _ := prompt(t, "joao\ncerta\n1\n1\n", c)

	// Exatamente o que o main faz na primeira execucao.
	cfg, _ := config.Carregar()

	if _, _, err := p.Resolver(cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.urlBase, "http") {
		t.Errorf("o cliente recebeu URL sem esquema: %q", c.urlBase)
	}
}

func TestLerTeclaEntendeSetasEnterECtrlC(t *testing.T) {
	// Setas simples, Ctrl+seta com parametro, Enter do Windows (\r) e do
	// Linux (\n), backspace, Ctrl+C e uma letra acentuada.
	entrada := "\x1b[A\x1b[B\x1b[1;5A\x1bOB\r\n\x7f\x03é"
	leitor := bufio.NewReader(strings.NewReader(entrada))

	esperado := []tecla{
		teclaCima, teclaBaixo, teclaCima, teclaBaixo,
		teclaEnter, teclaEnter, teclaApagar, teclaCancelar, teclaTexto,
	}

	for i, quer := range esperado {
		ev, err := lerTecla(leitor)
		if err != nil {
			t.Fatalf("tecla %d: %v", i, err)
		}
		if ev.tecla != quer {
			t.Errorf("tecla %d: esperava %d, veio %d", i, quer, ev.tecla)
		}
		if quer == teclaTexto && ev.letra != 'é' {
			t.Errorf("letra errada: %q", ev.letra)
		}
	}
}
