package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolar aponta o diretorio de config para um temporario do teste, para nao
// escrever no %APPDATA% real de quem roda os testes.
func isolar(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux/macOS
	t.Setenv("AppData", dir)         // Windows
}

func TestCarregarSemArquivoDevolveErrSemConfig(t *testing.T) {
	isolar(t)

	if _, err := Carregar(); !errors.Is(err, ErrSemConfig) {
		t.Fatalf("esperava ErrSemConfig, veio %v", err)
	}
}

func TestSalvarECarregar(t *testing.T) {
	isolar(t)

	if err := Salvar(Config{Usuario: "  joao  ", Software: SoftwareOBS, URLBase: "http://localhost:8899/"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Carregar()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Usuario != "joao" {
		t.Errorf("usuario nao foi trimado: %q", cfg.Usuario)
	}
	if cfg.Software != SoftwareOBS {
		t.Errorf("software nao foi salvo: %q", cfg.Software)
	}
	// Barra no fim viraria //api/... nas requisicoes.
	if cfg.URLBase != "http://localhost:8899" {
		t.Errorf("barra final nao foi removida: %q", cfg.URLBase)
	}
}

func TestURLBaseVaziaCaiNoPadrao(t *testing.T) {
	isolar(t)

	if err := Salvar(Config{Usuario: "joao"}); err != nil {
		t.Fatal(err)
	}

	cfg, _ := Carregar()
	if cfg.URLBase != URLPadrao {
		t.Errorf("esperava %q, veio %q", URLPadrao, cfg.URLBase)
	}
}

func TestCamposPadraoSaoPreenchidos(t *testing.T) {
	isolar(t)

	_ = Salvar(Config{Usuario: "joao"})
	cfg, _ := Carregar()

	if cfg.VMixCampos["lote"] == "" {
		t.Error("campos padrao do vMix nao foram preenchidos")
	}
	if cfg.VMixTitle == "" {
		t.Error("title padrao do vMix nao foi preenchido")
	}
}

func TestSoftwareDesconhecidoCaiNoMenuSemPreSelecao(t *testing.T) {
	isolar(t)

	_ = Salvar(Config{Software: "wirecast"})
	cfg, _ := Carregar()

	if cfg.Software != "" {
		t.Errorf("software invalido devia virar vazio, veio %q", cfg.Software)
	}
}

func TestConfigAntigaComChaveNaoGuardaMaisACredencial(t *testing.T) {
	isolar(t)

	caminho, _ := Caminho()
	_ = os.MkdirAll(filepath.Dir(caminho), permPasta)
	_ = os.WriteFile(caminho, []byte(`{"chave":"antiga","url_base":"http://x","vmix_title":"meu-title"}`), permArq)

	cfg, err := Carregar()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VMixTitle != "meu-title" {
		t.Errorf("o resto da config foi perdido: %+v", cfg)
	}
	if err := Salvar(cfg); err != nil {
		t.Fatal(err)
	}

	bruto, _ := os.ReadFile(caminho)
	if strings.Contains(string(bruto), "antiga") {
		t.Errorf("a chave antiga continuou no disco:\n%s", bruto)
	}
}

func TestArquivoCorrompidoNaoTravaOAgente(t *testing.T) {
	isolar(t)

	caminho, _ := Caminho()
	if err := os.MkdirAll(filepath.Dir(caminho), permPasta); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte("{isso nao e json"), permArq); err != nil {
		t.Fatal(err)
	}

	// Tratado como primeira execucao em vez de morrer as 19h de um sabado.
	if _, err := Carregar(); !errors.Is(err, ErrSemConfig) {
		t.Fatalf("esperava ErrSemConfig, veio %v", err)
	}
}

func TestSalvarNaoDeixaArquivoTemporario(t *testing.T) {
	isolar(t)

	if err := Salvar(Config{Usuario: "joao"}); err != nil {
		t.Fatal(err)
	}

	caminho, _ := Caminho()
	if _, err := os.Stat(caminho + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Error("o .tmp da escrita atomica sobrou no disco")
	}
}

// TestPrimeiraExecucaoTemURLBase cobre o bug que foi para producao: `Carregar`
// devolvia `Config{}` cru junto com ErrSemConfig, e o agente tentava
// `GET /api/transmissao/leilao` sem host -- "unsupported protocol scheme".
//
// Nenhum teste pegava porque todos passavam URLBase explicito, e os testes
// ponta a ponta sempre usavam --servidor. O caminho mais comum de todos
// (usuario novo dando dois cliques no .exe) nao era exercitado.
func TestPrimeiraExecucaoTemURLBase(t *testing.T) {
	isolar(t)

	cfg, err := Carregar()

	if !errors.Is(err, ErrSemConfig) {
		t.Fatalf("esperava ErrSemConfig, veio %v", err)
	}
	if cfg.URLBase != URLPadrao {
		t.Errorf("URLBase na primeira execucao: %q (esperava %q)", cfg.URLBase, URLPadrao)
	}
	if cfg.VMixTitle == "" || len(cfg.VMixCampos) == 0 {
		t.Errorf("defaults do vMix nao foram aplicados: %+v", cfg)
	}
}

func TestConfigCorrompidaTambemTemURLBase(t *testing.T) {
	isolar(t)

	caminho, _ := Caminho()
	_ = os.MkdirAll(filepath.Dir(caminho), permPasta)
	_ = os.WriteFile(caminho, []byte("{nao e json"), permArq)

	cfg, err := Carregar()

	if !errors.Is(err, ErrSemConfig) {
		t.Fatalf("esperava ErrSemConfig, veio %v", err)
	}
	if cfg.URLBase == "" {
		t.Error("URLBase vazia depois de config corrompida")
	}
}

func TestPadraoEhUtilizavel(t *testing.T) {
	cfg := Padrao()

	if cfg.URLBase == "" {
		t.Error("URLBase vazia")
	}
}

func TestCriarSeAusenteGravaOPadraoNaPrimeiraExecucao(t *testing.T) {
	isolar(t)

	if err := CriarSeAusente(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Carregar()
	if err != nil {
		t.Fatalf("o arquivo devia existir depois de CriarSeAusente, veio %v", err)
	}
	if cfg.URLBase != URLPadrao {
		t.Errorf("url_base devia ser o padrao, veio %q", cfg.URLBase)
	}
}

func TestCriarSeAusenteNaoMexeEmArquivoExistente(t *testing.T) {
	isolar(t)

	caminho, _ := Caminho()
	_ = os.MkdirAll(filepath.Dir(caminho), permPasta)

	// Mesmo corrompido: e' a edicao do operador, com uma virgula errada.
	original := []byte(`{"url_base": "https://homologacao.exemplo",}`)
	_ = os.WriteFile(caminho, original, permArq)

	if err := CriarSeAusente(); err != nil {
		t.Fatal(err)
	}

	bruto, _ := os.ReadFile(caminho)
	if string(bruto) != string(original) {
		t.Errorf("o arquivo existente foi sobrescrito:\n%s", bruto)
	}
}
