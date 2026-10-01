package fonte

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Simulado le o estado de um arquivo JSON local, relendo a cada ciclo.
//
// E' o que permite desenvolver, testar e demonstrar o agente sem estar num
// leilao e sem Windows: edita-se o arquivo a mao e observa-se a pagina publica.
type Simulado struct {
	caminho string
}

func NovoSimulado(caminho string) *Simulado {
	return &Simulado{caminho: caminho}
}

func (s *Simulado) Nome() string      { return "simulado" }
func (s *Simulado) Descricao() string { return s.caminho }
func (s *Simulado) Fechar() error     { return nil }

type estadoJSON struct {
	Lote  string `json:"lote"`
	Valor string `json:"valor"`

	QtdAnimais string `json:"qtd_animais"`
	Raca       string `json:"raca"`
	Sexo       string `json:"sexo"`
	Idade      string `json:"idade"`
	Peso       string `json:"peso"`
	Obs        string `json:"obs"`
}

func (s *Simulado) Ler() (Estado, error) {
	bruto, err := os.ReadFile(s.caminho)
	if errors.Is(err, os.ErrNotExist) {
		// Arquivo ausente = overlay vazio, nao erro. Permite testar o
		// "liberar pista" apagando o arquivo.
		return Estado{}, nil
	}
	if err != nil {
		return Estado{}, err
	}

	if len(bytesUteis(bruto)) == 0 {
		return Estado{}, nil
	}

	var bruta estadoJSON
	if err := json.Unmarshal(bruto, &bruta); err != nil {
		return Estado{}, fmt.Errorf("%s nao e' JSON valido: %w", s.caminho, err)
	}

	return limpar(Estado{
		Lote:  bruta.Lote,
		Valor: bruta.Valor,
		Overrides: map[string]string{
			"qtd_animais": bruta.QtdAnimais,
			"raca":        bruta.Raca,
			"sexo":        bruta.Sexo,
			"idade":       bruta.Idade,
			"peso":        bruta.Peso,
			"obs":         bruta.Obs,
		},
	}), nil
}

func bytesUteis(b []byte) []byte {
	inicio, fim := 0, len(b)
	for inicio < fim && (b[inicio] == ' ' || b[inicio] == '\n' || b[inicio] == '\r' || b[inicio] == '\t') {
		inicio++
	}
	return b[inicio:fim]
}
