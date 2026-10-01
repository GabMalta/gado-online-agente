# Agente de transmissão — Gado Online

Programa que roda na máquina da transmissão e informa ao site **qual lote está em
pista**, lendo direto do vMix ou do OBS.

Enquanto ele estiver aberto, a página pública do leilão acompanha o que está no ar.

---

## Instalar

1. Baixe o `agente-gadoonline.exe` na [página de releases][releases].
2. Salve numa pasta fixa (a Área de Trabalho serve).
3. Dê dois cliques.

Não precisa instalar nada mais, não precisa de senha de administrador.

> **O Windows vai mostrar "O Windows protegeu o seu computador".** O programa ainda
> não tem assinatura digital. Clique em **Mais informações** → **Executar assim
> mesmo**.

[releases]: https://github.com/GabMalta/gado-online-agente/releases

---

## Primeira vez

O agente pede a chave de transmissão:

```
  Nenhuma chave configurada.
  Pegue a chave no dashboard: card do leilão → botão "Transmissão".

  Cole a chave de transmissão e pressione Enter:
  >
```

**Onde pegar a chave:** no sistema, na lista de leilões, no card do leilão de hoje,
botão **Transmissão**. Copie a chave e cole aqui (Ctrl+V, ou clique com o botão
direito no console).

Ele confirma qual leilão é:

```
  Chave salva aponta para:
    LEILÃO REAL PRESENCIAL E VIRTUAL — 01/10/2026
    90 lotes no catálogo

  [Enter] usar este leilão      [N] colar outra chave
```

Confere o nome e a data, e aperte Enter.

## Nos leilões seguintes

A chave fica salva, então ele já abre mostrando qual leilão ela aponta. **Cada leilão
tem a sua própria chave**, então no próximo você aperta `N` e cola a nova.

Se a chave salva for de um leilão antigo, ele avisa — e aí o Enter já pede a chave
nova:

```
  ⚠  Esse leilão é de 04/08/2026 (há 58 dias).
     Você provavelmente precisa da chave do leilão de hoje.

  [Enter] colar nova chave      [U] usar este leilão mesmo assim
```

## Durante o leilão

Deixe a janela aberta num canto da tela:

```
  Gado Online — agente de transmissão          v1.0.0
  ────────────────────────────────────────────────────────
  vMix        ● conectado  (127.0.0.1:8088 · pista)
  Servidor    ● conectado  (último envio há 2s)
  Leilão      LEILÃO REAL PRESENCIAL E VIRTUAL

  ▶   lote 001   1850   ✎ obs
```

- **vMix / OBS conectado** — está lendo o overlay.
- **Servidor conectado** — o site está recebendo.
- **✎** — lista os campos que você digitou por cima do catálogo.

Para encerrar: `Ctrl+C`. Ele libera a pista antes de sair, e o site volta a mostrar
"Aguardando próximo lote".

---

## Configurar o vMix

### 1. Catálogo automático (acaba com a planilha)

Em vez de montar a planilha de Excel a cada leilão, o vMix busca o catálogo direto
do sistema.

1. **Settings → Data Sources → Add → XML**
2. Cole a URL que aparece na tela **Transmissão** do sistema (ela já vem com a chave).
3. Intervalo de atualização: **10 a 30 segundos** — baixo o bastante para um lote
   cadastrado durante o leilão aparecer.
4. No seu Title, ligue cada campo de texto à coluna correspondente:
   `numero`, `animais`, `raca`, `idade`, `peso`, `obs`, `vendedor`, `fazenda`,
   `status`, `valor`.

Os valores já vêm prontos para exibir: `20 machos`, `380 kg`, `R$ 4.250,00`.

### 2. Nomes dos campos do Title

O agente procura o Title chamado **pista** e, dentro dele, os campos:

| Campo | Nome esperado no Title |
|---|---|
| número do lote | `Lote.Text` |
| lance | `Valor.Text` |
| observação | `Obs.Text` |

