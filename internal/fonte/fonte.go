// Package fonte le o overlay do software de transmissao.
//
// O agente le o que esta RENDERIZADO no overlay, nao o que a Data Source
// mandou. E' isso que preserva a liberdade do operador: digitar por cima,
// anunciar um lote fora do catalogo, corrigir uma observacao na hora -- o site
// acompanha o que esta no ar.
package fonte

import "strings"

// Estado e' o conteudo atual do overlay.
//
// Representa a TELA INTEIRA, nao um delta. Campo vazio significa "vazio na
// tela", e o backend volta a usar o catalogo -- e' o que permite ao operador
// desfazer uma observacao digitada por engano.
type Estado struct {
	Lote  string
	Valor string

	// Chaves possiveis: qtd_animais, raca, sexo, idade, peso, obs.
	Overrides map[string]string
}

// Vazia diz que nao ha lote no overlay -- a pista deve ser liberada.
func (e Estado) Vazia() bool {
	return strings.TrimSpace(e.Lote) == ""
}

// Fonte e' de onde o estado vem: vMix, OBS ou arquivo simulado.
type Fonte interface {
	// Nome aparece no console de status ("vMix", "OBS", "simulado").
	Nome() string

	// Descricao e' o endereco ou arquivo, para o operador conferir de relance.
	Descricao() string

	// Ler devolve o estado atual. Erro aqui NAO derruba o agente: o laco loga e
	// tenta de novo no ciclo seguinte.
	Ler() (Estado, error)

	// Fechar libera conexoes (relevante para o OBS, que mantem WebSocket).
	Fechar() error
}

// limpar normaliza o que veio do overlay.
//
// O operador digita no vMix, entao espaco sobrando e' a regra, nao a excecao.
// Override vazio e' descartado aqui para nao trafegar: o backend tambem
// descarta, mas nao faz sentido mandar.
func limpar(estado Estado) Estado {
	estado.Lote = strings.TrimSpace(estado.Lote)
	estado.Valor = strings.TrimSpace(estado.Valor)

	limpos := make(map[string]string, len(estado.Overrides))
	for campo, valor := range estado.Overrides {
		if v := strings.TrimSpace(valor); v != "" {
			limpos[campo] = v
		}
	}
	estado.Overrides = limpos

	return estado
}
