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

## Abrir o agente

Dois cliques no `.exe`. Ele pede o **mesmo usuário e senha do sistema**:

```
  Gado Online — agente de transmissão
  Servidor:      https://backend.gadoonline.com.br
  Configuração:  C:\Users\voce\AppData\Roaming\GadoOnline\agente.json

  Entre com o seu usuário do sistema.
  Usuário: joao
  Senha: ******
```

Depois, tudo se escolhe com as **setas ↑ ↓** e **Enter**:

```
  Qual programa de transmissão?
  ▶ vMix
    OBS

  Qual leilão?
  ▶ LEILÃO REAL PRESENCIAL E VIRTUAL — 06/10/2026 — Em andamento — 90 lotes
    LEILÃO DE PRIMAVERA — 20/10/2026 — Aguardando — 45 lotes
```

Só aparecem os leilões **aguardando** ou **em andamento** da sua empresa. Se não
aparecer nenhum, cadastre o leilão (ou ajuste o status) no sistema e escolha
**Atualizar a lista**.

Da próxima vez, o usuário já vem preenchido e o menu abre no programa da última vez.
A senha **não** fica salva.

> **Não precisa mais colar chave.** O agente usa a chave de transmissão que já
> existe para o leilão, e só gera uma se ainda não houver. Ele nunca gera uma
> nova por cima: isso derrubaria a URL do overlay que já está no vMix/OBS.

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

### 1. Deixar o agente ler o OBS

1. **Ferramentas → Configurações do Servidor WebSocket** → marque **Ativar**.
2. Anote a porta (padrão `4455`) e a senha.
3. Abra o agente e escolha **OBS** no menu.

Ele lê os sources de **texto** chamados `Lote`, `Valor` e `Obs`. Se a sua senha não
for vazia ou a porta for outra, ajuste em [Mudar a configuração](#mudar-a-configuração).

### 2. Catálogo automático (acaba com a planilha)

O OBS não tem Data Source como o vMix. No lugar dela, um **Browser Source** mostra
o lote em pista já montado com os dados do catálogo — você digita só o número.

1. **Fontes → + → Navegador**.
2. URL: a que aparece na tela **Transmissão** do sistema, em *"Overlay para o
   Browser Source do OBS"* (já vem com a chave).
3. Largura e altura **iguais ao seu canvas** (normalmente 1920 × 1080), e deixe a
   fonte posicionada em 0,0 — o card se ancora sozinho no canto inferior esquerdo.
4. Deixe **"Desligar a fonte quando não estiver visível" desmarcado**: com ela
   marcada o OBS recarrega a página a cada troca de cena.

O card aparece quando você digita o número no source de texto `Lote`, e some
quando a pista é liberada. **Entre um lote e outro a página não desenha nada** —
nenhuma tarja fica no ar.

Para ajustar o tamanho, acrescente `?escala=1.25` ao fim da URL (de `0.5` a `3`).
Redimensionar a fonte pelo OBS borra o texto; a escala na URL desenha no tamanho
certo.

> **Não é o mesmo endereço do catálogo do vMix.** Aquele (`overlay.xml`) é XML cru
> e num Browser Source aparece como um amontoado de texto. No OBS use a URL do
> overlay.

### Por que você continua digitando o número

O source de texto `Lote` é a **entrada** (o agente lê o que você digita) e o
Browser Source é a **saída** (o card completo que volta do catálogo). São duas
coisas diferentes na cena: o `Lote` pode ficar fora do enquadramento, ou numa cena
que não vai ao ar.

Como no vMix, você continua podendo digitar por cima: `Valor` e `Obs` entram no
card, e um número que não está no catálogo aparece marcado como "fora do catálogo".

---

## Mudar a configuração

O caminho do arquivo aparece na tela de abertura do agente (`Configuração:`). Ele
fica em `%APPDATA%\GadoOnline\agente.json`; cole isso na barra de endereço do
Explorer para chegar lá:

```
%APPDATA%\GadoOnline
```

```json
{
  "url_base": "https://backend.gadoonline.com.br",
  "usuario": "joao",
  "software": "vmix",
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

`url_base` é o servidor para onde o agente manda tudo (login, leilões e pista).
Troque o domínio aqui para apontar para outro ambiente; salve e abra o agente de
novo — o endereço novo aparece em `Servidor:` na abertura.

Em `vmix_campos`, o lado esquerdo é fixo e o direito é o nome no seu Title. Também
aceita o número da camada (`"lote": "0"`) quando as camadas não têm nome.

> Nenhuma senha ou chave fica salva aqui: o agente pede o login a cada abertura e
> busca a chave no servidor. O arquivo também **não** fica junto do `.exe`, então dá
> para copiar o programa para outra máquina sem levar nada junto.

---

## Quando o site não atualiza

| O que você vê | O que é | O que fazer |
|---|---|---|
| `vMix ● sem contato` | Web Controller desligada, ou vMix fechado | Settings → Web Controller → Enable |
| `Servidor ● sem contato` | internet caiu | Ele tenta sozinho; a transmissão continua normal |
| `pista livre` com lote no ar | o agente não achou o campo do lote | Confira o nome do Title e dos campos |
| `Usuário ou senha inválidos` | login errado | O mesmo usuário e senha do sistema |
| `Nenhum leilão aguardando ou em andamento` | o leilão está finalizado/fechado, ou ainda não foi cadastrado | Ajuste no sistema e escolha **Atualizar a lista** |
| `A chave foi recusada` | alguém gerou chave nova ou revogou a chave no sistema | Feche e abra o agente de novo |
| Browser Source dizendo "Overlay sem leilão" | a chave da URL foi revogada ou é de outro leilão | Pegue a URL atual na tela Transmissão |
| Browser Source com tarja preta e texto cru | é a URL do catálogo do vMix (`overlay.xml`) | Use a URL do overlay, não a do XML |
| Site mostra outro lote | escolheu o leilão errado no menu | Reabra o agente e escolha o leilão certo |

Se o agente fechar sem você mandar, **a pista se libera sozinha em até 2 minutos** —
o site não fica preso num lote antigo.

---

## Linha de comando

```
--simular arquivo.json    lê de um arquivo em vez do vMix/OBS (testes e demonstração)
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
