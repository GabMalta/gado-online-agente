VERSAO ?= 0.1.0
PACOTE  = ./cmd/agente

# -s -w tira tabela de simbolos e DWARF: binario menor, e nada que o operador
# precise. A versao entra no painel de status.
LDFLAGS = -s -w -X main.versao=$(VERSAO)

.PHONY: tudo teste vet fmt windows recursos linux limpar

tudo: vet teste windows linux

teste:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w ./cmd ./internal

# O alvo real: cross-compila o .exe do Linux/WSL, sem maquina Windows.
windows: recursos
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" \
		-o dist/agente-gadoonline.exe $(PACOTE)

# Icone e "Detalhes" (botao direito -> Propriedades) do .exe. O go-winres gera
# um .syso em cmd/agente que o `go build` embute sozinho, e so no build do
# Windows (pelo sufixo _windows_amd64). Roda via `go run @versao` de proposito:
# e' ferramenta de build, nao entra no go.mod nem no binario.
WINRES = github.com/tc-hib/go-winres@v0.3.3

recursos:
	cd $(PACOTE) && go run $(WINRES) simply --arch amd64 --manifest cli \
		--icon ../../assets/icone.png \
		--product-name "Gado Online - Agente de Transmissão" \
		--file-description "Gado Online - Agente de Transmissão" \
		--original-filename agente-gadoonline.exe \
		--product-version $(VERSAO) --file-version $(VERSAO)

linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" \
		-o dist/agente-gadoonline $(PACOTE)

limpar:
	rm -rf dist/
