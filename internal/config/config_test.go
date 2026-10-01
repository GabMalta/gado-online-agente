package config

import (
	"errors"
	"os"
	"path/filepath"
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

	if err := Salvar(Config{Chave: "  abc123  ", URLBase: "http://localhost:8899/"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Carregar()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Chave != "abc123" {
		t.Errorf("chave nao foi trimada: %q", cfg.Chave)
	}
	// Barra no fim viraria //api/... nas requisicoes.
	if cfg.URLBase != "http://localhost:8899" {
		t.Errorf("barra final nao foi removida: %q", cfg.URLBase)
	}
	if !cfg.TemChave() {
		t.Error("TemChave devia ser true")
	}
}

func TestURLBaseVaziaCaiNoPadrao(t *testing.T) {
	isolar(t)

	if err := Salvar(Config{Chave: "abc"}); err != nil {
		t.Fatal(err)
	}

	cfg, _ := Carregar()
	if cfg.URLBase != URLPadrao {
		t.Errorf("esperava %q, veio %q", URLPadrao, cfg.URLBase)
	}
}

func TestCamposPadraoSaoPreenchidos(t *testing.T) {
	isolar(t)

	_ = Salvar(Config{Chave: "abc"})
	cfg, _ := Carregar()

	if cfg.VMixCampos["lote"] == "" {
		t.Error("campos padrao do vMix nao foram preenchidos")
	}
	if cfg.VMixTitle == "" {
		t.Error("title padrao do vMix nao foi preenchido")
	}
}

func TestEsquecerChaveMantemORestoDaConfig(t *testing.T) {
	isolar(t)

	_ = Salvar(Config{Chave: "abc", URLBase: "http://x", VMixTitle: "meu-title"})

	if err := EsquecerChave(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Carregar()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TemChave() {
		t.Error("a chave devia ter sido apagada")
	}
	if cfg.VMixTitle != "meu-title" {
		t.Errorf("o resto da config foi perdido: %+v", cfg)
	}
}

func TestEsquecerChaveSemConfigNaoFalha(t *testing.T) {
	isolar(t)

	if err := EsquecerChave(); err != nil {
		t.Fatalf("devia ser no-op, veio %v", err)
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

	// Tratado como primeira execucao: o agente pede a chave de novo em vez de
	// morrer as 19h de um sabado.
	if _, err := Carregar(); !errors.Is(err, ErrSemConfig) {
		t.Fatalf("esperava ErrSemConfig, veio %v", err)
	}
}

func TestSalvarNaoDeixaArquivoTemporario(t *testing.T) {
	isolar(t)

	if err := Salvar(Config{Chave: "abc"}); err != nil {
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
	if cfg.TemChave() {
		t.Error("Padrao nao devia ter chave")
	}
}
