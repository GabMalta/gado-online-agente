package fonte

import (
	"os"
	"path/filepath"
	"testing"
)

func escrever(t *testing.T, conteudo string) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "estado.json")
	if err := os.WriteFile(caminho, []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	return caminho
}

func TestSimuladoLeLoteEValor(t *testing.T) {
	f := NovoSimulado(escrever(t, `{"lote":"001","valor":"1850"}`))

	estado, err := f.Ler()
	if err != nil {
		t.Fatal(err)
	}
	if estado.Lote != "001" || estado.Valor != "1850" {
		t.Errorf("estado inesperado: %+v", estado)
	}
	if estado.Vazia() {
		t.Error("nao devia estar vazia")
	}
}

func TestSimuladoDescartaOverrideVazio(t *testing.T) {
	f := NovoSimulado(escrever(t, `{"lote":"001","obs":"   ","raca":"Nelore"}`))

	estado, _ := f.Ler()

	if _, existe := estado.Overrides["obs"]; existe {
		t.Errorf("obs em branco nao devia virar override: %v", estado.Overrides)
	}
	if estado.Overrides["raca"] != "Nelore" {
		t.Errorf("override preenchido foi perdido: %v", estado.Overrides)
	}
}

func TestSimuladoTrimaOsCampos(t *testing.T) {
	// O operador digita no vMix: espaco sobrando e' a regra.
	f := NovoSimulado(escrever(t, `{"lote":"  001  ","valor":" 1850 "}`))

	estado, _ := f.Ler()

	if estado.Lote != "001" || estado.Valor != "1850" {
		t.Errorf("nao trimou: %+v", estado)
	}
}

func TestSimuladoArquivoAusenteEhOverlayVazio(t *testing.T) {
	// Permite testar o "liberar pista" apagando o arquivo.
	f := NovoSimulado(filepath.Join(t.TempDir(), "nao-existe.json"))

	estado, err := f.Ler()
	if err != nil {
		t.Fatalf("ausencia de arquivo nao devia ser erro: %v", err)
	}
	if !estado.Vazia() {
		t.Error("devia estar vazia")
	}
}

func TestSimuladoArquivoVazioEhOverlayVazio(t *testing.T) {
	f := NovoSimulado(escrever(t, "   \n  "))

	estado, err := f.Ler()
	if err != nil {
		t.Fatalf("arquivo vazio nao devia ser erro: %v", err)
	}
	if !estado.Vazia() {
		t.Error("devia estar vazia")
	}
}

func TestSimuladoJSONInvalidoDevolveErro(t *testing.T) {
	f := NovoSimulado(escrever(t, "{isso nao e json"))

	if _, err := f.Ler(); err == nil {
		t.Fatal("esperava erro de parse")
	}
}

func TestEstadoVaziaComLoteEmBranco(t *testing.T) {
	if !(Estado{Lote: "   "}).Vazia() {
		t.Error("lote em branco devia contar como vazia")
	}
}
