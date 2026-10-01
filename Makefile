VERSAO ?= 0.1.0
PACOTE  = ./cmd/agente

# -s -w tira tabela de simbolos e DWARF: binario menor, e nada que o operador
# precise. A versao entra no painel de status.
LDFLAGS = -s -w -X main.versao=$(VERSAO)

.PHONY: tudo teste vet fmt windows linux limpar

tudo: vet teste windows linux

teste:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w ./cmd ./internal

# O alvo real: cross-compila o .exe do Linux/WSL, sem maquina Windows.
windows:
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" \
		-o dist/agente-gadoonline.exe $(PACOTE)

linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" \
		-o dist/agente-gadoonline $(PACOTE)

limpar:
	rm -rf dist/
