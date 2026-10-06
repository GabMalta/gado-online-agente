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

func TestChaveRecusadaVira401Tipado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"detail":"Unauthorized"}`)
	}))
	defer srv.Close()

	err := NovoCliente(srv.URL, "revogada").LiberarPista()

	// Tipado porque e' a unica falha do laco que exige acao do operador.
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

	err := NovoCliente(srv.URL, "k").LiberarPista()

	if !errors.Is(err, ErrRotaDesconhecida) {
		t.Fatalf("esperava ErrRotaDesconhecida, veio %v", err)
	}
}

func TestEntrarGuardaOTokenParaAsChamadasSeguintes(t *testing.T) {
	var login map[string]string
	var autorizacao string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("login nao devia mandar Authorization: %q", got)
			}
			_ = json.NewDecoder(r.Body).Decode(&login)
			_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":{"token":"jwt-1","user":{}}}`)
		case "/api/leilao/buscar":
			autorizacao = r.Header.Get("Authorization")
			_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":[]}`)
		}
	}))
	defer srv.Close()

	c := NovoClienteUsuario(srv.URL)
	if err := c.Entrar("joao", "segredo"); err != nil {
		t.Fatal(err)
	}
	if login["username"] != "joao" || login["password"] != "segredo" {
		t.Errorf("corpo do login errado: %v", login)
	}

	if _, err := c.ListarLeiloesAtivos(); err != nil {
		t.Fatal(err)
	}
	if autorizacao != "Bearer jwt-1" {
		t.Errorf("token do login nao foi usado: %q", autorizacao)
	}
}

func TestEntrarCom400ViraLoginInvalido(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":true,"message":"Usuário ou senha inválidos"}`)
	}))
	defer srv.Close()

	err := NovoClienteUsuario(srv.URL).Entrar("joao", "errada")

	if !errors.Is(err, ErrLoginInvalido) {
		t.Fatalf("esperava ErrLoginInvalido, veio %v", err)
	}
}

func TestServidorForaDoArViraErroConexao(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()

	err := NovoClienteUsuario(srv.URL).Entrar("joao", "x")

	// A abertura insiste so neste caso; senha errada nao pode virar repeticao.
	var conexao *ErroConexao
	if !errors.As(err, &conexao) {
		t.Fatalf("esperava ErroConexao, veio %T %v", err, err)
	}
}

func TestListarLeiloesAtivosFiltraEOrdena(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":[
			{"id":"a","nome":"FECHADO","data_leilao":"2026-10-01","status":"FECHADO","total_lotes":1},
			{"id":"b","nome":"AGUARDANDO TARDE","data_leilao":"2026-10-20","status":"AGUARDANDO","total_lotes":2},
			{"id":"c","nome":"FINALIZADO","data_leilao":"2026-10-02","status":"FINALIZADO","total_lotes":3},
			{"id":"d","nome":"AGUARDANDO CEDO","data_leilao":"2026-10-10","status":"AGUARDANDO","total_lotes":4},
			{"id":"e","nome":"AO VIVO","data_leilao":"2026-10-06","status":"EM_ANDAMENTO","total_lotes":5}
		]}`)
	}))
	defer srv.Close()

	leiloes, err := NovoClienteUsuario(srv.URL).ListarLeiloesAtivos()
	if err != nil {
		t.Fatal(err)
	}

	var ids string
	for _, leilao := range leiloes {
		ids += leilao.ID
	}
	if ids != "edb" {
		t.Errorf("esperava em andamento primeiro e aguardando por data (edb), veio %q", ids)
	}
}

func TestObterChaveReaproveitaAAtivaSemGerarNova(t *testing.T) {
	var metodos []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/aovivo/leilao/abc/chaves" {
			t.Errorf("rota errada: %q", r.URL.Path)
		}
		metodos = append(metodos, r.Method)
		_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":[
			{"id":2,"token":"revogada","ativa":false},
			{"id":1,"token":"atual","ativa":true}
		]}`)
	}))
	defer srv.Close()

	chave, err := NovoClienteUsuario(srv.URL).ObterChave("abc")
	if err != nil {
		t.Fatal(err)
	}
	if chave != "atual" {
		t.Errorf("esperava a chave ativa, veio %q", chave)
	}
	// Gerar chave revoga a anterior e derruba a URL do overlay ja configurada.
	if len(metodos) != 1 || metodos[0] != http.MethodGet {
		t.Errorf("so devia ter listado as chaves, chamou %v", metodos)
	}
}

func TestObterChaveGeraQuandoNaoHaAtiva(t *testing.T) {
	var metodos []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metodos = append(metodos, r.Method)
		if r.Method == http.MethodPost {
			_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":{"id":3,"token":"nova","ativa":true}}`)
			return
		}
		_, _ = io.WriteString(w, `{"error":false,"message":"ok","data":[{"id":1,"token":"velha","ativa":false}]}`)
	}))
	defer srv.Close()

	chave, err := NovoClienteUsuario(srv.URL).ObterChave("abc")
	if err != nil {
		t.Fatal(err)
	}
	if chave != "nova" {
		t.Errorf("esperava a chave gerada, veio %q", chave)
	}
	if len(metodos) != 2 || metodos[1] != http.MethodPost {
		t.Errorf("esperava GET e depois POST, veio %v", metodos)
	}
}

func TestTokenDeUsuarioRecusadoNaoViraChaveRecusada(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := NovoClienteUsuario(srv.URL).ListarLeiloesAtivos()

	if !errors.Is(err, ErrSessaoRecusada) {
		t.Fatalf("esperava ErrSessaoRecusada, veio %v", err)
	}
}
