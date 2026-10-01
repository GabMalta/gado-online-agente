// Package laco e' o ciclo do agente: ler o overlay, enviar, repetir.
package laco

import (
	"errors"
	"time"

	"github.com/GabMalta/gado-online-agente/internal/backend"
	"github.com/GabMalta/gado-online-agente/internal/fonte"
)

const (
	// IntervaloPadrao: o TTL da pista no backend e' 120s, entao 3s da' 40
	// tentativas de margem antes de a pista expirar.
	IntervaloPadrao = 3 * time.Second

	// BackoffMax: teto do recuo em falha de rede. Fica abaixo do TTL de 120s
	// de proposito -- recuar mais que isso deixaria a pista expirar enquanto o
	// agente ainda esta "tentando".
	BackoffMax = 30 * time.Second
)

// Situacao e' o que o console mostra. Atualizada a cada ciclo.
type Situacao struct {
	FonteOK     bool
	ServidorOK  bool
	UltimoEnvio time.Time
	Estado      fonte.Estado
	Erro        string

	// ChaveRecusada sinaliza 401: o agente para e pede credencial nova.
	ChaveRecusada bool
}

// Observador recebe cada mudanca de situacao. E' o console de status.
type Observador func(Situacao)

type Laco struct {
	fonte       fonte.Fonte
	cliente     *backend.Cliente
	intervalo   time.Duration
	aoAtualizar Observador

	// Guarda se havia lote no ciclo anterior, para so chamar `liberar` na
	// transicao. Chamar a cada ciclo com overlay vazio funcionaria, mas seria
	// uma requisicao por segundo durante os intervalos entre lotes.
	tinhaLote bool
}

func Novo(f fonte.Fonte, c *backend.Cliente, intervalo time.Duration, obs Observador) *Laco {
	if intervalo <= 0 {
		intervalo = IntervaloPadrao
	}
	if obs == nil {
		obs = func(Situacao) {}
	}
	return &Laco{fonte: f, cliente: c, intervalo: intervalo, aoAtualizar: obs}
}

// Rodar bloqueia ate `parar` fechar ou a chave ser recusada.
//
// NUNCA devolve erro por falha de ciclo: o leilao esta ao vivo, e um erro de
// rede nao pode derrubar o processo. So para em 401, porque ai o operador
// precisa agir.
func (l *Laco) Rodar(parar <-chan struct{}) error {
	backoff := time.Duration(0)

	for {
		situacao, recusada := l.umCiclo()
		l.aoAtualizar(situacao)

		if recusada {
			return backend.ErrChaveRecusada
		}

		// Recuo progressivo so quando o servidor falha; a fonte local falhando
		// nao justifica atrasar (o vMix pode voltar a qualquer momento).
		espera := l.intervalo
		if !situacao.ServidorOK && situacao.Erro != "" {
			backoff = proximoBackoff(backoff, l.intervalo)
			espera = backoff
		} else {
			backoff = 0
		}

		select {
		case <-parar:
			return nil
		case <-time.After(espera):
		}
	}
}

func (l *Laco) umCiclo() (Situacao, bool) {
	situacao := Situacao{}

	estado, err := l.fonte.Ler()
	if err != nil {
		// Falha ao ler o overlay nao mexe na pista: melhor manter o lote
		// anterior no ar (o TTL cobre) que liberar por um erro transitorio.
		situacao.Erro = "nao consegui ler o " + l.fonte.Nome() + ": " + err.Error()
		return situacao, false
	}

	situacao.FonteOK = true
	situacao.Estado = estado

	if estado.Vazia() {
		if !l.tinhaLote {
			// Nada a fazer: overlay vazio e pista ja liberada.
			situacao.ServidorOK = true
			return situacao, false
		}

		if err := l.cliente.LiberarPista(); err != nil {
			return l.comErroDeServidor(situacao, err)
		}

		l.tinhaLote = false
		situacao.ServidorOK = true
		situacao.UltimoEnvio = time.Now()
		return situacao, false
	}

	// Reenvia mesmo sem mudanca: e' o heartbeat que segura o TTL de 120s.
	// Reenviar o mesmo estado e' idempotente no backend, entao nao ha
	// deduplicacao delicada a acertar aqui.
	if err := l.cliente.EnviarPista(paraPista(estado)); err != nil {
		return l.comErroDeServidor(situacao, err)
	}

	l.tinhaLote = true
	situacao.ServidorOK = true
	situacao.UltimoEnvio = time.Now()
	return situacao, false
}

func (l *Laco) comErroDeServidor(situacao Situacao, err error) (Situacao, bool) {
	situacao.Erro = err.Error()

	if errors.Is(err, backend.ErrChaveRecusada) {
		situacao.ChaveRecusada = true
		return situacao, true
	}
	return situacao, false
}

// paraPista traduz o estado do overlay no payload do backend.
//
// Mapa -> struct de proposito: o `omitempty` do struct e' o que garante que
// campo vazio nao viaje como override vazio.
func paraPista(estado fonte.Estado) backend.Pista {
	return backend.Pista{
		Lote:       estado.Lote,
		Valor:      estado.Valor,
		QtdAnimais: estado.Overrides["qtd_animais"],
		Raca:       estado.Overrides["raca"],
		Sexo:       estado.Overrides["sexo"],
		Idade:      estado.Overrides["idade"],
		Peso:       estado.Overrides["peso"],
		Obs:        estado.Overrides["obs"],
	}
}

func proximoBackoff(atual, base time.Duration) time.Duration {
	if atual <= 0 {
		return base
	}
	if dobro := atual * 2; dobro < BackoffMax {
		return dobro
	}
	return BackoffMax
}
