package backend

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuscarLeilao(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer minha-chave" {
			t.Errorf("header Authorization errado: %q", got)
		}
		if r.URL.Path != "/api/transmissao/leilao" {
			t.Errorf("rota errada: %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":
			{"id":"abc","nome":"LEILAO REAL","data_leilao":"2026-08-04","total_lotes":90}}`)
	}))
	defer srv.Close()

	leilao, err := NovoCliente(srv.URL, "minha-chave").BuscarLeilao()
	if err != nil {
		t.Fatal(err)
	}

	if leilao.Nome != "LEILAO REAL" || leilao.TotalLotes != 90 {
		t.Errorf("leilao inesperado: %+v", leilao)
	}
	if leilao.DataLeilao != "2026-08-04" {
		t.Errorf("data inesperada: %q", leilao.DataLeilao)
	}
}

func TestChaveRecusadaVira401Tipado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"detail":"Unauthorized"}`)
	}))
	defer srv.Close()

	_, err := NovoCliente(srv.URL, "revogada").BuscarLeilao()

	// Tipado porque e' a unica falha que exige acao: apagar a chave do disco.
	if !errors.Is(err, ErrChaveRecusada) {
		t.Fatalf("esperava ErrChaveRecusada, veio %v", err)
	}
}

func TestEnviarPistaOmiteCamposVazios(t *testing.T) {
	var recebido map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&recebido)
		_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":null}`)
	}))
	defer srv.Close()

	err := NovoCliente(srv.URL, "k").EnviarPista(Pista{Lote: "001", Valor: "1850"})
	if err != nil {
		t.Fatal(err)
	}

	// Campo ausente = "ausente na tela"; mandar "obs":"" faria o backend
	// receber um override vazio em vez de cair no catalogo.
	if _, existe := recebido["obs"]; existe {
		t.Errorf("obs vazia nao devia ser enviada: %v", recebido)
	}
	if recebido["lote"] != "001" {
		t.Errorf("lote nao chegou: %v", recebido)
	}
}

func TestEnviarPistaMandaOverridePreenchido(t *testing.T) {
	var recebido map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&recebido)
		_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":null}`)
	}))
	defer srv.Close()

	_ = NovoCliente(srv.URL, "k").EnviarPista(Pista{Lote: "001", Obs: "Reagrupado"})

	if recebido["obs"] != "Reagrupado" {
		t.Errorf("override nao chegou: %v", recebido)
	}
}

func TestErroDoBackendVemComAMensagem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w,
			`{"error":true,"message":"Numero do lote e' obrigatorio","data":null}`)
	}))
	defer srv.Close()

	err := NovoCliente(srv.URL, "k").EnviarPista(Pista{Lote: " "})

	if err == nil || err.Error() != "Numero do lote e' obrigatorio" {
		t.Fatalf("esperava a mensagem do backend, veio %v", err)
	}
}

func TestRespostaSemEnvelopeNaoViraSucesso(t *testing.T) {
	// 502 de proxy: HTML, nao JSON. O status tem que valer mais que o parse.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "<html>Bad Gateway</html>")
	}))
	defer srv.Close()

	if err := NovoCliente(srv.URL, "k").LiberarPista(); err == nil {
		t.Fatal("502 com corpo HTML passou como sucesso")
	}
}

func TestTimeoutDevolveErroEnaoTrava(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	c := NovoCliente(srv.URL, "k")
	c.http.Timeout = 50 * time.Millisecond

	if err := c.LiberarPista(); err == nil {
		t.Fatal("esperava erro de timeout")
	}
}

func TestLiberarPistaChamaARotaCerta(t *testing.T) {
	var rota, metodo string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rota, metodo = r.URL.Path, r.Method
		_, _ = io.WriteString(w, `{"error":false,"message":"Pista liberada","data":null}`)
	}))
	defer srv.Close()

	if err := NovoCliente(srv.URL, "k").LiberarPista(); err != nil {
		t.Fatal(err)
	}
	if rota != "/api/transmissao/pista/liberar" || metodo != http.MethodPost {
		t.Errorf("chamada errada: %s %s", metodo, rota)
	}
}

func Test404ViraErroTipadoEnaoMensagemDeRedeFalsa(t *testing.T) {
	// Backend mais antigo que o agente: a rota nao existe. Dizer "nao consegui
	// falar com o servidor" seria enganoso -- ele falou e foi respondido.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"detail":"Not Found"}`)
	}))
	defer srv.Close()

	_, err := NovoCliente(srv.URL, "k").BuscarLeilao()

	if !errors.Is(err, ErrRotaDesconhecida) {
		t.Fatalf("esperava ErrRotaDesconhecida, veio %v", err)
	}
}
