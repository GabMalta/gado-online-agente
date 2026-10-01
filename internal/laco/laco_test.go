package laco

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GabMalta/gado-online-agente/internal/backend"
	"github.com/GabMalta/gado-online-agente/internal/fonte"
)

// fonteFalsa devolve estados roteirizados, incluindo erros.
type fonteFalsa struct {
	estados []fonte.Estado
	erros   []error
	i       int
}

func (f *fonteFalsa) Nome() string      { return "falsa" }
func (f *fonteFalsa) Descricao() string { return "memoria" }
func (f *fonteFalsa) Fechar() error     { return nil }

func (f *fonteFalsa) Ler() (fonte.Estado, error) {
	defer func() { f.i++ }()

	if f.i < len(f.erros) && f.erros[f.i] != nil {
		return fonte.Estado{}, f.erros[f.i]
	}
	if f.i < len(f.estados) {
		return f.estados[f.i], nil
	}
	if len(f.estados) == 0 {
		return fonte.Estado{}, nil
	}
	return f.estados[len(f.estados)-1], nil
}

// servidorEspiao conta as chamadas de cada rota.
type servidorEspiao struct {
	*httptest.Server
	pistas, liberadas atomic.Int32
	status            atomic.Int32
}

func novoEspiao() *servidorEspiao {
	e := &servidorEspiao{}
	e.status.Store(http.StatusOK)

	e.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s := int(e.status.Load()); s != http.StatusOK {
			w.WriteHeader(s)
			_, _ = io.WriteString(w, `{"error":true,"message":"falhou","data":null}`)
			return
		}

		switch r.URL.Path {
		case "/api/transmissao/pista":
			e.pistas.Add(1)
		case "/api/transmissao/pista/liberar":
			e.liberadas.Add(1)
		}
		_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":null}`)
	}))
	return e
}

// rodarCiclos roda exatamente n ciclos e devolve a ultima situacao.
func rodarCiclos(t *testing.T, l *Laco, n int) Situacao {
	t.Helper()

	var ultima Situacao
	for i := 0; i < n; i++ {
		s, _ := l.umCiclo()
		ultima = s
	}
	return ultima
}

func TestEnviaPistaQuandoHaLote(t *testing.T) {
	e := novoEspiao()
	defer e.Close()

	f := &fonteFalsa{estados: []fonte.Estado{{Lote: "001", Valor: "1850"}}}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond, nil)

	s := rodarCiclos(t, l, 1)

	if !s.ServidorOK || !s.FonteOK {
		t.Errorf("situacao inesperada: %+v", s)
	}
	if e.pistas.Load() != 1 {
		t.Errorf("esperava 1 envio, veio %d", e.pistas.Load())
	}
}

func TestReenviaSemMudancaPorqueEhHeartbeat(t *testing.T) {
	e := novoEspiao()
	defer e.Close()

	// Mesmo estado em todos os ciclos.
	f := &fonteFalsa{estados: []fonte.Estado{{Lote: "001", Valor: "1850"}}}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond, nil)

	rodarCiclos(t, l, 4)

	// Sem o reenvio a pista expiraria pelo TTL de 120s no backend.
	if e.pistas.Load() != 4 {
		t.Errorf("heartbeat nao reenviou: %d envios em 4 ciclos", e.pistas.Load())
	}
}

func TestLiberaNaTransicaoDeLoteParaVazio(t *testing.T) {
	e := novoEspiao()
	defer e.Close()

	f := &fonteFalsa{estados: []fonte.Estado{
		{Lote: "001"},
		{}, // overlay limpo = fim do lote
	}}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond, nil)

	rodarCiclos(t, l, 2)

	if e.liberadas.Load() != 1 {
		t.Errorf("esperava 1 liberacao, veio %d", e.liberadas.Load())
	}
}

func TestNaoLiberaRepetidamenteComOverlayVazio(t *testing.T) {
	e := novoEspiao()
	defer e.Close()

	f := &fonteFalsa{estados: []fonte.Estado{{}}}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond, nil)

	rodarCiclos(t, l, 5)

	// Seria uma requisicao por ciclo durante todo o intervalo entre lotes.
	if e.liberadas.Load() != 0 {
		t.Errorf("liberou %d vezes sem nunca ter tido lote", e.liberadas.Load())
	}
}

func TestErroDeLeituraNaoMexeNaPista(t *testing.T) {
	e := novoEspiao()
	defer e.Close()

	f := &fonteFalsa{
		estados: []fonte.Estado{{Lote: "001"}, {}},
		erros:   []error{nil, errors.New("vMix fora do ar")},
	}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond, nil)

	rodarCiclos(t, l, 2)

	// Melhor manter o lote no ar (o TTL cobre) que liberar por erro transitorio.
	if e.liberadas.Load() != 0 {
		t.Error("liberou a pista por causa de uma falha de leitura")
	}
	s := rodarCiclos(t, l, 0)
	_ = s
}

