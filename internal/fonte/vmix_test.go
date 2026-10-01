package fonte

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GabMalta/gado-online-agente/internal/config"
)

// xmlVMix é a forma geral da resposta de /api/ do vMix.
const xmlVMix = `<vmix>
  <version>27.0.0.70</version>
  <inputs>
    <input key="aaa" number="1" type="Capture" title="Camera 1"/>
    <input key="bbb" number="3" type="GT" title="pista.gtzip">
      <text index="0" name="Lote.Text">001</text>
      <text index="1" name="Valor.Text">1850</text>
      <text index="2" name="Obs.Text"></text>
      <text index="3" name="Raca.Text">Nelore</text>
    </input>
  </inputs>
  <active>3</active>
</vmix>`

func servirVMix(t *testing.T, corpo string) *VMix {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(corpo))
	}))
	t.Cleanup(srv.Close)

	v := NovoVMix("pista", config.CamposPadraoVMix())
	v.endereco = srv.URL
	return v
}

func TestVMixLeOsCamposDoTitle(t *testing.T) {
	estado, err := servirVMix(t, xmlVMix).Ler()
	if err != nil {
		t.Fatal(err)
	}

	if estado.Lote != "001" || estado.Valor != "1850" {
		t.Errorf("estado inesperado: %+v", estado)
	}
	if estado.Overrides["raca"] != "Nelore" {
		t.Errorf("override nao foi lido: %v", estado.Overrides)
	}
	// Campo em branco no overlay nao vira override.
	if _, tem := estado.Overrides["obs"]; tem {
		t.Errorf("obs vazia virou override: %v", estado.Overrides)
	}
}

func TestVMixAchaOTitlePorPrefixo(t *testing.T) {
	// O title do vMix vem com a extensao do arquivo; exigir o nome exato faria
	// o operador errar na config.
	v := servirVMix(t, xmlVMix)
	v.title = "pista"

	estado, err := v.Ler()
	if err != nil {
		t.Fatal(err)
	}
	if estado.Lote != "001" {
		t.Errorf("nao achou o Title por prefixo: %+v", estado)
	}
}

func TestVMixAchaOTitlePorKey(t *testing.T) {
	v := servirVMix(t, xmlVMix)
	v.title = "bbb"

	estado, err := v.Ler()
	if err != nil {
		t.Fatal(err)
	}
	if estado.Lote != "001" {
		t.Errorf("nao achou o Title por key: %+v", estado)
	}
}

func TestVMixTitleAusenteEhOverlayVazioENaoErro(t *testing.T) {
	// O operador pode nao ter carregado o Title ainda: pista livre, nao falha.
	v := servirVMix(t, xmlVMix)
	v.title = "nao-existe"

	estado, err := v.Ler()
	if err != nil {
		t.Fatalf("Title ausente nao devia ser erro: %v", err)
	}
	if !estado.Vazia() {
		t.Error("devia estar vazia")
	}
}

func TestVMixLeTambemPorIndiceQuandoNaoHaNome(t *testing.T) {
	// Camadas nao nomeadas no Title: o index e' a unica referencia.
	semNome := `<vmix><inputs><input key="bbb" type="GT" title="pista">
	  <text index="0">007</text><text index="1">2500</text>
	</input></inputs></vmix>`

	v := servirVMix(t, semNome)
	v.campos = map[string]string{"lote": "0", "valor": "1"}

	estado, err := v.Ler()
	if err != nil {
		t.Fatal(err)
	}
	if estado.Lote != "007" || estado.Valor != "2500" {
		t.Errorf("nao leu por indice: %+v", estado)
	}
}

func TestVMixForaDoArDevolveErro(t *testing.T) {
	v := NovoVMix("pista", config.CamposPadraoVMix())
	v.endereco = "http://127.0.0.1:1" // porta fechada

	if _, err := v.Ler(); err == nil {
		t.Fatal("esperava erro de conexao")
	}
}

func TestVMixXMLInvalidoDevolveErro(t *testing.T) {
	if _, err := servirVMix(t, "<vmix><inputs>").Ler(); err == nil {
		t.Fatal("esperava erro de parse")
	}
}

func TestVMixTrimaOTextoDoOverlay(t *testing.T) {
	comEspaco := `<vmix><inputs><input key="b" type="GT" title="pista">
	  <text index="0" name="Lote.Text">  012  </text>
	</input></inputs></vmix>`

	estado, err := servirVMix(t, comEspaco).Ler()
	if err != nil {
		t.Fatal(err)
	}
	if estado.Lote != "012" {
		t.Errorf("nao trimou: %q", estado.Lote)
	}
}
