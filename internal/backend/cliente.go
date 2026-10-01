// Package backend fala com a API do Gado Online.
//
// Tres chamadas, todas com a chave de transmissao no header Authorization. A
// chave identifica o leilao, entao nenhuma URL carrega leilao_id -- e' o que
// permite ao operador configurar um endereco so na maquina da transmissao.
package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrChaveRecusada e' 401: chave invalida, revogada ou ausente.
//
// Tratada em separado porque e' a unica falha que exige acao do operador --
// quem a recebe apaga a credencial do disco e pede uma nova.
var ErrChaveRecusada = errors.New("chave de transmissao recusada")

// Timeout curto de proposito: o laco roda a cada ~3s e o backend responde em
// milissegundos. Esperar 30s por uma resposta so atrasaria o ciclo seguinte.
const timeoutPadrao = 10 * time.Second

type Cliente struct {
	urlBase string
	chave   string
	http    *http.Client
}

func NovoCliente(urlBase, chave string) *Cliente {
	return &Cliente{
		urlBase: urlBase,
		chave:   chave,
		http:    &http.Client{Timeout: timeoutPadrao},
	}
}

// Leilao e' o que a rota /api/transmissao/leilao devolve.
type Leilao struct {
	ID         string `json:"id"`
	Nome       string `json:"nome"`
	DataLeilao string `json:"data_leilao"` // "2026-08-04"
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

// BuscarLeilao diz para qual leilao a chave aponta.
//
// Chamada na abertura por tres motivos: rotular a chave salva em disco, validar
// a credencial antes do leilao comecar em vez de descobrir no primeiro lote, e
// detectar chave revogada.
func (c *Cliente) BuscarLeilao() (Leilao, error) {
	corpo, err := c.requisitar(http.MethodGet, "/api/transmissao/leilao", nil)
	if err != nil {
		return Leilao{}, err
	}

	var leilao Leilao
	if err := json.Unmarshal(corpo, &leilao); err != nil {
		return Leilao{}, fmt.Errorf("resposta inesperada do servidor: %w", err)
	}
	return leilao, nil
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

	req.Header.Set("Authorization", "Bearer "+c.chave)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bruto, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrChaveRecusada
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
		return nil, errors.New(msg)
	}

	return env.Data, nil
}