func TestErroDeLeituraReportaMasNaoDerruba(t *testing.T) {
	e := novoEspiao()
	defer e.Close()

	f := &fonteFalsa{erros: []error{errors.New("vMix fora do ar")}}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond, nil)

	s, recusada := l.umCiclo()

	if recusada {
		t.Error("falha de leitura nao devia parar o agente")
	}
	if s.FonteOK {
		t.Error("FonteOK devia ser false")
	}
	if s.Erro == "" {
		t.Error("o erro devia aparecer na situacao, para o console mostrar")
	}
}

func TestChaveRecusadaParaOLaco(t *testing.T) {
	e := novoEspiao()
	defer e.Close()
	e.status.Store(http.StatusUnauthorized)

	f := &fonteFalsa{estados: []fonte.Estado{{Lote: "001"}}}
	l := Novo(f, backend.NovoCliente(e.URL, "revogada"), time.Millisecond, nil)

	parar := make(chan struct{})
	err := l.Rodar(parar)

	// E' a unica falha que para o agente: o operador precisa agir.
	if !errors.Is(err, backend.ErrChaveRecusada) {
		t.Fatalf("esperava ErrChaveRecusada, veio %v", err)
	}
}

func TestFalhaDeServidorNaoParaOLaco(t *testing.T) {
	e := novoEspiao()
	defer e.Close()
	e.status.Store(http.StatusInternalServerError)

	f := &fonteFalsa{estados: []fonte.Estado{{Lote: "001"}}}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond, nil)

	s, recusada := l.umCiclo()

	if recusada {
		t.Error("500 nao devia parar o agente: o leilao esta ao vivo")
	}
	if s.ServidorOK {
		t.Error("ServidorOK devia ser false")
	}
}

func TestRecuperaSozinhoQuandoOServidorVolta(t *testing.T) {
	e := novoEspiao()
	defer e.Close()
	e.status.Store(http.StatusInternalServerError)

	f := &fonteFalsa{estados: []fonte.Estado{{Lote: "001"}}}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond, nil)

	l.umCiclo()
	e.status.Store(http.StatusOK)
	s, _ := l.umCiclo()

	if !s.ServidorOK {
		t.Error("nao recuperou quando o servidor voltou")
	}
}

func TestRodarParaQuandoOCanalFecha(t *testing.T) {
	e := novoEspiao()
	defer e.Close()

	f := &fonteFalsa{estados: []fonte.Estado{{Lote: "001"}}}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond, nil)

	parar := make(chan struct{})
	pronto := make(chan error, 1)

	go func() { pronto <- l.Rodar(parar) }()
	time.Sleep(20 * time.Millisecond)
	close(parar)

	select {
	case err := <-pronto:
		if err != nil {
			t.Errorf("esperava parada limpa, veio %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Rodar nao parou ao fechar o canal")
	}
}

func TestBackoffCresceEhLimitado(t *testing.T) {
	base := 3 * time.Second

	d := proximoBackoff(0, base)
	if d != base {
		t.Errorf("primeiro recuo devia ser o intervalo base, veio %v", d)
	}

	for i := 0; i < 20; i++ {
		d = proximoBackoff(d, base)
	}
	// Teto abaixo do TTL de 120s: recuar mais deixaria a pista expirar.
	if d != BackoffMax {
		t.Errorf("backoff nao foi limitado: %v", d)
	}
	if BackoffMax >= 120*time.Second {
		t.Error("BackoffMax tem que ficar abaixo do TTL de 120s da pista")
	}
}

func TestParaPistaMapeiaOsOverrides(t *testing.T) {
	p := paraPista(fonte.Estado{
		Lote:      "001",
		Valor:     "1850",
		Overrides: map[string]string{"obs": "Reagrupado", "raca": "Nelore"},
	})

	if p.Obs != "Reagrupado" || p.Raca != "Nelore" {
		t.Errorf("overrides nao mapearam: %+v", p)
	}
	if p.Peso != "" {
		t.Errorf("override ausente devia ficar vazio: %+v", p)
	}
}

func TestObservadorRecebeCadaCiclo(t *testing.T) {
	e := novoEspiao()
	defer e.Close()

	var vistas int32
	f := &fonteFalsa{estados: []fonte.Estado{{Lote: "001"}}}
	l := Novo(f, backend.NovoCliente(e.URL, "k"), time.Millisecond,
		func(Situacao) { atomic.AddInt32(&vistas, 1) })

	parar := make(chan struct{})
	go func() { time.Sleep(30 * time.Millisecond); close(parar) }()
	_ = l.Rodar(parar)

	if atomic.LoadInt32(&vistas) == 0 {
		t.Error("o console nunca foi notificado")
	}
}
