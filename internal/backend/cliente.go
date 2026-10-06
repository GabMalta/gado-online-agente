// Package backend fala com a API do Gado Online.
//
// Dois clientes. `Cliente` usa a chave de transmissao e e' o que roda durante o
// leilao: a chave identifica o leilao, entao nenhuma URL carrega leilao_id.
// `ClienteUsuario` usa o login do operador e so existe na abertura, para
// escolher o leilao e buscar a chave dele.
package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

// ErrChaveRecusada e' 401: chave invalida, revogada ou ausente.
//
// Tratada em separado porque e' a unica falha do laco que exige acao do
// operador -- abrir o agente de novo, que busca a chave atual do leilao.
var ErrChaveRecusada = errors.New("chave de transmissao recusada")

// ErrRotaDesconhecida e' 404 nas rotas que o agente usa.
//
// Separado porque a causa quase sempre e' uma so: o backend esta numa versao
// anterior a estas rotas. Dizer "nao consegui falar com o servidor" nesse caso
// seria enganoso -- ele falou, e foi respondido.
var ErrRotaDesconhecida = errors.New("o servidor nao conhece as rotas de transmissao")

// ErrLoginInvalido e' o 400 do login: usuario ou senha errados.
var ErrLoginInvalido = errors.New("usuario ou senha invalidos")

// ErrSessaoRecusada e' 401 com o token de usuario: o login valeu, mas o usuario
// foi desativado no meio da abertura.
var ErrSessaoRecusada = errors.New("o servidor recusou o seu usuario (ele foi desativado?)")

// ErroConexao e' a falha de rede: o servidor nao respondeu.
//
// Tipado porque a abertura trata diferente de um erro que o servidor devolveu:
// sem conexao ela insiste (a internet do parque demora a subir); com resposta
// de erro, insistir nao adianta. A mensagem e' a original, que e' o que o
// painel de status mostra.
type ErroConexao struct{ Err error }

func (e *ErroConexao) Error() string { return e.Err.Error() }
func (e *ErroConexao) Unwrap() error { return e.Err }

// Status de leilao que aparecem para o agente -- os mesmos do catalogo
// publico (`Leilao.STATUS_PUBLICOS` no backend). FINALIZADO e FECHADO nao
// recebem mais lote em pista.
const (
	StatusAguardando  = "AGUARDANDO"
	StatusEmAndamento = "EM_ANDAMENTO"
)

// Timeout curto de proposito: o laco roda a cada ~3s e o backend responde em
// milissegundos. Esperar 30s por uma resposta so atrasaria o ciclo seguinte.
const timeoutPadrao = 10 * time.Second

type Cliente struct {
	urlBase string
	token   string
	http    *http.Client

	// recusado e' o erro devolvido em 401: chave recusada ou usuario recusado,
	// conforme a credencial que o cliente carrega.
	recusado error
}

func NovoCliente(urlBase, chave string) *Cliente {
	return &Cliente{
		urlBase:  urlBase,
		token:    chave,
		http:     &http.Client{Timeout: timeoutPadrao},
		recusado: ErrChaveRecusada,
	}
}

// Leilao e' o recorte de `/api/leilao/buscar` que o agente usa.
type Leilao struct {
	ID         string `json:"id"`
	Nome       string `json:"nome"`
	DataLeilao string `json:"data_leilao"` // "2026-08-04"
	Status     string `json:"status"`
	TotalLotes int    `json:"total_lotes"`
}

// Pista e' o estado completo do overlay a ser enviado.
//
// `omitempty` em tudo menos `lote` e' deliberado: campo ausente significa
// "ausente na tela", e o backend volta a usar o catalogo. E' o que faz o
// operador desfazer uma obs digitada por engano -- ela sai do overlay e sai do
// site no envio seguinte.
type Pista struct {
	Lote  string `json:"lote"`
	Valor string `json:"valor,omitempty"`

	QtdAnimais string `json:"qtd_animais,omitempty"`
	Raca       string `json:"raca,omitempty"`
	Sexo       string `json:"sexo,omitempty"`
	Idade      string `json:"idade,omitempty"`
	Peso       string `json:"peso,omitempty"`
	Obs        string `json:"obs,omitempty"`
}

