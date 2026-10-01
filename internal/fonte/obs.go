package fonte

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// OBS le o texto de sources do OBS pelo obs-websocket v5 (embutido desde o
// OBS 28).
//
// Protocolo falado direto em vez de usar um cliente pronto: o unico pedido
// necessario e' `GetInputSettings`, e a biblioteca mais comum trazia 6
// dependencias transitivas — incluindo uma de profiling e uma abandonada desde
// 2013. Em binario que vai para a maquina do cliente, cada dependencia e'
// superficie.
type OBS struct {
	endereco string
	senha    string
	campos   map[string]string

	conn      *websocket.Conn
	proximoID atomic.Int64
}

// Opcodes do obs-websocket v5.
const (
	opHello         = 0
	opIdentify      = 1
	opIdentified    = 2
	opRequest       = 6
	opRequestResult = 7
)

func NovoOBS(endereco, senha string, campos map[string]string) (*OBS, error) {
	o := &OBS{endereco: endereco, senha: senha, campos: campos}
	if err := o.conectar(); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *OBS) Nome() string      { return "OBS" }
func (o *OBS) Descricao() string { return o.endereco }

func (o *OBS) Fechar() error {
	if o.conn == nil {
		return nil
	}
	err := o.conn.Close()
	o.conn = nil
	return err
}

type mensagemOBS struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
}

type dadosHello struct {
	RPCVersion int `json:"rpcVersion"`
	Auth       *struct {
		Challenge string `json:"challenge"`
		Salt      string `json:"salt"`
	} `json:"authentication"`
}

func (o *OBS) conectar() error {
	u := url.URL{Scheme: "ws", Host: o.endereco}

	discador := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := discador.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("não consegui conectar no OBS em %s: %w", o.endereco, err)
	}

	o.conn = conn
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var hello mensagemOBS
	if err := conn.ReadJSON(&hello); err != nil {
		_ = o.Fechar()
		return fmt.Errorf("o OBS não mandou Hello: %w", err)
	}

	var dados dadosHello
	if err := json.Unmarshal(hello.D, &dados); err != nil {
		_ = o.Fechar()
		return err
	}

	identify := map[string]any{
		"rpcVersion": dados.RPCVersion,
		// Zero eventos: o agente faz polling e nao precisa de nenhum. Assinar
		// eventos encheria a conexao de mensagens que seriam descartadas.
		"eventSubscriptions": 0,
	}

	if dados.Auth != nil {
		if o.senha == "" {
			_ = o.Fechar()
			return fmt.Errorf("o OBS exige senha e nenhuma foi configurada " +
				"(Ferramentas → Configurações do Servidor WebSocket)")
		}
		identify["authentication"] = autenticarOBS(o.senha, dados.Auth.Salt, dados.Auth.Challenge)
	}

	if err := conn.WriteJSON(mensagemComDados(opIdentify, identify)); err != nil {
		_ = o.Fechar()
		return err
	}

	var identificado mensagemOBS
	if err := conn.ReadJSON(&identificado); err != nil {
		_ = o.Fechar()
		// Senha errada fecha a conexao sem explicar; a dica vale mais que o erro cru.
		return fmt.Errorf("o OBS recusou a conexão (senha do WebSocket?): %w", err)
	}
	if identificado.Op != opIdentified {
		_ = o.Fechar()
		return fmt.Errorf("o OBS respondeu op %d em vez de Identified", identificado.Op)
	}

	_ = conn.SetReadDeadline(time.Time{})
	return nil
}

// autenticarOBS implementa o desafio do obs-websocket v5:
// base64(sha256(base64(sha256(senha+salt)) + challenge)).
func autenticarOBS(senha, salt, challenge string) string {
	segredo := sha256.Sum256([]byte(senha + salt))
	codificado := base64.StdEncoding.EncodeToString(segredo[:])

	resposta := sha256.Sum256([]byte(codificado + challenge))
	return base64.StdEncoding.EncodeToString(resposta[:])
}

func (o *OBS) Ler() (Estado, error) {
	if o.conn == nil {
		// Reconecta sozinho: o operador pode ter reiniciado o OBS no meio do
		// leilao, e isso nao pode exigir reiniciar o agente.
		if err := o.conectar(); err != nil {
			return Estado{}, err
		}
	}

	valores := make(map[string]string, len(o.campos))

	for campo, source := range o.campos {
		if source == "" {
			continue
		}

		texto, err := o.textoDoSource(source)
		if err != nil {
			// Conexao caiu: fecha para o proximo ciclo reconectar.
			_ = o.Fechar()
			return Estado{}, err
		}
		valores[campo] = texto
	}

	return limpar(Estado{
		Lote:  valores["lote"],
		Valor: valores["valor"],
		Overrides: map[string]string{
			"qtd_animais": valores["qtd_animais"],
			"raca":        valores["raca"],
			"sexo":        valores["sexo"],
			"idade":       valores["idade"],
			"peso":        valores["peso"],
			"obs":         valores["obs"],
		},
	}), nil
}

type respostaPedido struct {
	RequestID string `json:"requestId"`
	Status    struct {
		Result  bool   `json:"result"`
		Code    int    `json:"code"`
		Comment string `json:"comment"`
	} `json:"requestStatus"`
	// `GetInputSettings` nao inclui defaults (confirmado nos docs do
	// protocolo): num source onde o operador nunca digitou, `text` vem AUSENTE
	// e nao vazio. Cai em "" aqui, que e' exatamente o que queremos — ausente
	// na tela significa pista sem aquele campo.
	ResponseData struct {
		InputSettings struct {
			Text string `json:"text"`
		} `json:"inputSettings"`
	} `json:"responseData"`
}

func (o *OBS) textoDoSource(source string) (string, error) {
	id := fmt.Sprintf("r%d", o.proximoID.Add(1))

	pedido := mensagemComDados(opRequest, map[string]any{
		"requestType": "GetInputSettings",
		"requestId":   id,
		"requestData": map[string]string{"inputName": source},
	})

	_ = o.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if err := o.conn.WriteJSON(pedido); err != nil {
		return "", err
	}

	_ = o.conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// Descarta mensagens que nao sejam a resposta deste pedido. Com
	// eventSubscriptions=0 isso quase nao acontece, mas o OBS ainda manda
	// mensagens de protocolo.
	for tentativas := 0; tentativas < 10; tentativas++ {
		var msg mensagemOBS
		if err := o.conn.ReadJSON(&msg); err != nil {
			return "", err
		}
		if msg.Op != opRequestResult {
			continue
		}

		var resp respostaPedido
		if err := json.Unmarshal(msg.D, &resp); err != nil {
			return "", err
		}
		if resp.RequestID != id {
			continue
		}

		if !resp.Status.Result {
			// Source inexistente e' configuracao errada, nao falha de conexao:
			// trata como campo vazio para nao derrubar o ciclo inteiro.
			if resp.Status.Code == 600 {
				return "", nil
			}
			return "", fmt.Errorf("OBS recusou ler %q: %s", source, resp.Status.Comment)
		}

		return resp.ResponseData.InputSettings.Text, nil
	}

	return "", fmt.Errorf("o OBS não respondeu ao pedido de %q", source)
}

func mensagemComDados(op int, dados any) map[string]any {
	return map[string]any{"op": op, "d": dados}
}