Se os seus campos têm outros nomes, não precisa renomear nada no vMix — veja
[Mudar a configuração](#mudar-a-configuração).

> **A Web Controller do vMix precisa estar ligada** para o agente ler o overlay:
> **Settings → Web Controller → Enable**.

### Você continua podendo digitar por cima

O agente lê **o que está na tela**, não o que a Data Source mandou. Então:

- corrigir uma observação na hora → o site mostra a sua correção;
- anunciar um lote que ainda não está cadastrado → o site mostra o que você digitou,
  marcado como "fora do catálogo". Quando o lote for cadastrado, ele se liga sozinho.

---

## Configurar o OBS

1. **Ferramentas → Configurações do Servidor WebSocket** → marque **Ativar**.
2. Anote a porta (padrão `4455`) e a senha.
3. Abra o agente com `--obs`.

Ele lê os sources de texto chamados `Lote`, `Valor` e `Obs`.

---

## Mudar a configuração

O arquivo fica em `%APPDATA%\GadoOnline\agente.json`. Cole isso na barra de endereço
do Explorer para chegar lá:

```
%APPDATA%\GadoOnline
```

```json
{
  "chave": "...",
  "url_base": "https://backend.gadoonline.com.br",
  "vmix_title": "pista",
  "vmix_campos": {
    "lote": "Lote.Text",
    "valor": "Valor.Text",
    "obs": "Obs.Text"
  },
  "obs_endereco": "localhost:4455",
  "obs_senha": "",
  "obs_campos": { "lote": "Lote", "valor": "Valor", "obs": "Obs" }
}
```

Em `vmix_campos`, o lado esquerdo é fixo e o direito é o nome no seu Title. Também
aceita o número da camada (`"lote": "0"`) quando as camadas não têm nome.

> A chave **não** fica junto do `.exe` de propósito: assim você pode copiar o
> programa para outra máquina sem levar a credencial junto.

---

## Quando o site não atualiza

| O que você vê | O que é | O que fazer |
|---|---|---|
| `vMix ● sem contato` | Web Controller desligada, ou vMix fechado | Settings → Web Controller → Enable |
| `Servidor ● sem contato` | internet caiu | Ele tenta sozinho; a transmissão continua normal |
| `pista livre` com lote no ar | o agente não achou o campo do lote | Confira o nome do Title e dos campos |
| `A chave foi recusada` | chave revogada, ou de outro leilão | Pegue a chave atual no sistema |
| Site mostra outro lote | chave de outro leilão | Reabra o agente e confira o nome do leilão |

Se o agente fechar sem você mandar, **a pista se libera sozinha em até 2 minutos** —
o site não fica preso num lote antigo.

---

## Linha de comando

```
--simular arquivo.json    lê de um arquivo em vez do vMix/OBS (testes e demonstração)
--obs                     lê do OBS em vez do vMix
--servidor URL            outro servidor (homologação)
--intervalo 3s            intervalo entre leituras
```

---

## Testar sem vMix e sem OBS

`--simular` troca a leitura do overlay por um arquivo JSON, **relido a cada
ciclo**. Você edita no Notepad, salva, e a página pública acompanha em segundos.
Serve para testar, demonstrar e treinar operador sem montar transmissão.

**1. Crie o arquivo** (`C:\pista.json`):

```json
{ "lote": "001", "valor": "1850" }
```

**2. Rode apontando para ele:**

```
agente-gadoonline.exe --simular C:\pista.json --intervalo 1s
```

O `--intervalo 1s` deixa o retorno mais rápido enquanto você brinca. Em leilão de
verdade o padrão de 3s é melhor.

**3. Abra a página pública do leilão** e edite o arquivo para ver cada caso:

| Salve isto | O que acontece na página |
|---|---|
| `{"lote":"001","valor":"1850"}` | lote 001 em pista, lance R$ 1.850,00, resto do catálogo |
| `{"lote":"001","valor":"2000"}` | só o lance muda |
| `{"lote":"001","valor":"2000","obs":"Reagrupado"}` | observação do overlay, com selo "informado na transmissão" |
| `{"lote":"001","valor":"2000"}` | a observação **sai** — o payload é a tela inteira, não um acréscimo |
| `{"lote":"001","qtd_animais":"25","sexo":"F","peso":"410"}` | sobrescreve quantidade, sexo e peso **de um lote do catálogo** |
| `{"lote":"9999","valor":"3200","qtd_animais":"7","sexo":"F","raca":"Girolando"}` | lote fora do catálogo, montado só com o que você digitou |
| `{}` (ou apague o arquivo) | pista liberada, volta para "Aguardando próximo lote" |

Campos aceitos: `lote` (obrigatório), `valor`, `qtd_animais`, `raca`, `sexo`,
`idade`, `peso`, `obs`. **Todos funcionam em qualquer lote** — tanto para corrigir
um do catálogo quanto para montar um que não existe.

Deixar um campo de fora ou em branco é a mesma coisa: aquele dado volta a vir do
catálogo. É assim que o operador desfaz algo digitado por engano.

### Você pode escrever como aparece na tela

O agente lê o texto **renderizado** no overlay, então ele aceita o que o operador
realmente digita, não códigos:

| Você escreve | O sistema entende |
|---|---|
| `"sexo": "MACHOS"` · `"Fêmeas"` · `"m"` | `M` / `F` |
| `"qtd_animais": "25 cabeças"` · `"25"` · `25` | 25 |
| `"peso": "410 kg"` · `"410 KGS"` · `"410"` | 410 (a página escreve o "kg") |

**Um campo que o sistema não entende é descartado sozinho, e o resto do envio
continua valendo** — aquele dado volta a vir do catálogo. Um `"sexo": "indefinido"`
não derruba o lance nem o número do lote.

### Apontando para o seu Django local

O servidor padrão é a produção. Para testar contra a sua máquina, rode o Django
escutando em todas as interfaces (o Windows precisa alcançar o WSL):

```bash
python manage.py runserver 0.0.0.0:8899
```

E no Windows, com o IP do WSL (`hostname -I` no terminal do Ubuntu):

```
agente-gadoonline.exe --simular C:\pista.json --servidor http://172.x.x.x:8899
```

A chave tem que ser gerada **nesse** ambiente — chave da produção não vale no
banco local, e vice-versa.

> ⚠️ Se o agente disser **"O servidor respondeu, mas não conhece as rotas de
> transmissão"**, o backend daquele endereço está numa versão anterior às rotas
> `/api/transmissao/*`. É o caso da produção enquanto o backend não for mergeado.

---

## Para quem desenvolve

```bash
make teste      # go test ./...
make vet
make windows    # cross-compila o .exe (do Linux/WSL, sem máquina Windows)
make linux
```

Protocolo e decisões de arquitetura:
`Frontend/docs/integracao/agente-transmissao.md` no repositório principal.

Dependências: só `gorilla/websocket`, usada pelo adaptador do OBS. Todo o resto é
biblioteca padrão — em binário que vai para a máquina do cliente, cada dependência
é superfície. Inclusive as cores do console: o Virtual Terminal Processing do
Windows é ligado via `kernel32` por `syscall`, e quando não dá, o painel sai sem
enfeite em vez de imprimir os códigos crus.

### Verificado e não verificado

Testado ponta a ponta contra o backend real com a fonte `--simular`: lote do
catálogo, override de campo, override saindo do overlay, lote fora do catálogo,
liberar pista, chave revogada no meio do leilão (apaga do disco), queda e volta do
servidor, e o fluxo de abertura nos três caminhos.

**Nunca testado contra vMix ou OBS de verdade.** O parse do XML do vMix segue a forma
geral da API e o adaptador do OBS segue o protocolo oficial do obs-websocket v5, com
o desafio de autenticação travado em teste — mas nenhum dos dois passou por uma
máquina real. Antes do primeiro leilão, rode `curl http://127.0.0.1:8088/api/` com o
Title no ar e confira os nomes dos campos.