type envelope struct {
	Error   bool            `json:"error"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// EnviarPista anuncia o lote em pista. Reenviar o mesmo estado e' inofensivo e
// e' justamente o heartbeat que segura o TTL do backend.
func (c *Cliente) EnviarPista(pista Pista) error {
	_, err := c.requisitar(http.MethodPost, "/api/transmissao/pista", pista)
	return err
}

// LiberarPista deixa a pista vazia -- o card volta para "Aguardando proximo
// lote". Caminho normal de fim de lote; o TTL e' so a rede de protecao.
func (c *Cliente) LiberarPista() error {
	_, err := c.requisitar(http.MethodPost, "/api/transmissao/pista/liberar", nil)
	return err
}

func (c *Cliente) requisitar(metodo, rota string, payload any) (json.RawMessage, error) {
	var corpo io.Reader

	if payload != nil {
		bruto, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		corpo = bytes.NewReader(bruto)
	}

	req, err := http.NewRequest(metodo, c.urlBase+rota, corpo)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &ErroConexao{Err: err}
	}
	defer resp.Body.Close()

	bruto, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, c.recusado
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrRotaDesconhecida
	}

	var env envelope
	// O envelope pode nao vir em erro de infraestrutura (502 de proxy, por
	// exemplo), entao o status vale mais que o parse.
	_ = json.Unmarshal(bruto, &env)

	if resp.StatusCode >= 400 || env.Error {
		msg := env.Message
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, &ErroServidor{Status: resp.StatusCode, Mensagem: msg}
	}

	return env.Data, nil
}

// ErroServidor e' a resposta de erro do backend, com a mensagem dele.
type ErroServidor struct {
	Status   int
	Mensagem string
}

func (e *ErroServidor) Error() string { return e.Mensagem }

// ClienteUsuario fala com o backend com o login do operador.
//
// O token fica so em memoria, e so durante a abertura. O JWT do projeto nao
// expira e da acesso a empresa inteira -- e' exatamente o que a chave de
// transmissao existe para nao deixar num arquivo da maquina do parque.
type ClienteUsuario struct {
	*Cliente
}

func NovoClienteUsuario(urlBase string) *ClienteUsuario {
	return &ClienteUsuario{Cliente: &Cliente{
		urlBase:  urlBase,
		http:     &http.Client{Timeout: timeoutPadrao},
		recusado: ErrSessaoRecusada,
	}}
}

// Entrar faz o login e guarda o token para as chamadas seguintes.
func (c *ClienteUsuario) Entrar(usuario, senha string) error {
	c.token = ""

	corpo, err := c.requisitar(http.MethodPost, "/api/auth/login", map[string]string{
		"username": usuario,
		"password": senha,
	})

	var erroServidor *ErroServidor
	if errors.As(err, &erroServidor) && erroServidor.Status == http.StatusBadRequest {
		return ErrLoginInvalido
	}
	if err != nil {
		return err
	}

	var dados struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(corpo, &dados); err != nil || dados.Token == "" {
		return errors.New("resposta inesperada do servidor no login")
	}

	c.token = dados.Token
	return nil
}

// ListarLeiloesAtivos devolve os leiloes da empresa que ainda recebem lote em
// pista: em andamento primeiro, depois os aguardando, cada grupo por data.
func (c *ClienteUsuario) ListarLeiloesAtivos() ([]Leilao, error) {
	corpo, err := c.requisitar(http.MethodGet, "/api/leilao/buscar", nil)
	if err != nil {
		return nil, err
	}

	var todos []Leilao
	if err := json.Unmarshal(corpo, &todos); err != nil {
		return nil, fmt.Errorf("resposta inesperada do servidor: %w", err)
	}

	ativos := make([]Leilao, 0, len(todos))
	for _, leilao := range todos {
		if leilao.Status == StatusAguardando || leilao.Status == StatusEmAndamento {
			ativos = append(ativos, leilao)
		}
	}

	sort.SliceStable(ativos, func(i, j int) bool {
		a, b := ativos[i], ativos[j]
		if a.Status != b.Status {
			return a.Status == StatusEmAndamento
		}
		return a.DataLeilao < b.DataLeilao
	})
	return ativos, nil
}

type chaveTransmissao struct {
	Token string `json:"token"`
	Ativa bool   `json:"ativa"`
}

// ObterChave devolve a chave de transmissao ativa do leilao, e so gera uma
// quando nao ha nenhuma.
//
// Reaproveitar e' obrigatorio, nao economia: gerar chave revoga as anteriores
// (`criar_chave` no backend), e a chave antiga e' a que esta na URL do overlay
// configurada no Browser Source do OBS e na Data Source do vMix. Abrir o
// agente nao pode derrubar o overlay.
func (c *ClienteUsuario) ObterChave(leilaoID string) (string, error) {
	rota := "/api/aovivo/leilao/" + leilaoID + "/chaves"

	corpo, err := c.requisitar(http.MethodGet, rota, nil)
	if err != nil {
		return "", err
	}

	var chaves []chaveTransmissao
	if err := json.Unmarshal(corpo, &chaves); err != nil {
		return "", fmt.Errorf("resposta inesperada do servidor: %w", err)
	}

	for _, chave := range chaves {
		if chave.Ativa && chave.Token != "" {
			return chave.Token, nil
		}
	}

	corpo, err = c.requisitar(http.MethodPost, rota, nil)
	if err != nil {
		return "", err
	}

	var nova chaveTransmissao
	if err := json.Unmarshal(corpo, &nova); err != nil || nova.Token == "" {
		return "", errors.New("o servidor nao devolveu a chave de transmissao")
	}
	return nova.Token, nil
}
