---
inclusion: fileMatch
fileMatchPattern: "**/*.go,**/go.mod,**/Makefile"
---

# Psicoman — Convenções Go do projeto

Complementa o steering global de Go (idiomas, estilo, concorrência). Aqui ficam as decisões **específicas do Psicoman**, definidas em `docs/architecture.md`. Sempre consultar esse documento antes de desviar de um padrão aqui descrito.

## Estrutura e camadas

- Layout: **binário único** `cmd/psicoman` (importa o PocketBase como framework via `pocketbase.New()` + hook `app.OnServe()` para registrar as rotas próprias). Os pacotes internos são `internal/{domain/<bounded contexts>, app, platform/{crypto,pb}, integration/{google,gemini}, drivesync, api}` (ver `docs/architecture.md`).
  - `internal/domain/{patient,clinical,billing,scheduling,costs,therapist}` + o tipo `SealedText`.
  - `internal/app`: serviços de negócio enxutos, que apenas orquestram.
  - `internal/platform/{crypto,pb}`: `crypto` abriga o AES-256-GCM + `KeyProvider` + o envelope do `SealedText`; `pb` é o adapter que mapeia domínio↔Record do PocketBase.
  - `internal/integration/{google,gemini}`, `internal/drivesync`, `internal/api` (rotas `/v1/*` próprias).
- Regra de dependência **unidirecional**: o **domínio é a autoridade e não importa o PocketBase** (nem nada do projeto além de `platform/crypto` para o `SealedText`). `platform/pb` é o **único** pacote que conhece o PocketBase; `app` orquestra sobre o domínio e os ports; `api` expõe `/v1` consumindo `app`. Nunca importar de uma camada "de fora" para "dentro" (ex: `domain` importando `app` ou `platform/pb`). Trocar o PocketBase por outra base = reescrever só o adapter `platform/pb`.
- **Domínio rico**: os agregados em `internal/domain` carregam comportamento, serialização/desserialização, formatação de log e as ações do próprio domínio (OO bem feita) — não são sacos de dados anêmicos. A orquestração (serviços de negócio) fica enxuta em `internal/app`, apenas coordenando os agregados.
- **Binário único atrás do `Pangolin`**: `cmd/psicoman` roda sempre atrás do `Pangolin` e nunca é exposto direto à internet. A segregação admin/portal é **lógica** (grupos de rota `/v1/admin/*` e `/v1/portal/*` + regras de auth por grupo), não física — não há mais dois processos. Código exclusivo de um namespace (ex: middleware de auth do `Pangolin`) fica no grupo de rotas correspondente dentro de `internal/api`, nunca em pacotes compartilhados do domínio.

## Padrões de domínio (não negociáveis)

- **IDs**: ULID em toda entidade nova (`internal/platform/ulid`). Nunca `int` autoincrement nem `uuid.New()` puro.
- **Dinheiro**: sempre `int64` em centavos, moeda BRL implícita. Nunca `float64` para valores monetários — em cálculo, divisão ou arredondamento envolvendo dinheiro, usar inteiro e documentar a regra de arredondamento (ex: rateio de custo, ver `docs/architecture.md`).
- **Tempo**: `time.Time` sempre em `America/Sao_Paulo` (não usar `time.Now()` puro sem carregar a location; usar um helper central, ex: `platform/clock`). Persistir como texto ISO-8601 no SQLite.
- **Idempotência**: qualquer operação que gera efeito financeiro (débito) precisa de `idempotency_key` e usar `INSERT ... ON CONFLICT DO NOTHING` (ou equivalente) — nunca "checar antes de inserir" com race condition entre check e insert.
- **Erros para o usuário**: toda mensagem de erro retornada pela API é em **PT-BR**, mesmo que o erro interno/log seja em inglês. Não deixar `err.Error()` (que costuma vir da lib em inglês) vazar direto pro body da resposta.
- **Cripto por campo**: campos clínicos sensíveis (corpo da sessão, anamnese, notas, resumo de IA) usam o tipo de domínio `SealedText` — paralelo à regra do dinheiro: assim como valor nunca é `float`, texto clínico nunca é `string` em claro ao persistir. `SealedText` carrega o claro **em memória** durante a vida do agregado e **não** tem `MarshalJSON` que exponha o claro; a cifra/decifra acontece na **borda de persistência** (adapter `internal/platform/pb`), via a interface `domain.Cipher` (implementada em `internal/platform/crypto`, AES-256-GCM): grava o envelope `versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext` (base64) e lê decifrando para `SealedText`. O claro só sai via `Reveal(ctx)`, e a saída para a API passa **sempre por DTO explícito** — nenhuma resposta serializa `SealedText` direto; só o ator terapeuta (`/v1/app/*`) revela o claro. A chave vem da config atrás do `KeyProvider` (o `id_da_chave` no envelope habilita rotação). O PocketBase **não** é a fronteira de cifra — a cifra é do nosso `SealedText`. A cifra é por campo: nome, email, telefone, datas, status e valores ficam em claro e indexáveis; a busca clínica se dá pelas **tags** da IA (em claro), não pelo corpo.

## Contexto e cancelamento

- Toda função de `app`/`platform/pb`/`integration` que faz I/O recebe `context.Context` como primeiro parâmetro.
- Chamadas à API do Google (Calendar/Gmail/Drive) sempre com timeout via `context.WithTimeout` — nunca sem timeout, para não travar o handler HTTP esperando uma API externa.

## Testes

- Regras de negócio sensíveis (idempotência de débito, rateio de custo, gatilho de plano fechado, checagem de conflito de agenda) sempre com teste unitário cobrindo o caso feliz **e** o caso de borda (dupla chamada, período vazio, conflito exato no limite do horário).
- Ver `psicoman-testes-e2e.md` para a convenção de E2E.

## Middleware / contrato de API

- O contrato público são as **rotas próprias `/v1/*`** registradas sobre o router do PocketBase (via `app.OnServe()`). A **REST automática do PocketBase não é contrato público** — não expor dado clínico por ela; o contrato versionado é o nosso `/v1`.
- Toda rota HTTP passa pela chain `recover → request-id → timing → logging → auth → handler` (ver `docs/architecture.md`). Novas rotas não pulam etapas dessa chain.
- Toda resposta usa o envelope padrão `{ message, elapsed_ms, data, error }` (`message` em PT-BR). Não introduzir um formato de resposta alternativo sem atualizar `docs/architecture.md` primeiro.
