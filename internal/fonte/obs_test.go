package fonte

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// TestAutenticarOBS trava o desafio do obs-websocket v5.
//
// A senha, o salt e o challenge sao os do exemplo oficial em
// obs-websocket/docs/generated/protocol.md, secao "Creating an authentication
// string". Os docs NAO publicam a string final, entao o valor esperado abaixo
// foi derivado dos quatro passos documentados por uma implementacao
// independente, e confere com esta.
//
// Isso prova que o algoritmo nao vai regredir num refactor. NAO prova que
// conecta num OBS real com senha — isso precisa de um OBS de verdade, e esta
// anotado como pendente no README.
func TestAutenticarOBS(t *testing.T) {
	got := autenticarOBS(
		"supersecretpassword",
		"lM1GncleQOaCu9lT1yeUZhFYnqhsLLP1G5lAGo3ixaI=",
		"+IxH4CnCiqpX1rM9scsNynZzbOe4KhDeYcTNS3PDaeY=",
	)
	const esperado = "1Ct943GAT+6YQUUX47Ia/ncufilbe6+oD6lY+5kaCu4="

	if got != esperado {
		t.Errorf("hash de autenticacao errado:\n  veio      %s\n  esperado  %s", got, esperado)
	}
}

// obsFalso simula o OBS: Hello, Identified e respostas de GetInputSettings.
type obsFalso struct {
	*httptest.Server
	endereco string
	textos   map[string]string
	comSenha bool
	senha    string
}

func novoOBSFalso(t *testing.T, textos map[string]string) *obsFalso {
	t.Helper()

	f := &obsFalso{textos: textos}
	atualizador := websocket.Upgrader{}

	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := atualizador.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		hello := map[string]any{"op": opHello, "d": map[string]any{"rpcVersion": 1}}
		if f.comSenha {
			hello["d"] = map[string]any{
				"rpcVersion": 1,
				"authentication": map[string]string{
					"challenge": "desafio", "salt": "sal",
				},
			}
		}
		if err := conn.WriteJSON(hello); err != nil {
			return
		}

		var identify mensagemOBS
		if err := conn.ReadJSON(&identify); err != nil {
			return
		}

		if f.comSenha {
			var d struct{ Authentication string }
			_ = json.Unmarshal(identify.D, &d)
			if d.Authentication != autenticarOBS(f.senha, "sal", "desafio") {
				// Senha errada: o OBS fecha sem explicar.
				return
			}
		}

		_ = conn.WriteJSON(map[string]any{"op": opIdentified, "d": map[string]any{"negotiatedRpcVersion": 1}})

		for {
			var msg mensagemOBS
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			if msg.Op != opRequest {
				continue
			}

			var pedido struct {
				RequestID   string            `json:"requestId"`
				RequestData map[string]string `json:"requestData"`
			}
			_ = json.Unmarshal(msg.D, &pedido)

			source := pedido.RequestData["inputName"]
			texto, existe := f.textos[source]

			resposta := map[string]any{
				"requestId":     pedido.RequestID,
				"requestStatus": map[string]any{"result": existe, "code": 100},
				"responseData":  map[string]any{"inputSettings": map[string]string{"text": texto}},
			}
			if !existe {
				// 600 = ResourceNotFound
				resposta["requestStatus"] = map[string]any{
					"result": false, "code": 600, "comment": "No source was found by the name",
				}
			}

			_ = conn.WriteJSON(map[string]any{"op": opRequestResult, "d": resposta})
		}
	}))

	t.Cleanup(f.Close)
	f.endereco = strings.TrimPrefix(f.URL, "http://")
	return f
}

func TestOBSLeOsSources(t *testing.T) {
	f := novoOBSFalso(t, map[string]string{
		"Lote": "001", "Valor": "1850", "Obs": "",
	})

	o, err := NovoOBS(f.endereco, "", map[string]string{
		"lote": "Lote", "valor": "Valor", "obs": "Obs",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Fechar()

	estado, err := o.Ler()
	if err != nil {
		t.Fatal(err)
	}

	if estado.Lote != "001" || estado.Valor != "1850" {
		t.Errorf("estado inesperado: %+v", estado)
	}
	if _, tem := estado.Overrides["obs"]; tem {
		t.Errorf("obs vazia virou override: %v", estado.Overrides)
	}
}

func TestOBSComSenhaAutentica(t *testing.T) {
	f := novoOBSFalso(t, map[string]string{"Lote": "007"})
	f.comSenha, f.senha = true, "minha-senha"

	o, err := NovoOBS(f.endereco, "minha-senha", map[string]string{"lote": "Lote"})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Fechar()

	estado, err := o.Ler()
	if err != nil {
		t.Fatal(err)
	}
	if estado.Lote != "007" {
		t.Errorf("nao leu com senha: %+v", estado)
	}
}

func TestOBSExigindoSenhaSemSenhaConfiguradaExplica(t *testing.T) {
	f := novoOBSFalso(t, nil)
	f.comSenha, f.senha = true, "x"

	_, err := NovoOBS(f.endereco, "", map[string]string{"lote": "Lote"})

	if err == nil {
		t.Fatal("esperava erro")
	}
	// A mensagem precisa dizer ONDE resolver, nao so que falhou.
	if !strings.Contains(err.Error(), "WebSocket") {
		t.Errorf("mensagem pouco util: %v", err)
	}
}

func TestOBSSourceInexistenteNaoDerrubaOCiclo(t *testing.T) {
	// Configuracao errada e' do operador; nao pode virar falha de leitura que
	// esconde o lote que ESTA sendo lido.
	f := novoOBSFalso(t, map[string]string{"Lote": "001"})

	o, err := NovoOBS(f.endereco, "", map[string]string{
		"lote": "Lote", "obs": "SourceQueNaoExiste",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Fechar()

	estado, err := o.Ler()
	if err != nil {
		t.Fatalf("source inexistente nao devia derrubar a leitura: %v", err)
	}
	if estado.Lote != "001" {
		t.Errorf("perdeu o lote por causa de um source errado: %+v", estado)
	}
}

func TestOBSForaDoArDevolveErro(t *testing.T) {
	if _, err := NovoOBS("127.0.0.1:1", "", map[string]string{"lote": "Lote"}); err == nil {
		t.Fatal("esperava erro de conexao")
	}
}
