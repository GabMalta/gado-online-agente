// Package config guarda as preferencias do agente no disco.
//
// Nenhuma credencial fica aqui: a senha e o token de usuario vivem so em
// memoria, e a chave de transmissao e' buscada no servidor a cada abertura. O
// arquivo mora no diretorio de configuracao do usuario -- %APPDATA% no
// Windows, ~/.config no Linux -- e nao ao lado do executavel, que e' copiado
// de maquina em maquina.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// URLPadrao e' sobrescrita em build com -ldflags para apontar para homologacao.
var URLPadrao = "https://backend.gadoonline.com.br"

const (
	pastaApp  = "GadoOnline"
	nomeArq   = "agente.json"
	permPasta = 0o700
	permArq   = 0o600
)

// Software de transmissao que o agente le.
const (
	SoftwareVMix = "vmix"
	SoftwareOBS  = "obs"
)

// Config e' o que persiste entre execucoes.
//
// `Usuario` e `Software` sao so conveniencia: o login vem preenchido e o menu
// abre no software da ultima vez. Config antiga com `chave` salva perde o
// campo na proxima gravacao -- credencial nao fica mais em disco.
type Config struct {
	URLBase  string `json:"url_base"`
	Usuario  string `json:"usuario,omitempty"`
	Software string `json:"software,omitempty"`

	// Nome do input de Title no vMix e dos campos dentro dele. Vem da config e
	// nao hardcoded: renomear um Title no vMix nao pode quebrar o ingest em
	// silencio -- o operador troca aqui e o agente volta a achar.
	VMixTitle  string            `json:"vmix_title,omitempty"`
	VMixCampos map[string]string `json:"vmix_campos,omitempty"`

	// OBS: nome de cada source de texto por campo do payload.
	OBSEndereco string            `json:"obs_endereco,omitempty"`
	OBSSenha    string            `json:"obs_senha,omitempty"`
	OBSCampos   map[string]string `json:"obs_campos,omitempty"`
}

// ErrSemConfig indica que ainda nao existe arquivo -- primeira execucao.
var ErrSemConfig = errors.New("config: nenhuma configuracao salva")

// Caminho devolve o arquivo de config do usuario atual.
func Caminho() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, pastaApp, nomeArq), nil
}

// Padrao e' a config de primeira execucao, com os defaults ja aplicados.
//
// Existe porque devolver `Config{}` cru junto com o erro deixava `URLBase`
// vazia, e o agente tentava `GET /api/transmissao/leilao` sem host --
// "unsupported protocol scheme". Quem recebe um erro daqui tem que receber
// uma config utilizavel junto.
func Padrao() Config {
	var cfg Config
	cfg.aplicarPadroes()
	return cfg
}

// CriarSeAusente grava a config padrao quando ainda nao ha arquivo.
//
// Chamado na abertura: a tela mostra o caminho do arquivo, e quem precisa
// trocar o `url_base` antes do primeiro login (homologacao, servidor novo) tem
// que achar o arquivo la. Arquivo existente -- mesmo corrompido -- nao e'
// tocado: sobrescrever um JSON com virgula errada apagaria a edicao do
// operador sem aviso.
func CriarSeAusente() error {
	caminho, err := Caminho()
	if err != nil {
		return err
	}

	if _, err := os.Stat(caminho); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return Salvar(Padrao())
}

// Carregar le a config do disco.
//
// Devolve ErrSemConfig na primeira execucao -- e nesse caso devolve tambem a
// config padrao, nao a zerada.
func Carregar() (Config, error) {
	caminho, err := Caminho()
	if err != nil {
		return Padrao(), err
	}

	bruto, err := os.ReadFile(caminho)
	if errors.Is(err, os.ErrNotExist) {
		return Padrao(), ErrSemConfig
	}
	if err != nil {
		return Padrao(), err
	}

	var cfg Config
	if err := json.Unmarshal(bruto, &cfg); err != nil {
		// Arquivo corrompido nao pode travar o agente as 19h de um sabado:
		// trata como primeira execucao.
		return Padrao(), ErrSemConfig
	}

	cfg.aplicarPadroes()
	return cfg, nil
}

// Salvar grava a config com permissao restrita ao usuario.
func Salvar(cfg Config) error {
	caminho, err := Caminho()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(caminho), permPasta); err != nil {
		return err
	}

	cfg.aplicarPadroes()

	bruto, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Escreve em arquivo temporario e renomeia: queda de luz no meio da
	// gravacao nao deixa uma config meio escrita.
	tmp := caminho + ".tmp"
	if err := os.WriteFile(tmp, bruto, permArq); err != nil {
		return err
	}
	return os.Rename(tmp, caminho)
}

func (c *Config) aplicarPadroes() {
	c.Usuario = strings.TrimSpace(c.Usuario)
	c.URLBase = strings.TrimRight(strings.TrimSpace(c.URLBase), "/")

	if c.URLBase == "" {
		c.URLBase = URLPadrao
	}
	if c.Software != SoftwareVMix && c.Software != SoftwareOBS {
		c.Software = ""
	}
	if c.VMixTitle == "" {
		c.VMixTitle = "pista"
	}
	if len(c.VMixCampos) == 0 {
		c.VMixCampos = CamposPadraoVMix()
	}
	if c.OBSEndereco == "" {
		c.OBSEndereco = "localhost:4455"
	}
	if len(c.OBSCampos) == 0 {
		c.OBSCampos = CamposPadraoOBS()
	}
}

// CamposPadraoVMix mapeia campo do payload -> nome do campo de texto no Title.
func CamposPadraoVMix() map[string]string {
	return map[string]string{
		"lote":        "Lote.Text",
		"valor":       "Valor.Text",
		"obs":         "Obs.Text",
		"qtd_animais": "Qtd.Text",
		"raca":        "Raca.Text",
		"sexo":        "Sexo.Text",
		"idade":       "Idade.Text",
		"peso":        "Peso.Text",
	}
}

// CamposPadraoOBS mapeia campo do payload -> nome do source de texto no OBS.
func CamposPadraoOBS() map[string]string {
	return map[string]string{
		"lote":  "Lote",
		"valor": "Valor",
		"obs":   "Obs",
	}
}
