package fonte

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EnderecoVMixPadrao: a API do vMix fica na propria maquina do operador.
const EnderecoVMixPadrao = "http://127.0.0.1:8088/api/"

// VMix le o texto de um input de Title pela API HTTP do vMix.
//
// Le o que esta RENDERIZADO no Title, nao o que a Data Source mandou: e' isso
// que deixa o operador digitar por cima, anunciar lote fora do catalogo ou
// corrigir uma observacao na hora, com o site acompanhando.
type VMix struct {
	endereco string
	title    string
	campos   map[string]string
	http     *http.Client
}

func NovoVMix(title string, campos map[string]string) *VMix {
	return &VMix{
		endereco: EnderecoVMixPadrao,
		title:    title,
		campos:   campos,
		// Timeout curto: e' loopback. Se o vMix nao responde em 3s, ele esta
		// travado, e esperar mais so atrasaria o ciclo seguinte.
		http: &http.Client{Timeout: 3 * time.Second},
	}
}

func (v *VMix) Nome() string      { return "vMix" }
func (v *VMix) Descricao() string { return "127.0.0.1:8088 · " + v.title }
func (v *VMix) Fechar() error     { return nil }

// respostaVMix mapeia so o que interessa do XML de estado do vMix.
type respostaVMix struct {
	Inputs struct {
		Input []inputVMix `xml:"input"`
	} `xml:"inputs"`
}

type inputVMix struct {
	Key    string      `xml:"key,attr"`
	Title  string      `xml:"title,attr"`
	Tipo   string      `xml:"type,attr"`
	Textos []textoVMix `xml:"text"`
}

type textoVMix struct {
	Nome  string `xml:"name,attr"`
	Index string `xml:"index,attr"`
	Valor string `xml:",chardata"`
}

func (v *VMix) Ler() (Estado, error) {
	resp, err := v.http.Get(v.endereco)
	if err != nil {
		return Estado{}, fmt.Errorf("vMix não respondeu em %s: %w", v.endereco, err)
	}
	defer resp.Body.Close()

	bruto, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return Estado{}, err
	}

	var estado respostaVMix
	if err := xml.Unmarshal(bruto, &estado); err != nil {
		return Estado{}, fmt.Errorf("não entendi o XML do vMix: %w", err)
	}

	input := v.acharInput(estado.Inputs.Input)
	if input == nil {
		// Title ausente nao e' erro de conexao: o operador pode nao ter
		// carregado o Title ainda. Overlay vazio = pista livre.
		return Estado{}, nil
	}

	textos := indexarTextos(input.Textos)

	return limpar(Estado{
		Lote:  textos[v.campos["lote"]],
		Valor: textos[v.campos["valor"]],
		Overrides: map[string]string{
			"qtd_animais": textos[v.campos["qtd_animais"]],
			"raca":        textos[v.campos["raca"]],
			"sexo":        textos[v.campos["sexo"]],
			"idade":       textos[v.campos["idade"]],
			"peso":        textos[v.campos["peso"]],
			"obs":         textos[v.campos["obs"]],
		},
	}), nil
}

// acharInput localiza o Title configurado por `key` ou por nome.
//
// Compara por prefixo sem caixa porque o `title` do vMix costuma vir com a
// extensao do arquivo ("pista.gtzip") — exigir o nome exato faria o operador
// errar na config.
func (v *VMix) acharInput(inputs []inputVMix) *inputVMix {
	alvo := strings.ToLower(strings.TrimSpace(v.title))
	if alvo == "" {
		return nil
	}

	for i := range inputs {
		if strings.EqualFold(inputs[i].Key, alvo) {
			return &inputs[i]
		}
	}

	for i := range inputs {
		if strings.HasPrefix(strings.ToLower(inputs[i].Title), alvo) {
			return &inputs[i]
		}
	}
	return nil
}

// indexarTextos mapeia nome e índice do campo para o valor.
//
// Indexa pelos dois porque o `name` depende de o operador ter nomeado as camadas
// do Title; quando não nomeou, o `index` ("0", "1") é a única referência.
func indexarTextos(textos []textoVMix) map[string]string {
	indexados := make(map[string]string, len(textos)*2)

	for _, texto := range textos {
		if texto.Nome != "" {
			indexados[texto.Nome] = texto.Valor
		}
		if texto.Index != "" {
			indexados[texto.Index] = texto.Valor
		}
	}
	return indexados
}
