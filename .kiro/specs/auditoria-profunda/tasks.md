# auditoria-profunda — Plano de Implementação (Tasks)

Corrige os achados da auditoria profunda de 2026-11-05 sobre o Psicoman: auditoria LGPD no prontuário, snapshot inteiro do Drive com retenção, criação automática da pasta de DR, golangci-lint reativado, rate limit e `state` do OAuth, ator real no `audit_log`, métricas Prometheus ricas e coerência de docs e steering. Refs: [`requirements.md`](./requirements.md) (R1–R10), [`design.md`](./design.md) (§1–§13). Formato herdado de [`../pocketbase-finish/tasks.md`](../pocketbase-finish/tasks.md).

Convenções: `[ ]` pendente · `[~]` em progresso · `[x]` concluído. Cada task fecha com **build verde** (`make check`), **docs no mesmo ciclo** e **testes cobrindo** (unit/E2E; segurança → permitido+negado; cripto → round-trip+não-vazamento). Mensagens ao usuário em PT-BR; centavos; `America/Sao_Paulo`. Regra de dependência inalterada: `domain`/`app`/`config` não importam o PocketBase **nem** a lib de métricas. Arquivos marcados **(novo)** não existem hoje; os demais foram conferidos no repositório.

**Git:** branch da onda `auditoria-profunda`, criada a partir da `main` na task 0.1 (nunca trabalhar direto na `main`). Commit em PT-BR, no imperativo, ao fim de cada fase, com `make check` verde. **Sem push antes do fim da onda inteira e só com OK do Rafael** (task 10.4).

**Escopo:** esta spec é só o plano. Nada aqui foi executado; a edição de steering, `docs/` e código acontece nas tasks abaixo, em outra sessão.

---

## Ordem de execução e por quê

O `requirements.md` agrupa em sub-ondas A/B/C **por prioridade**. A execução abaixo segue a **dependência técnica** do `design.md` (§11.1), que diverge dessa prioridade em quatro pontos:

1. **R4 (lint) vem primeiro** (Fases 1 e 2), embora seja prioridade média: o código novo de todas as fases seguintes já nasce sob o gate real. A Fase 2 isola a triagem dos achados, em commits separados do trabalho funcional.
2. **R7.4 (ator no contexto) antecede R1** (Fase 3): R1 grava `audit_log` com o ator vindo do `context`, e R5 testa as duas coisas juntas. Por isso o R7.4, de prioridade baixa, entra antes do R1, que é alta.
3. **R3 antecede as métricas de backup de R8.6** e **R2 (retenção) antecede a métrica de snapshots mantidos**: Fases 4 e 5 vêm antes da Fase 8. O `drivesync.Observer` nasce na Fase 5 com `NopObserver`, e a Fase 8 só liga a implementação Prometheus.
4. **R1 antecede as métricas de auditoria de R8.7** (Fase 3 antes da Fase 8). **R8 base** (Fase 7) antecede as **famílias** (Fase 8), e o limitador de R7.1 (Fase 6) expõe um `OnReject` opcional que a Fase 8 liga ao recorder.

Tasks de **spike** (suposições do `design.md` §11.5) vêm antes de qualquer task que dependa delas: lint (1.1), `drive.file` (4.1), `Request.Pattern`/`countingWriter`/`go mod tidy` (7.1 a 7.3). Se um spike falhar, a fase para e o Rafael decide. Nunca se reintroduz skip silencioso do linter.

---

## 📊 Status ao vivo da onda

> `✅ concluída` · `🔄 em progresso` · `⬜ pendente` · `🚫 fora de escopo`.

| Fase | Estado | Commit | Observação |
|------|--------|--------|------------|
| 0 — Preparação (branch, baseline, specs) | ✅ concluída | `c7bb6a5` | branch criada; baseline `make check` verde (lint v1.64.8 advisory pulado, esperado); spec já versionada na `main` (b56d3a3), então 0.2 fechou só com o ADR da 0.3 |
| 1 — R4: golangci-lint v2 reativado | ✅ concluída | `af66a87` | spike ✅; `.golangci.yml` v2 + exclusões justificadas (misspell PT-BR, revive:error-strings/unused-parameter/exported/unexported-return/context-as-argument, **staticcheck:ST1005** — achado novo); Makefile (`tools`+`lint` obrigatório+`SKIP_LINT=1`) validado nos 4 cenários; CI com `make tools`; ADR R4 + README + deploy |
| 2 — R4: triagem dos achados do linter | ✅ concluída | `66ca737` (bugs), `af66a87` (estilo/docs) | 195→36 (pós-exclusões) → **0 issues**. Bugs reais corrigidos (errcheck, QF, S1017, SA1019, SA9005 nolint, builtins sombreados); estilo de baixo valor desligado por regra justificada. `make check` verde com o **linter real** (v2.15.0, 0 issues) |
| 3 — R7.4 + R1 + R5: ator real e auditoria clínica | ✅ concluída | `5a37ddb`, `de33df6` (E2E) | `domain.WithActor`/`ActorFrom`; `AuditService` resolve ator + `SetLogger`; `httpx.PropagateActor` nos grupos app/admin; `ProntuarioService.SetAudit` (ler/criar/atualizar, sem texto no meta); unit (domain, Record, PropagateActor, prontuario) + E2E real da trilha (ator real, não-vazamento, 401 não audita, session.cancel). `make check` verde (lint 0). TODO Fase 5 removido. ADR + §0.22 arch + aviso dev no deploy |
| 4 — R3: pasta de DR criada automaticamente | ✅ concluída (spike 4.1 pendente do Rafael) | `a59c98e` | DriveClient estendido (List/Trash/Find/Create/GetFolder/AppProps/ErrDriveNotFound); migration `0050_drive_dr` + repo; `FolderResolver` (config>persistido>busca>criação, não importa google/app); `wireDriveSync` sem exigir `drive_folder`; teste em processo do fluxo de backup. **Spike 4.1 (credencial Google) pendente**: implementei assumindo o pré-verificado, fallback registrado |
| 5 — R2: snapshot inteiro, retenção e degradação observável | ✅ concluída | `604a82a` | ADR + steering do Drive; `drive_retention`; `Syncer` novo (retenção por lixeira, Observer, Status); adapter `List/Trash`; `/readyz` `checks.drive_dr`; 404 persistente sem laço. Retenção em runtime depende do spike 4.1 |
| 6 — R7.1 + R7.2: rate limit e `state` do OAuth | ✅ concluída | `dea5247` | `httpx.Limiter` (5/h e-mail, 10/h paciente, 429+Retry-After); `OAuthStateStore` + cookie; `400 state_invalido`; testes unit/em processo/E2E; ADRs + deploy + arch §0.23 |
| 7 — R8 base: dependência, registry, middleware e listener | ✅ concluída | `b2cab83` | `client_golang` v1.25.0 (tidy só acrescenta); `app.Metrics`; `platform/metrics` (registry próprio); `httpx.Instrument` (raiz, prio -5000); listener fail-closed `metrics.addr`; ADR R8.12; E2E de isolamento/HTTP/401-403/vazamento/fail-closed |
| 8 — R8 famílias: negócio, integrações, auditoria, backup | ✅ concluída | `e6d8e89` | negócio por `SetMetrics`; filas (coletor c/ cache); decoradores Calendar/Gemini/cifra; auditoria; backup/DR; rate limit; `google_oauth_ok`; docs + ADR (fecha OBS-1 e R5.2) |
| 9 — R6 + R7.3: docs, steering e tabela da pocketbase-finish | ✅ concluída | `<fase9>` | steering sem Gmail + regras de E2E de métricas/429; coerência README/docs; tabela da `pocketbase-finish` consolidada (PR #4); checklist de rotação de segredos |
| 10 — Fechamento (release notes + checklist + push) | ⬜ pendente | — | push só com OK do Rafael |

**Agora:** Fases 0–9 concluídas (spike manual 4.1 pendente de credencial do Rafael). Gate real verde. Próxima ação: Fase 10 — fechamento (release notes, verificação final com `-race`, checklist) e então **PARAR e pedir o OK do Rafael antes do push** (task 10.4).

---

## ✅ Decisões do Rafael (2026-11-05) — a respeitar

1. **R2:** snapshot inteiro, sem backup incremental; steering atualizado. R2.1 e R2.2 descartados.
2. **R3:** criação automática da pasta; `google.drive_folder` passa a ser opcional. R3.2 descartado.
3. **R7.1:** rate limit por e-mail no `register` e por paciente da sessão em `appointment-requests`, mais o limite por IP do nginx (mantido).
4. **R7.2:** validar o `state` do OAuth no callback.
5. **R8:** `client_golang`; listener separado em porta interna, sem exposição à internet, fail-closed.
6. **Rotação de segredos (R7.3):** operacional, fica com o Rafael (task 9.5 e 10.3).

**Decisões fechadas (2026-11-05, por delegação do Rafael; ajustáveis na revisão)** (`design.md`, "Decisões fechadas"): retenção de 14 snapshots do Drive por lixeira (`trashed`, não `delete`); listener de métricas só sobe quando `metrics.addr` está configurado (`127.0.0.1:9091` no host, `0.0.0.0:9091` no container, porta não publicada); auditoria best-effort (falha ao gravar loga e conta métrica, sem derrubar a requisição); rate limit de 5 cadastros/h por e-mail normalizado e 10 pedidos de agendamento/h por paciente; baseline do linter se a triagem passar de ~100 achados (medido: 185, ≈35 depois de desligar os três ruídos justificados; ver task 1.5).

**Verificações pré-feitas (2026-11-05):** as suposições do design foram resolvidas localmente (relatório em `.agents/tasks/auditoria-profunda-spec/resolver-suposicoes.md`, git-ignored). As tasks de spike abaixo continuam, mas só para **reconfirmar rapidamente** (o ambiente pode mudar). Só ficou aberto, por exigir credencial real do Google, o comportamento do Drive com pai na lixeira ou apagado e a confirmação em runtime do `list`/`update` sob `drive.file` (task 4.1). Refutado: o nome `CalendarGateway` (as interfaces reais são `app.SessionCalendar` e `app.ConflictChecker`) e a menção a `swagger.go` para declarar `429` (a spec é `openapi.base.json`).

---

## Fase 0 — Preparação

- [x] **0.1 Criar a branch da onda a partir da `main`** (dep: —) **(PRIMEIRA TASK)**
  - `git switch main`, confirmar a `main` atualizada com `git status`, `git switch -c auditoria-profunda`. Não trabalhar na `main` em nenhum momento.
  - **Pronto quando:** `git branch --show-current` imprime `auditoria-profunda`.
  - **Refs:** `psicoman-processo-entrega.md` (Fluxo de Git).
- [x] **0.2 Baseline do gate e inclusão das specs na branch** (dep: 0.1) — baseline `exit=0` (`/tmp/check-baseline.log`: gofmt+vet+build+testes unit/E2E verdes; lint v1.64.8 advisory pulado por incompatibilidade com Go 1.27, que a Fase 1 corrige). `git status` limpo: os três arquivos da spec **já estavam commitados na `main`** (b56d3a3), então não há o commit "adiciona a spec"; a Fase 0 fecha só com o ADR da 0.3.
  - Rodar `make check > /tmp/check-baseline.log 2>&1; echo "exit=$?"` e registrar o resultado na observação da Fase 0. Com `git status`, listar o que está modificado/novo: `.kiro/specs/auditoria-profunda/requirements.md` (modificado), `design.md` e este `tasks.md` (novos). O diff do `requirements.md` vem das decisões do Rafael e das correções factuais já aplicadas; os três arquivos de spec entram juntos no primeiro commit da branch. Nenhum arquivo de `.agents/` entra no commit (diretório git-ignored).
  - Arquivos: `.kiro/specs/auditoria-profunda/requirements.md`, `.kiro/specs/auditoria-profunda/design.md`, `.kiro/specs/auditoria-profunda/tasks.md` (novo).
  - **Pronto quando:** baseline `exit=0` anotado; `git status` não mostra nada fora dos arquivos de spec esperados.
- [x] **0.3 Registrar a spec em `docs/decisions.md`** (dep: 0.2) — entrada "Onda `auditoria-profunda`" no topo (data, contexto, decisão, consequências), apontando para `.kiro/specs/auditoria-profunda/`.
  - Entrada curta no topo do arquivo: contexto (auditoria de 2026-11-05), decisão (onda `auditoria-profunda` com as decisões do Rafael acima), consequência (ponteiro para `.kiro/specs/auditoria-profunda/`). Atende à regra "toda spec nascida de auditoria tem entrada correspondente".
  - Arquivo: `docs/decisions.md`.
  - **Pronto quando:** a entrada existe, com data, contexto, decisão e consequências, apontando para a spec.
- [x] **Fechamento Fase 0:** `make check` verde; commit `c7bb6a5` `auditoria-profunda Fase 0: registra a decisao da onda e inicia o acompanhamento no tasks.md` (ajuste: spec já estava na `main`, então o commit traz o ADR e o acompanhamento, não a spec).

---

## Fase 1 — R4: golangci-lint v2 reativado

> Dep. de fase: Fase 0. Faz o gate real valer antes de qualquer código novo. Se o spike 1.1 falhar, **a fase para** e o Rafael decide (esperar release, ou rodar o linter com outro toolchain, perdendo análise de código do Go 1.27).

- [x] **1.1 SPIKE: golangci-lint v2.15.0 compilado com o toolchain local** (dep: —)
  - `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0`, depois `golangci-lint run ./... > /tmp/lint-spike.log 2>&1; echo "exit=$?"`. Contar achados por linter (`grep -c`) e anotar o total na observação da fase. Sem alterar arquivo do repositório nesta task.
  - **Pré-verificado em 2026-11-05:** `v2.15.0` existe (última v2); `GOBIN=/tmp/gobin go install` compila com go1.27.1; `golangci-lint run` decodifica o export data do Go 1.27. Achados: **185** com a config produzida por `migrate` (`revive` 127, `misspell` 43, `staticcheck` 15) e **195** com o rascunho do design §4 (`errcheck` 8, `misspell` 43, `revive` 129, `staticcheck` 15); `config verify` passa no rascunho. Top de regras: `revive:error-strings` ≈60, `revive:unused-parameter` ≈56, `misspell` 43 (falsos positivos em português).
  - **Pronto quando (reconfirmar rapidamente):** repetir `go install`, `golangci-lint version` e `run`; o linter decodifica o export data do Go 1.27 (sem a mensagem "não decodifica o export data") e o total de achados está registrado e é da mesma ordem (~190). Falha → parar e reportar ao Rafael.
  - **✅ RECONFIRMADO (2026-11-06):** `GOBIN=/tmp/gobin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0` → `version 2.15.0 built with go1.27.1`; `run` com o rascunho v2 → exit 1, **0 erros de export data/typecheck**, **195 achados** (revive 129, misspell 43, staticcheck 15, errcheck 8). `config verify` do rascunho = exit 0. Idêntico ao pré-verificado. Logs: `/tmp/lint-spike.log`.
  - **Refs:** R4.1 · design §4 e §11.5 item 1.
- [x] **1.2 Migrar `.golangci.yml` para o formato v2** (dep: 1.1) — adotado o rascunho explícito do design §4 (`version: "2"`, `default: none`, enable de 7 linters, `gofmt` em `formatters`, exclusão de `errcheck` em `_test.go`). `config verify` = exit 0; `run ./...` executa (195 achados).
  - **Pré-verificado em 2026-11-05:** o `migrate` mantém o conjunto padrão de linters, acrescenta `exclusions.presets` (que escondem os `Close()` do `errcheck`) e descarta `run.timeout`; o rascunho do design §4 passa no `config verify`. Preferir o rascunho explícito.
  - Rodar `golangci-lint migrate`, revisar à mão contra o rascunho do design §4 (`version: "2"`, `linters.default: none`, `enable: [errcheck, govet, ineffassign, staticcheck, unused, misspell, revive]`, `formatters.enable: [gofmt]`, exclusão de `errcheck` em `_test.go` em `linters.exclusions.rules`).
  - Arquivo: `.golangci.yml`.
  - **Pronto quando:** `golangci-lint config verify` sai com 0 e `golangci-lint run ./...` executa de fato.
  - **Refs:** R4.1 · design §4.
- [x] **1.3 Reescrever o alvo `lint` do Makefile e adicionar `tools`** (dep: 1.2) — `GOLANGCI_LINT_VERSION ?= v2.15.0`; alvo `tools`; `lint: fmt-check vet` com 3 guardas (ausente→exit 1 pedindo `make tools`; versão≠fixada→exit 1; ok→`run ./...` sem `|| echo`/`@true`/probe/timeout); escape `SKIP_LINT=1` barulhento; cabeçalho atualizado; `check` encadeia `lint`. **Validação manual (logs /tmp/lint-t{1..4}.log):** (1) ausente→exit≠0 "Rode 'make tools'"; (2) shim v2.14.0→exit≠0 "difere da versão fixada"; (3) SKIP_LINT=1→exit 0 "LINT PULADO POR PEDIDO EXPLÍCITO"; (4) linter certo→roda de verdade (imprime `golangci-lint v2.15.0`, 195 achados) e **gate quebra** (exit 2). O estado pré-triagem do repo já prova a quebra do gate; a Fase 2 zera os achados.
  - `GOLANGCI_LINT_VERSION ?= v2.15.0`; alvo `tools` com `go install ...@$(GOLANGCI_LINT_VERSION)`; `lint: fmt-check vet` seguido de: linter ausente → `exit 1` com "rode `make tools`"; versão diferente da fixada → `exit 1`; senão `golangci-lint run ./...` **sem** `|| echo`, **sem** `@true`, **sem** probe e **sem** `timeout` externo. Escape explícito `make check SKIP_LINT=1` que imprime `!! LINT PULADO POR PEDIDO EXPLÍCITO` e sai 0. Atualizar o comentário do cabeçalho do Makefile ("lint advisory"). Checar que `check:` (linha 75) continua encadeando `lint`.
  - Arquivo: `Makefile`.
  - **Pronto quando (validação manual, anotada na observação da fase):** `make lint` com linter ausente (via `PATH` sem o binário) falha; com versão errada falha; com `SKIP_LINT=1` avisa e passa; com tudo certo roda de verdade. Prova de que o gate quebra: inserir um `ineffassign` temporário em um arquivo, ver `make lint` falhar, remover e ver passar.
  - **Refs:** R4.2 · design §4.
- [x] **1.4 Instalar o linter no CI** (dep: 1.3) — no job `check`, antes de `make check`: passo "install golangci-lint (make tools)" que roda `make tools` e acrescenta `$(go env GOPATH)/bin` ao `$GITHUB_PATH`. Não usa a action em modo binário (comentário explica o motivo do export data). YAML validado (`python3 yaml.safe_load` = válido); passo posicionado antes do `make check`.
  - No job `check` de `.github/workflows/ci.yml`, antes de `make check`: passo `make tools` e `$(go env GOPATH)/bin` no `PATH`. Não usar action em modo binário (binário pré-compilado repete o erro de export data).
  - Arquivo: `.github/workflows/ci.yml`.
  - **Pronto quando:** `yq`/leitura do YAML mostra o passo antes de `make check`; sintaxe do workflow válida (conferir com `actionlint` se existir; senão, revisão manual anotada).
  - **Refs:** R4.2 · design §4.
- [x] **1.5 Baseline condicional e docs de R4** (dep: 1.3) — configuradas 4 exclusões justificadas no YAML: `misspell` (43 falsos positivos PT-BR), `revive:error-strings` (≈60), `revive:unused-parameter` (≈56) e **`staticcheck:ST1005`** (≈60 — **achado novo não previsto no design**: na v2 o staticcheck absorve o stylecheck e reporta o mesmo check de capitalização do revive:error-strings; desligado pela mesma justificativa). Pós-exclusões: **36 achados** (staticcheck 15, revive 13, errcheck 8) < 100 → **sem baseline `new-from-rev`**. ADR de R4 criado em `docs/decisions.md`; `README.md` e `docs/deploy.md` passam a instruir `make tools`.
  - **Antes de decidir o baseline**, configurar as três exclusões justificadas (comentário no YAML): `misspell` (falsos positivos em português), `revive:error-strings` e `revive:unused-parameter`; medido em 2026-11-05, isso deixa ≈35 achados (8 `errcheck` de `Close()`, 15 `staticcheck`, resto de `revive`). Contar de novo **depois** das exclusões e aplicar a regra abaixo a esse número.
  - Se o spike contou mais de ~100 achados (já com as exclusões): `issues.new-from-rev` no YAML apontando para o commit anterior à fase (o gate cobra só código novo) e abrir a task de dívida na Fase 2. Caso contrário, nenhum baseline. Atualizar `README.md` e `docs/deploy.md` (instalar com `make tools`) e criar o ADR curto em `docs/decisions.md` ("linter v2, fim do modo advisory", escolha do baseline).
  - Arquivos: `.golangci.yml`, `README.md`, `docs/deploy.md`, `docs/decisions.md`.
  - **Pronto quando:** `make lint` roda o linter e sai 0 (com baseline, se aplicável); ADR e docs citam `make tools`.
  - **Refs:** R4.1, R4.2 · design §4.
- [x] **Fechamento Fase 1:** commit `af66a87` `auditoria-profunda Fase 1: reativa o golangci-lint v2 no gate e no CI`. Para respeitar "cada commit deixa a base consistente" (steering), a ordem foi: (1) `66ca737` correções de bug — passam no gate antigo (advisory); (2) `af66a87` reativação v2 + exclusões de estilo + docs — com os bugs já corrigidos, `make check` fica verde com o linter real. Nenhum commit publicado deixou o gate quebrado.

---

## Fase 2 — R4: triagem dos achados do linter

> Dep. de fase: Fase 1. Commits **separados** do trabalho funcional das demais fases, para o diff funcional não ser contaminado (design §4).
>
> **Nota de execução (2026-11-06):** sem baseline, a reativação do linter (Fase 1) deixa 36 achados pré-existentes expostos, o que impede `make check` de ficar verde num commit só da Fase 1. Para respeitar "cada commit deixa a base consistente" (steering de processo), a ordem de commits é: (a) triagem que zera os achados (bugs reais e estilo), depois (b) o commit de reativação da Fase 1 (`.golangci.yml`, `Makefile`, CI, docs/ADR) — ou, de forma equivalente e mais simples, a reativação e as correções entram como commits separados por natureza em sequência contígua, todos verificados com `make check` verde ao final. O importante: nenhum commit publicado deixa o gate quebrado.

- [x] **2.1 Classificar os achados e corrigir os bugs reais** (dep: 1.5) — corrigidos: 8 `errcheck` de `Close()` (drivesync, gemini, calendar, drive, ged_store) → `defer func(){ _ = x.Close() }()`; 1 `S1017` (drivesync `encName`) → `strings.TrimSuffix`; 7 `QF1012` (pdf.go) → `fmt.Fprintf`; 1 `QF1001` (httpx_test) → De Morgan; 1 `SA9005` (sealed_text_test) → `//nolint` justificado (marshal de tipo opaco é o invariante); 5 `SA1019` → `ResetBootstrapState`→`ClearBootstrap` (3) e `go/parser.ParseDir`→helper `ParseFile` (2, leak_test, sem dep nova). Builtins sombreados: `clear`→`plain` (health.go), `func min` redundantes removidos (pdf_test, invoice_test). Testes unit+e2e verdes.
- [x] **2.2 Tratar o estilo de baixo valor** (dep: 2.1) — desligadas por regra no `.golangci.yml`, com comentário justificando cada uma: `revive:exported` (stutters — renomear DTOs/tipos públicos é churn de API sem ganho), `revive:unexported-return` (construtor devolve concreto, consumido via interface — intencional), `revive:context-as-argument` (`resolvePatient(e, ctx)`, `e` é o portador do handler). `golangci-lint run ./...` → **0 issues**. O único `//nolint` (SA9005) tem justificativa na mesma linha.
- [x] **Fechamento Fase 2:** `make check` verde com o linter real (v2.15.0, 0 issues; `/tmp/check-fase2.log`); ADR de R4 atualizado com o desfecho da triagem; commits por natureza: `66ca737` (bugs: errcheck/staticcheck/builtins) e `af66a87` (reativação + estilo + docs). _Nota: as desativações de estilo e o ADR entraram no mesmo commit da reativação (`af66a87`), porque o `.golangci.yml` é indivisível; ambos os commits deixam o gate verde._

---

## Fase 3 — R7.4 + R1 + R5: ator real e auditoria clínica

> Dep. de fase: Fase 1 (lint no gate). Ordem interna: ator (3.1 a 3.3) → R1 (3.4 a 3.5) → testes (3.6 a 3.7).

- [x] **3.1 `domain.WithActor`/`ActorFrom` e `ActorSystem`** (dep: —) — `internal/domain/actor.go` + teste (ctx sem/com ator, sobrescrita, valor vazio propagado). `domain` segue só stdlib.
  - Arquivos: `internal/domain/actor.go` (novo), `internal/domain/actor_test.go` (novo). Só stdlib; `domain` continua sem PocketBase.
  - **Pronto quando:** `go test ./internal/domain/...` passa cobrindo ctx sem ator (`""`), com ator e sobrescrita.
  - **Refs:** R7.4 · design §6.
- [x] **3.2 `AuditService.Record` resolve o ator; logger injetável** (dep: 3.1) — `Record` troca actor `""`/`sistema` por `ActorFrom(ctx)`, senão `sistema`; campo `log *slog.Logger` + `SetLogger` (default `slog.Default()`); falha de `Insert` logada (`slog.Error`, sem meta/ids). Métrica fica para a Fase 8 (8.4). Teste: ator do ctx, sistema explícito trocado, sem ator = sistema, falha não grava e é observável. `fakeAuditRepo.insertErr` adicionado.
  - `Record` troca `actor` `""` ou `domain.ActorSystem` por `domain.ActorFrom(ctx)`, e por `ActorSystem` se ainda vazio. `AuditService` ganha `log *slog.Logger` (default `slog.Default()`) e `metrics Metrics` com `SetLogger`/`SetMetrics`. **Como `app.Metrics` só nasce na Fase 7**, nesta task o campo `metrics` fica de fora e a falha de `repo.Insert` é apenas logada com `slog.Error("falha ao gravar audit_log", "acao", ..., "entidade", ...)`, sem `meta` nem ids; a Fase 8 (task 8.4) acrescenta `SetMetrics`. As 11 chamadas existentes com `"sistema"` (ai, payment, billing x3, invoice, history, session x3, dayfeed) **não** são editadas.
  - Arquivos: `internal/app/audit_service.go`, `internal/app/core_services_test.go` ou novo `internal/app/audit_service_test.go` (novo; usar `fakeAuditRepo` de `internal/app/fakes_test.go`).
  - **Pronto quando:** `go test ./internal/app/...` cobre: ctx com ator grava o ator; sem ator grava `"sistema"`; `"sistema"` explícito com ctx com ator troca pelo ator; erro do repo é logado e não propagado.
  - **Refs:** R7.4, R1 (falha observável) · design §1 e §6.
- [x] **3.3 Middleware `httpx.PropagateActor` e ligação nos grupos `/v1/app` e `/v1/admin`** (dep: 3.1) — `internal/api/httpx/actor.go` (`WithContext(domain.WithActor(...))`), ligado após `RequirePangolinIdentity` nos dois grupos em `api.go`. TODO Fase 5 de `sessions.go` removido (comentário explica que `CancelByTherapist` é o papel, não a identidade). Teste do `PropagateActor` (com/sem identidade). Build ok.
  - `PropagateActor` lê `ActorIdentity(e)` e faz `e.Request = e.Request.WithContext(domain.WithActor(...))`, ligado **depois** de `RequirePangolinIdentity`. Remover o `// TODO Fase 5` de `internal/api/app/sessions.go` (~l.127) e trocar por comentário explicando que `scheduling.CancelByTherapist` está correto em single-tenant. Confirmar em `internal/api/api.go` onde os grupos `app` e `admin` são montados.
  - Arquivos: `internal/api/httpx/auth.go` (ou novo `internal/api/httpx/actor.go`), `internal/api/httpx/auth_test.go` (padrão), `internal/api/api.go`, `internal/api/app/sessions.go`.
  - **Pronto quando:** unit do `PropagateActor` coloca o ator no ctx; `go test ./internal/api/...` verde; o TODO sumiu (`go build ./...` ok).
  - **Refs:** R7.4 · design §6.
- [x] **3.4 `ProntuarioService.SetAudit` e pontos de auditoria** (dep: 3.2) — campo `audit` + `SetAudit`; auditoria best-effort só após sucesso: `GetChart`→`ler`/`prontuario` (`{patient_id, notas, anamnese}`), `AddNote`→`criar`/`note`, `UpsertAnamnesis`→`criar`(ErrNotFound)/`atualizar`/`anamnesis`. Nunca texto no meta. Fiação `prontuario.SetAudit(audit)` em `wiring.go`.
  - Campo `audit *AuditService`, `SetAudit(a *AuditService)`. Auditar **só após sucesso**, best-effort: `GetChart` → `AuditActionRead` em `prontuario`/`patientID` com `{patient_id, notas, anamnese}`; `AddNote` → `AuditActionCreate` em `note` com `{patient_id, session_id}`; `UpsertAnamnesis` → `Create` (ramo `ErrNotFound`) ou `Update` em `anamnesis` com `{patient_id}`. Nunca texto clínico no `meta`.
  - Arquivos: `internal/app/prontuario_service.go`, `internal/api/wiring.go` (após `app.NewProntuarioService(prontuarioRepo, nil)`: `prontuario.SetAudit(audit)`).
  - **Pronto quando:** `go build ./...` ok; os testes da 3.6 passam.
  - **Refs:** R1.1, R1.2, R1.3, R1.4 · design §1.
- [x] **3.5 Ligar logger do audit no wiring** (dep: 3.2) — `audit.SetLogger(a.Logger())` em `buildServices`. Build ok.
  - `audit.SetLogger(a.Logger())` em `buildServices`.
  - Arquivo: `internal/api/wiring.go`.
  - **Pronto quando:** `go build ./...` ok e uma falha forçada de `Insert` aparece no log do PocketBase em teste da 3.2.
  - **Refs:** R1 · design §1.
- [x] **3.6 Testes unit de R1 (R5.1)** (dep: 3.4) — `prontuario_service_test.go`: GetChart/AddNote/UpsertAnamnesis (criar+atualizar) gravam a ação certa; `metaContemTexto` varre valores string (incl. nested/Stringer) garantindo não-vazamento; falha da operação não audita; `audit==nil` não panica. `fakeProntuarioRepo` ganhou erros opcionais. Verde.
  - Novo arquivo com fake de `AuditRepository` reaproveitando `fakeAuditRepo` e o helper `newAudit()` de `internal/app/fakes_test.go` (**verificado em 2026-11-05**; o repositório de prontuário fake é `fakeProntuarioRepo`, no mesmo arquivo). Casos: `GetChart`, `AddNote`, `UpsertAnamnesis` (criar e atualizar) gravam a ação certa; `meta` não contém o texto enviado (varrer valores `string`); falha da operação não grava nada; `audit == nil` não panica.
  - Arquivo: `internal/app/prontuario_service_test.go` (novo).
  - **Pronto quando:** `go test ./internal/app/ -run Prontuario` passa.
  - **Refs:** R5.1, R1.4 · design §1 e §5.
- [x] **3.7 E2E da trilha de auditoria (R5.2)** (dep: 3.3, 3.4) — `test/e2e/audit_prontuario_test.go`: cria paciente, POST notes, PUT anamnesis, GET chart (todos com `X-Pangolin-User`), lê `GET /v1/admin/audit-log` conferindo ações/entidades, `actor==header` e ausência do texto clínico no JSON inteiro; **negado:** GET chart sem header → 401 e nenhuma leitura nova de prontuário na trilha; `TestAuditSessionCancel` confere ator real em `session.cancel`. Sem `t.Skip`. Ambos verdes.
  - `test/e2e/audit_prontuario_test.go` (novo), usando o helper de subida do binário já usado em `test/e2e/core_test.go`. Fluxo: cria paciente, `POST /patients/{id}/notes`, `PUT /patients/{id}/anamnesis`, `GET /patients/{id}`, todos com `X-Pangolin-User`; lê `GET /v1/admin/audit-log`; confere ações/entidades, `actor == X-Pangolin-User` e ausência do texto clínico no JSON inteiro. **Negado:** `GET /patients/{id}` sem header → `401` e nenhuma entrada de auditoria nova. Adicionar um `POST .../sessions/{id}/cancel` conferindo o ator no `audit_log` (`session.cancel`).
  - **Pronto quando:** `go test ./test/e2e/ -run AuditProntuario` passa; sem `t.Skip`.
  - **Refs:** R5.2, R7.4, R1 · design §1, §5 e §6.
- [x] **3.8 Docs e ADR da fase** (dep: 3.7) — ADR "Auditoria no caminho clínico e ator real" em `docs/decisions.md`; §0.22 nova em `docs/architecture.md` (operações auditadas + ator propagado) e §0.7 atualizada (consumidor do ActorIdentity); aviso em `docs/deploy.md` de que `pangolin.dev_identity` não é identidade verificada.
  - ADR "Auditoria no caminho clínico" (best-effort com log e métrica, alternativa fail-closed descartada; ator por contexto; portal fora do escopo de R7.4) em `docs/decisions.md`; lista de operações auditadas e ator propagado em `docs/architecture.md`; aviso em `docs/deploy.md` de que o ator em modo dev não é identidade verificada.
  - Arquivos: `docs/decisions.md`, `docs/architecture.md`, `docs/deploy.md`.
  - **Pronto quando:** os três documentos citam as operações auditadas e o ator em modo dev.
  - **Refs:** R1, R7.4, R10 · design §1 e §6.
- [x] **Fechamento Fase 3:** `make check` verde (`/tmp/check-fase3.log`, lint 0 issues, unit+e2e verdes); docs/ADR atualizados; commits `5a37ddb` (código+docs) e `de33df6` (E2E da trilha, que faltou no primeiro `git add`).

---

## Fase 4 — R3: pasta de DR criada automaticamente

> Dep. de fase: Fase 1. Ordem interna: spike manual (4.1) → cliente Drive (4.2) → persistência (4.3) → resolver (4.4) → wiring e config (4.5) → testes e docs (4.6).

- [~] **4.1 SPIKE manual: escopo `drive.file` e pasta na lixeira** (dep: —) — **PENDENTE DO RAFAEL (credencial real).** A doc oficial confirma `list`/`update`(trashed)/`create` sob `drive.file`; a obs. empírica (pasta manual só via `google.drive_folder`) está no ADR. **Em aberto:** upload com pai na lixeira/apagado (não verificável localmente). O design é robusto nos dois casos (revalidação `trashed` + retry único); fallback registrado (retenção desativa com aviso se `list`/`update` for negado). Implementei a fase assumindo o pré-verificado; **não usei tokens do Rafael**. _Reconfirmar em runtime quando houver credencial._
  - Com credencial de dev (fora da suíte automatizada, sem copiar segredos): confirmar que `drive.file` permite `files.list`, `files.get` e `files.update` (lixeira) sobre arquivos criados pelo app; criar uma pasta, mandá-la para a lixeira e fazer upload com ela como pai, anotando o status devolvido. Registrar o resultado no ADR de R3 (task 4.6). Sem credencial disponível, registrar "não verificado" e seguir: a revalidação `trashed` do resolver vale nos dois desfechos, mas a retenção (Fase 5) fica condicionada (fallback no design §2).
  - **Pré-verificado em 2026-11-05:** (a) a documentação oficial do Google diz que `drive.file` "funciona com todos os recursos REST da Drive API" com acesso por arquivo ao que o app criou ou abriu: `files.list`, `files.update` (`trashed`) e `files.create` de pasta são permitidos para arquivos do próprio app (**confirmada pela documentação**; falta runtime). (b) Observação empírica (conta real do Rafael): upload com `parents:[<pasta criada à mão>]` **funcionou** sob `drive.file`; a **busca** só enxerga o que o app criou, então a pasta manual não é achada pelo passo 3 e só vale via `google.drive_folder`. (c) Upload com pai na **lixeira** ou **apagado**: **não verificável localmente** (exige credencial real); o design é robusto a qualquer resultado.
  - **Pronto quando (reconfirmar rapidamente, com credencial de dev):** `files.list` e `files.update` (lixeira) sobre arquivo criado pelo app funcionam em runtime; o status do upload com pai na lixeira e com pai apagado está anotado (é o único item realmente em aberto); o resultado vai para a observação da fase e para o ADR (task 4.6).
  - **Refs:** R3.1, R2.4 · design §2, §3 e §11.5 item 2.
- [x] **4.2 `DriveClient`: `CreateFolder`, `FindFolder`, `GetFolder`, `List`, `Trash`, `AppProps`, `ErrDriveNotFound`** (dep: —) — interface estendida em `drive.go`; métodos REST em `drive_folder.go` (paginação com teto 10×100; `appProperties`; 404→`ErrDriveNotFound` em Upload/GetFolder); vars `driveFilesURL`; `FakeDriveClient` ganhou Files/ListErr/TrashErr/Trashed/FolderID/FolderFound/erros/contadores. Testes httptest de todos os métodos. Verde.
  - `DriveFile`, `DriveListInput`, novos métodos na interface `DriveClient`; `DriveUploadInput.AppProps` gravado como `appProperties` no multipart; `ErrDriveNotFound` (código novo: hoje não existe sentinela nem tratamento de 404) mapeado a partir de 404 no `Upload` e no `GetFolder`; vars de pacote `driveFilesURL` análogas a `driveUploadURL`; paginação de `List` por `pageToken` com teto de 10 páginas de 100. `FakeDriveClient` ganha `Files`, `ListErr`, `TrashErr`, `Trashed`, contadores de `CreateFolder`/`FindFolder`/`GetFolder`.
  - Arquivos: `internal/integration/google/drive.go`, `internal/integration/google/fake.go`, `internal/integration/google/drive_test.go` (servidor `httptest`).
  - **Pronto quando:** `go test ./internal/integration/google/...` cobre `CreateFolder`, `FindFolder`, `GetFolder` (inclui 404 → `ErrDriveNotFound`), `List`, `Trash` e o `appProperties` no multipart.
  - **Refs:** R3.1 · design §2 e §3.
- [x] **4.3 Migration `0050_drive_dr`, porta e adapter `DriveFolderRepository`** (dep: —) — `0050_drive_dr.go` (coleção singleton, `folder_id` obrigatório, timestamps, down=dropCollections); porta `app.DriveFolderRepository` (Load/Save); adapter `pb.NewDriveFolderRepository`. Testes: round-trip, singleton sobrescreve, Load vazio→ErrNotFound, coleção existe, idempotência (grupoE). Verde.
  - Coleção base `drive_dr` (`folder_id` obrigatório, `folder_name`, timestamps; regras nulas) com `down` = `dropCollections`. Porta `DriveFolderRepository` (`Load`, `Save` singleton) em `internal/app/ports.go`; adapter no estilo de `google_token_repo.go`. O id da pasta fica em claro (não é segredo).
  - Arquivos: `internal/platform/pb/migrations/0050_drive_dr.go` (novo), `internal/platform/pb/migrations/migrations_test.go`, `internal/app/ports.go`, `internal/platform/pb/drive_folder_repo.go` (novo), `internal/platform/pb/drive_folder_repo_test.go` (novo; padrão `tests.NewTestApp` de `pb_test.go`).
  - **Pronto quando:** `go test ./internal/platform/pb/...` cobre round-trip de `Save`/`Load`, `Save` sobrescrevendo o singleton e `Load` vazio → `app.ErrNotFound`.
  - **Refs:** R3.1, R3.4 · design §3.
- [x] **4.4 `FolderResolver` no `drivesync`** (dep: 4.2) — `folder.go`: `FolderClient`/`FolderStore`, sentinelas `ErrFolderNotFound`/`ErrNoStoredFolder`, resolução config>persistido(revalidado 1x)>busca>criação, `Invalidate`, mutex. **Não importa integration/google nem app** (verificado por `go list -deps`). 10 casos de teste (config vence, persistido reusado/validado 1x, lixeira/404 recria, busca/criação persistem, 2 ID=1 create, falha create não persiste, Invalidate não relê, config+Invalidate não recria). Verde.
  - Interfaces `FolderClient` e `FolderStore`, sentinelas locais `ErrFolderNotFound` e `ErrNoStoredFolder`, `FolderResolver` com ordem config > persistido (revalidado uma vez por processo via `GetFolder`) > busca por `appProperties` > criação, `Invalidate`, mutex. O pacote **não** importa `integration/google` nem `app`.
  - Arquivos: `internal/drivesync/folder.go` (novo), `internal/drivesync/folder_test.go` (novo).
  - **Pronto quando:** `go test ./internal/drivesync/ -run Folder` cobre: config vence sem chamar o client; persistido reusado e validado uma vez; persistido na lixeira ou 404 → busca/criação e novo id salvo; `Invalidate` seguido de `ID` não relê o persistido; busca acha e persiste; criação persiste; dois `ID` seguidos = 1 `CreateFolder`; falha de `CreateFolder` não persiste; `Invalidate` com id do config não recria.
  - **Refs:** R3.1, R3.4 · design §3 e §3.2.
- [x] **4.5 Config opcional, adapter e `wireDriveSync` sem exigir `drive_folder`** (dep: 4.3, 4.4) — `driveUploaderAdapter` troca `folderID` por `resolver`; `Upload` resolve a pasta, grava `AppProps` snapshot, traduz `google.ErrDriveNotFound`→`drivesync.ErrFolderNotFound` e, com pasta persistida, invalida+refaz **uma vez**. Adapters `driveFolderClientAdapter` e `driveFolderStoreAdapter` (traduzem erros na fronteira). `wireDriveSync` exige só client_id/secret+refresh; seam ganhou `folders app.DriveFolderRepository`; `buildServices` cria `pb.NewDriveFolderRepository`. `config.example.yaml`/`deploy/config.docker.yaml` comentam `drive_folder` opcional. Build + `go test ./internal/api/` verdes.
  - `driveUploaderAdapter` troca `folderID string` por `resolver *FolderResolver`; no `Upload`, resolve a pasta, traduz `google.ErrDriveNotFound` em `drivesync.ErrFolderNotFound` (com `%w`), e com id **persistido** invalida e refaz **uma única vez** (com id do config, o 404 vira erro observável, sem criar pasta). Adapter de `FolderStore` converte `app.ErrNotFound` em `drivesync.ErrNoStoredFolder`. `wireDriveSync` deixa de exigir `DriveFolder`; assinatura do seam `driveSyncWirer` ganha `folders app.DriveFolderRepository` (hoje nenhum teste usa o seam, confirmado por grep). Mensagem de desativação cita só `client_id/client_secret` e refresh token. `buildServices` cria `pb.NewDriveFolderRepository(a)` e o passa. Atualizar `config.example.yaml` e `deploy/config.docker.yaml` (comentar `drive_folder` como opcional).
  - Arquivos: `internal/api/wiring.go`, `internal/config/config.go` (comentários/validação, sem mudar o tipo), `config.example.yaml`, `deploy/config.docker.yaml`.
  - **Pronto quando:** `go build ./...` ok; `go test ./internal/api/...` verde; mensagem de desativação sem `drive_folder`.
  - **Refs:** R3.4 · design §3.
- [x] **4.6 Testes de integração do fluxo e docs** (dep: 4.5) — `internal/api/drive_dr_test.go`: reatribui `driveSyncWirer` com `FakeDriveClient`, `POST /v1/admin/backup` (202) → 1 CreateFolder, 1 Upload com FolderID da pasta criada + AppProps snapshot, registro em `drive_dr`, 2º backup sem nova criação; pasta obsoleta (404) recria uma vez e repete o upload (client `recriaDriveClient`). `TestBackupSemGoogleNao500` segue verde. ADR da pasta de DR em `docs/decisions.md` (inclui resultado/fallback do spike 4.1). Docs §0.10/deploy/restauração ficam na 5.7.
  - Teste em processo em `internal/api` reatribuindo o seam `driveSyncWirer` com `FakeDriveClient`, disparando `POST /v1/admin/backup`: 1 `CreateFolder`, 1 `Upload` com o `FolderID` da pasta criada, registro em `drive_dr`, segundo backup sem nova criação. Pasta obsoleta (`ErrFolderNotFound`) recria uma vez e repete o upload; segunda falha não faz laço (em `internal/drivesync/drivesync_test.go` ao entrar a Fase 5, aqui no nível do adapter). O E2E `TestBackupSemGoogleNao500` em `test/e2e/backup_test.go` permanece verde. ADR da pasta de DR (persistência em `drive_dr`, ordem de resolução, resultado do spike 4.1, e a observação de que a pasta criada à mão aceita upload sob `drive.file` mas não é achada pela busca, então só vale via `google.drive_folder`) em `docs/decisions.md`; `docs/deploy.md` e `docs/architecture.md §0.10` na task 5.7.
  - Arquivos: `internal/api/drive_dr_test.go` (novo), `docs/decisions.md`.
  - **Pronto quando:** `go test ./internal/api/ -run DriveDR` e `go test ./test/e2e/ -run Backup` passam.
  - **Refs:** R3.1, R3.4 · design §3.2.
- [x] **Fechamento Fase 4:** `make check` verde; ADR da pasta de DR atualizado; commit `a59c98e` `auditoria-profunda Fase 4: cria e reutiliza a pasta de DR do Drive automaticamente`. **Ressalva:** spike manual 4.1 (`drive.file` em runtime, pai na lixeira) pendente de credencial real do Rafael — fase implementada sobre o pré-verificado, fallback documentado.

---

## Fase 5 — R2: snapshot inteiro, retenção e degradação observável

> Dep. de fase: Fase 4 (cliente Drive, resolver e wiring). Entrega a decisão R2.3 (ADR + steering), a retenção R2.4, o `Syncer` novo, a degradação R3.3 e o `/readyz`.

- [x] **5.1 ADR do snapshot inteiro e da política de retenção (R2.3, R2.4)** (dep: —) — ADR "Fase 5" em `docs/decisions.md`: snapshot inteiro (consequências: crescimento linear, banda, restauração em um arquivo), retenção N=14 por lixeira, degradação observável e o fallback do spike 4.1.
  - Duas entradas (ou uma com duas decisões) em `docs/decisions.md`: backup é snapshot inteiro cifrado (consequências: crescimento linear, banda proporcional, restauração em um arquivo) e retenção N=14 por lixeira (`google.drive_retention`, `0` = manual), com o fallback "retenção manual" se o spike 4.1 negou `list`/`update`.
  - Arquivo: `docs/decisions.md`.
  - **Pronto quando:** as decisões estão registradas no formato data/contexto/decisão/consequências e citam o spike.
  - **Refs:** R2.3, R2.4 · design §2.
- [x] **5.2 Atualizar o steering `psicoman-google-api.md`: Drive em snapshot inteiro (R2.3)** (dep: 5.1) — removidas as linhas de incremental/manifesto; seção Drive agora descreve snapshot inteiro cifrado, retenção por lixeira e pasta automática. (O Gmail segue para a 9.1.)
  - Trocar "Backup incremental por hash (manifesto no Drive)..." por "Snapshot inteiro por execução, cifrado (AES-256-GCM) antes do upload; o zip do PocketBase já é compactado; retenção por contagem feita pelo `drivesync`"; acrescentar a criação automática da pasta. Sem incremental e sem manifesto. **A remoção do Gmail neste arquivo fica na task 9.1** (mesmo arquivo, mudança de motivo diferente).
  - Arquivo: `.kiro/steering/psicoman-google-api.md`.
  - **Pronto quando:** `grep -in "incremental\|manifesto" .kiro/steering/psicoman-google-api.md` não encontra a exigência antiga e a seção Drive descreve snapshot inteiro e retenção.
  - **Refs:** R2.3 · design §2 e §9.
- [x] **5.3 Config `google.drive_retention`** (dep: —) — `*int` no YAML (ausente=14 `default`, `0`=manual, positivo=`arquivo`, negativo=erro de config no `Load`); sem env. `config_test`: 4 casos + sem arquivo. `archtest` verde.
  - `yamlDoc.Google.DriveRetention *int` (ponteiro distingue ausente de `0`), `GoogleConfig.DriveRetention int` default 14, origem registrada (`arquivo|default`), sem env. Valor negativo → erro de config claro (`google.drive_retention deve ser >= 0`), boot aborta.
  - Arquivos: `internal/config/config.go`, `internal/config/config_test.go`.
  - **Pronto quando:** `go test ./internal/config/...` cobre ausente (14), `0`, positivo e negativo (erro). `internal/archtest` continua verde (`config` sem PocketBase).
  - **Refs:** R2.4 · design §2.
- [x] **5.4 `drivesync`: `SnapshotStore`, `Observer`, `Status`, `selectToTrash` e `New` novo** (dep: 4.4, 5.3) — `store.go` (File/SnapshotStore/Observer/NopObserver/Status nil-safe), `retention.go` (`selectToTrash` garante o recém-enviado, ignora sem `.enc`), `Syncer` reescrito (mede local via `Attributes().Size` e drive pelo blob cifrado; retenção só após upload ok e `retention>0`; nunca propaga falha de List/Trash; log com `etapa`). Hook testado com `tests.NewTestApp` (backup local real, Drive fake): retenção, Trash/List com erro, retention=0, falha de upload=degradado, sem store=desativado. 25 testes verdes.
  - `internal/drivesync/store.go` com `File`, `SnapshotStore`, `Observer`, `NopObserver`, `Status` (`SetConfigured`, `Set`, `State`, tabela do design §3.1; valor zero = `desativado`); `internal/drivesync/retention.go` com `selectToTrash(files, keep, justUploadedID)` aplicando `strings.HasSuffix(Name, ".enc")` e garantindo o recém-enviado; `New(store, sealer, retention, obs, st)` chama `st.SetConfigured(store != nil)`. O hook mede o backup local (`destino="local"`, tamanho via `Attributes(fileKey) (*blob.Attributes, error)` do `filesystem.System` do PocketBase v0.40.6, campo `.Size int64`: **verificado em 2026-11-05**; plano B: contar os bytes lidos pelo `mirror`), o espelhamento (`destino="drive"`, tamanho do blob cifrado), roda a retenção só após upload ok e `retention > 0`, e nunca propaga falha de `List`/`Trash`. Atualizar o `doc.go` do pacote se descrever o contrato antigo.
  - Arquivos: `internal/drivesync/drivesync.go`, `internal/drivesync/store.go` (novo), `internal/drivesync/retention.go` (novo), `internal/drivesync/drivesync_test.go`, `internal/drivesync/retention_test.go` (novo), `internal/drivesync/doc.go`.
  - **Pronto quando:** `go test ./internal/drivesync/...` cobre: tabela de `selectToTrash` (menos que N, exatamente N, mais que N, recém-enviado fora da listagem, arquivos sem prop/sem `.enc`); upload ok + retenção apaga os excedentes; `Trash` com erro não falha o backup; `retention=0` não lista; tabela de `State()` (zero value, configurado sem tentativa, `erro`, `ok`, `ok` depois de `erro`); `recordingObserver` recebe `Backup`, `Mirror`, `Retention`, `RetentionFailure` nos caminhos certos; pasta obsoleta recria uma vez e segunda falha não faz laço.
  - **Refs:** R2.4, R3.3 · design §2 e §3.1.
- [x] **5.5 Adapter do wiring satisfaz `SnapshotStore`; `Status` único em `Services`; `/readyz` com `checks.drive_dr`** (dep: 5.4) — `driveUploaderAdapter` ganhou `List` (pasta resolvida + `psicoman_kind=snapshot`, converte `google.DriveFile`→`drivesync.File`) e `Trash`; `buildServices` cria um `*drivesync.Status` passado ao seam (`driveSyncWirer` ganhou `obs` e `st`) e a `Services.DriveStatus`; `readyz` ganhou `checks.drive_dr` sem alterar `ready` e omite a chave com `DriveStatus==nil`. Testes `health_drive_test.go` (nil + 3 estados). Observer = `NopObserver` até a Fase 8.
  - `driveUploaderAdapter` passa a implementar `SnapshotStore` e `FolderClient` (`List` com `FolderID` resolvido e `AppProps{"psicoman_kind":"snapshot"}`, converte `google.DriveFile` em `drivesync.File`; `Trash` delega; `Upload` grava `AppProps`). `buildServices` cria um único `*drivesync.Status`, passa a `wireDriveSync` e a `Services.DriveStatus`. `readyz` ganha `checks.drive_dr` (`desativado|ok|degradado`) **sem** alterar `ready`; com `DriveStatus == nil` omite a chave sem panicar. Log `Error` com `backup`, `etapa` (`resolver_pasta`, `upload`, `retencao`) e `erro` sem segredo.
  - Arquivos: `internal/api/wiring.go`, `internal/api/service/service.go`, `internal/api/health.go`, `internal/api/health_test.go`.
  - **Pronto quando:** `go test ./internal/api/...` verde, com `checks.drive_dr` nos três estados e ausência quando `DriveStatus == nil`; `go test ./test/e2e/ -run Health` verde.
  - **Refs:** R3.3, R2.4 · design §3 e §3.1.
- [x] **5.6 Teste em processo da retenção e do `/readyz` do DR** (dep: 5.5) — `drive_dr_test.go`: retenção (lixeira `[b c]`, arquivo sem `.enc` intocado) + `/readyz` ok→degradado (upload falha)→ok (recupera); desativado sem credenciais; 404 persistente não faz laço (exatamente 2 uploads, DR degradado, backup 202).
  - Em `internal/api/drive_dr_test.go` (criado na 4.6): vários backups seguidos com `FakeDriveClient`, `google.drive_retention` pequeno, conferindo `Trashed` e o `checks.drive_dr` (`ok`, depois `degradado` com `UploadErr`/falha forçada).
  - **Pronto quando:** `go test ./internal/api/ -run DriveDR` passa.
  - **Refs:** R2.4, R3.3 · design §3.2.
- [x] **5.7 Docs do Drive DR (R6.2) e `config.example.yaml`** (dep: 5.5) — `architecture.md §0.10` reescrita (snapshot inteiro, ordem de resolução, `drive_dr`, retenção, degradação) + nota em §0.15; `deploy.md` (`drive_folder` opcional, `drive_retention`, nome "Psicoman DR", restauração, `drive_dr` no readyz e no troubleshooting); `config.example.yaml` e `deploy/config.docker.yaml` com `drive_retention`; Swagger do `/readyz` cita `drive_dr` (`go test ./internal/api/swagger/` verde).
  - `docs/architecture.md §0.10` (snapshot inteiro, ordem de resolução da pasta, `drive_dr`, retenção N=14 por lixeira, degradação observável) e a nota da §0.21; `docs/deploy.md`: `google.drive_folder` opcional, `google.drive_retention`, criação automática, nome "Psicoman DR" (renomear/mover é seguro, apagar recria), restauração (baixar `.enc`, decifrar com o `BlobSealer`/chave, descompactar), snapshots anteriores sem `appProperties` fora da retenção. `config.example.yaml` e `deploy/config.docker.yaml` com `drive_retention`. Swagger: `checks.drive_dr` no `/readyz` em `internal/api/swagger/openapi.base.json` (a spec é o JSON embutido; `swagger.go` só a serve), conferindo `contract_test.go`.
  - Arquivos: `docs/architecture.md`, `docs/deploy.md`, `config.example.yaml`, `deploy/config.docker.yaml`, `internal/api/swagger/openapi.base.json`.
  - **Pronto quando:** `go test ./internal/api/swagger/...` verde; os docs descrevem o comportamento real (snapshot inteiro, pasta automática, retenção).
  - **Refs:** R6.2, R2.3, R3.3 · design §2, §3 e §9.
- [x] **Fechamento Fase 5:** `make check` verde; ADRs, steering e docs atualizados; commit `604a82a` `auditoria-profunda Fase 5: oficializa o snapshot inteiro com retenção e degradação observável`. **Ressalva:** a retenção em runtime depende do spike 4.1 (`list`/`update` sob `drive.file`), pendente de credencial do Rafael; fallback documentado.

---

## Fase 6 — R7.1 + R7.2: rate limit e `state` do OAuth

> Dep. de fase: Fase 3 (padrão de testes e ator). As duas partes são independentes entre si.

- [x] **6.1 `httpx.Limiter` (janela fixa, em memória) e `EmailKey`** (dep: —) — `internal/api/httpx/ratelimit.go` (stdlib, sem goroutine; `Allow` recusa por estouro e por saturação chamando `OnReject(scope)`; `Warn` ≤1/min só com a contagem; `EmailKey` = sha256 hex normalizado). 8 testes: limite/estouro, expiração, limpeza, saturação (nova recusada, existente atendida, volta após a janela), `OnReject` 1x por 429 e 0 em ok, Warn 1/min sem vazar a chave, EmailKey.
  - `LimiterOptions{Limit, Window, MaxKeys, Scope, Log, OnReject, Clock}`, `ScopeCadastro`/`ScopeAgendamento`, `Allow` (recusa por estouro **e** por saturação, chamando `OnReject(scope)`; `ok` nunca chama), limpeza sem goroutine, `Warn` de saturação no máximo 1/min com a contagem de chaves, `EmailKey(scope, email)` = sha256 em hex do e-mail normalizado. `OnReject` nulo é no-op (a ligação ao recorder é feita na Fase 8, task 8.6).
  - Arquivos: `internal/api/httpx/ratelimit.go` (novo), `internal/api/httpx/ratelimit_test.go` (novo).
  - **Pronto quando:** `go test ./internal/api/httpx/ -run Limiter` cobre: dentro do limite, estouro, expiração da janela, limpeza, saturação com `maxKeys` pequeno (chave nova → recusada; chave existente dentro do limite → atendida; expirada a janela, a nova volta a ser aceita), `Warn` uma vez por minuto (logger fake), `OnReject` exatamente uma vez por 429 com o `Scope` certo e nenhuma vez em `ok`.
  - **Refs:** R7.1 · design §7.
- [x] **6.2 Aplicar o limite em `register` e em `appointment-requests`** (dep: 6.1) — `portal.Mount` cria um limitador por escopo (`maxLimiterKeys=10000`, `registerLimitPerHour=5`, `appointmentLimitPerHour=10`); register: após `BindBody` e antes da validação de senha (e-mail vazio segue p/ 422); appointment: após `resolvePatient`, chave = hash do `patient_id`; `429 limite_excedido` + `Retry-After`; log só com 8 hex do hash. Swagger declara `429` nas duas rotas (`go test ./internal/api/swagger/` verde).
  - `portal.Mount` cria um limitador por escopo (`maxLimiterKeys = 10000`, `registerLimitPerHour = 5`, `appointmentLimitPerHour = 10` como constantes nomeadas). `POST /v1/portal/register`: logo após `BindBody` e **antes** da validação de senha, chave `EmailKey(ScopeCadastro, body.Email)`; e-mail vazio segue para a validação normal (422); o 429 é decidido antes de qualquer consulta ao banco. `POST /v1/portal/appointment-requests`: após `resolvePatient`, chave `EmailKey(ScopeAgendamento, patientID)`. Resposta `httpx.RespondError(e, 429, "limite_excedido", "Muitas tentativas. Tente novamente mais tarde.", nil)` com `Retry-After`. Nunca guardar nem logar o e-mail (só hash truncado de 8 hex no `Warn` de bloqueio).
  - Arquivos: `internal/api/portal/me.go`, `internal/api/portal/appointments.go`, `internal/api/swagger/openapi.base.json` (declarar `"429"` em `responses` das duas rotas do portal). **Verificado em 2026-11-05:** o `contract_test.go` compara só método+caminho e exige `responses` não vazio; não compara códigos, então basta editar o JSON.
  - **Pronto quando:** `go build ./...` ok; `go test ./internal/api/swagger/...` verde.
  - **Refs:** R7.1 · design §7.
- [x] **6.3 Testes E2E do rate limit (permitido e negado)** (dep: 6.2) — E2E `TestPortalRegisterRateLimit429` (5 variações de caixa/espaço, 6º=429 com `Retry-After` numérico e envelope, corpo sem o e-mail, outro e-mail=201); em processo `TestPortalAgendamentoRateLimit429` (10 ok, 11º=429, outro paciente livre, sem sessão=401).
  - Em `test/e2e/portal_test.go`: 5 cadastros com o mesmo e-mail (variando caixa e espaços), o 6º recebe `429` com envelope e `Retry-After`; outro e-mail segue normal (2xx/4xx); o corpo do 429 não contém o e-mail. Agendamento: teste em processo com paciente aprovado em `internal/api/portal_gate_test.go` (ou novo arquivo na mesma pasta), conferindo 429 após o limite e que outro paciente não é afetado.
  - **Pronto quando:** `go test ./test/e2e/ -run Portal` e `go test ./internal/api/ -run Portal` passam.
  - **Refs:** R7.1 · design §7.
- [x] **6.4 `OAuthStateStore` em memória** (dep: —) — `internal/app/oauth_state.go` (128 bits `crypto/rand` base64url, uso único, TTL 10 min, teto 100: descarta expirados, cheio remove o mais antigo, mutex, relógio injetável); `Services.OAuthState` criado em `buildServices` com seam `oauthStateClock`. 7 testes. `archtest` verde.
  - `Issue()` (16 bytes de `crypto/rand` em base64url), `Consume(state)` uso único com TTL de 10 min, `maxPending = 100` (descarta expirados; se cheio, remove o mais antigo), mutex, relógio injetável. Campo `OAuthState app.OAuthStateStore` em `Services`, criado em `buildServices`.
  - Arquivos: `internal/app/oauth_state.go` (novo), `internal/app/oauth_state_test.go` (novo), `internal/api/service/service.go`, `internal/api/wiring.go`.
  - **Pronto quando:** `go test ./internal/app/ -run OAuthState` cobre emite/consome uma vez, expira, cheio, reuso falha.
  - **Refs:** R7.2 · design §8.
- [x] **6.5 `GoogleAuthorize`/`GoogleCallback` com cookie e validação** (dep: 6.4) — authorize emite state + cookie (`HttpOnly`, `SameSite=Lax`, `Path=/v1/admin/google`, `MaxAge=600`, `Secure` sob TLS/`X-Forwarded-Proto`); callback: lê e expira o cookie em todos os caminhos → `error` do Google (422) → state query==cookie (`subtle.ConstantTimeCompare`) → `Consume` → code → Configured → troca; falha = `400 state_invalido` (mesma mensagem). Swagger documenta o `400 state_invalido`.
  - `GoogleAuthorize` emite o `state` e grava o cookie `psicoman_oauth_state` (`HttpOnly`, `SameSite=Lax`, `Path=/v1/admin/google`, `MaxAge=600`, `Secure` quando `X-Forwarded-Proto=https` ou TLS). `GoogleCallback`, nesta ordem: `error` do Google (422, limpa o cookie) → `state` da query vs cookie (`subtle.ConstantTimeCompare`; ausente ou diferente → `400 state_invalido`) → `Consume` falso → `400 state_invalido` (mesma mensagem) → `code`, `Configured()` e troca. Cookie expirado (`MaxAge=-1`) em todos os caminhos. Mensagem: "Estado da autorização inválido ou expirado. Inicie a autorização novamente." Documentar `400 state_invalido` no Swagger (`openapi.base.json`).
  - Arquivos: `internal/api/admin/google_oauth.go`, `internal/api/swagger/openapi.base.json`.
  - **Pronto quando:** `go build ./...` ok; testes da 6.6 passam.
  - **Refs:** R7.2 · design §8.
- [x] **6.6 Atualizar e ampliar os testes do callback** (dep: 6.5) — `apiResp` ganhou `header`; `google_oauth_test.go` e `health_test.go` passam pelo fluxo authorize→cookie→callback (`autorizarGoogle`); novos: atributos do cookie (+Secure via proxy), negados (sem cookie, divergente, sem state, reuso, expirado via `oauthStateClock`) com 400 e cookie expirado, `error` do Google expira o cookie. E2E real `oauth_state_test.go` (config temporário com client_id fictício via `PSICOMAN_CONFIG`; authorize 302+cookie; callbacks inválidos=400 antes da troca). Sem `t.Skip`.
  - Os testes existentes `internal/api/google_oauth_test.go` (~l.80, `state=abc`) e `internal/api/health_test.go` (~l.60, `state=s`) chamam o callback sem passar pelo authorize e **quebram**: ajustá-los para o fluxo authorize → `Set-Cookie` e `state` da `Location` → callback com `Cookie`. Novos casos: sem cookie, state diferente, state reusado (2º callback → 400), state expirado (relógio do store). E2E real em `test/e2e/` (ex.: novo `test/e2e/oauth_state_test.go`): authorize (302 + cookie), callback sem state → `400` (falha antes da troca; o Google real não é tocado).
  - **Pronto quando:** `go test ./internal/api/... ./test/e2e/...` passam; sem `t.Skip`.
  - **Refs:** R7.2 · design §8.
- [x] **6.7 Docs e ADR da fase** (dep: 6.3, 6.6) — 2 ADRs em `docs/decisions.md` (state do OAuth; rate limit em duas camadas, com saturação por botnet como risco aceito e 409/login nativo como backlog); `deploy.md` (duas camadas, mesmo navegador, `X-Forwarded-Proto`); `architecture.md §0.23`. Steering de segurança não muda.
  - ADR "state do OAuth validado" e ADR do rate limit em duas camadas (nginx por IP, app por e-mail/paciente; saturação por botnet como risco aceito; enumeração por `409` e login nativo sem limite como achados de backlog) em `docs/decisions.md`. `docs/deploy.md`: duas camadas, callback no mesmo navegador do authorize, o proxy/Pangolin deve repassar `X-Forwarded-Proto`. `docs/architecture.md`. Steering `psicoman-seguranca-lgpd.md` **não muda** (já pede as duas camadas).
  - Arquivos: `docs/decisions.md`, `docs/deploy.md`, `docs/architecture.md`.
  - **Pronto quando:** os três documentos descrevem as duas camadas e o fluxo do `state`.
  - **Refs:** R7.1, R7.2 · design §7 e §8.
- [x] **Fechamento Fase 6:** `make check` verde; Swagger e docs atualizados; commit `dea5247` `auditoria-profunda Fase 6: limita tentativas por e-mail e valida o state do OAuth`.

---

## Fase 7 — R8 base: dependência, registry, middleware HTTP e listener

> Dep. de fase: Fase 1. Spikes (7.1 a 7.3) antes de qualquer código que dependa deles. Entrega o que o E2E de isolamento, HTTP, 401/403 e vazamento exige.

- [x] **7.1 SPIKE: `Request.Pattern` e `countingWriter` no mux do PocketBase** (dep: —) — **reconfirmado em runtime** (sem plano B): `TestMetrics_HTTPSegurancaEPatterns` (mux real em processo) mostra `GET /v1/app/patients/{id}` com `{id}` (nunca o ULID), catch-all `/` = `nao_encontrada`, e o 401 de `RequirePangolinIdentity` já com o Pattern; `TestCountingWriter_ConservaTrackersEFlush` prova `Status()/Written()` via `Unwrap` e `Flush` (SSE) via `http.ResponseController`. O erro devolvido por `Next()` é escrito no writer ORIGINAL pelo PB (lido em `router.go`), então com `err != nil` o tamanho fica desconhecido (-1). O Dashboard `/_/` cai em `infra` por prefixo (sem cardinalidade).
  - Teste descartável (ou definitivo) em `internal/api/httpx/` que monta o router do PB v0.40.6, registra um middleware raiz e confirma: `GET /v1/app/patients/<ulid>` chega com `Pattern == "GET /v1/app/patients/{id}"`; rota inexistente chega com `Pattern == "/"`; um `countingWriter` que embute o `ResponseWriter` e expõe `Unwrap()` mantém `Status()`/`Written()`/flush reconhecidos (`StatusTracker`, SSE de `/api/realtime`). Anotar o resultado; se o `countingWriter` falhar, adotar o **plano B** do design §10.4 (sem contar bytes; usar `Content-Length` quando presente).
  - **Pré-verificado em 2026-11-05 (runtime, teste descartável já removido):** `GET /v1/app/patients/<ulid>` chega com `Pattern == "GET /v1/app/patients/{id}"` (com **prefixo de método**; o 401 do `RequirePangolinIdentity` também chega com o Pattern, ou seja, está populado antes dos middlewares do grupo); rota inexistente e método sem rota chegam com `Pattern == "/"` e `e.Status() == 0` (erro ainda não escrito, status vem de `router.ToApiError`); `/v1/health` → `GET /v1/health`, `/api/health` → `GET /api/health`. O `countingWriter` com `Unwrap()` é reconhecido por `getStatus`/`getWritten` (leitura de `tools/router/router.go`), **ainda sem teste de runtime**.
  - **Pronto quando (reconfirmar rapidamente):** os valores acima se repetem num teste rápido (que pode virar o teste definitivo de `RouteLabels`/`Instrument`), o `countingWriter` é exercitado em runtime (`Status()`, `Written()`, `FlushError`/SSE) e o plano B está decidido e anotado só se algo divergir; incluir `/_/` no binário real para ver o Pattern do Dashboard.
  - **Refs:** R8.2, R8.9.2 · design §10.4 e §11.5 item 3.
- [x] **7.2 SPIKE: `client_golang@v1.25.0` e `go mod tidy`** (dep: —) — `go get ...@v1.25.0` + `go mod tidy` + `go build ./...` ok; `go.mod` só acrescentou linhas (`client_golang v1.25.0` + `beorn7/perks`, `cespare/xxhash/v2`, `munnerz/goautoneg`, `client_model v0.6.3`, `common v0.72.0`, `procfs v0.22.0`, `protobuf v1.36.12`); **nenhuma versão do PocketBase mudou**; `go.sum` só trocou 6 entradas obsoletas de deps de teste. `govulncheck` **não instalado** (registrado). `go test ./...` roda no gate.
  - `go get github.com/prometheus/client_golang@v1.25.0 && go mod tidy`, depois `go build ./...`, `go test ./... > /tmp/test-spike.log 2>&1`, e conferir o diff do `go.sum`. Se o `tidy` rebaixar versões que o PocketBase v0.40.6 fixa, **parar e reportar**. Rodar `govulncheck` se disponível; registrar a ausência.
  - Arquivos: `go.mod`, `go.sum`.
  - **Pré-verificado em 2026-11-05:** `v1.25.0` existe; módulo temporário com `go 1.27` passa `go build` e `go vet`; numa cópia do repositório (sem `.git`), `go get` + `go mod tidy` + `go build ./...` saem 0 e o `go.mod` só ganha linhas (`client_golang v1.25.0` e as indiretas `beorn7/perks`, `cespare/xxhash/v2`, `munnerz/goautoneg`, `client_model v0.6.3`, `common v0.72.0`, `procfs v0.22.0`, `protobuf v1.36.12`); nenhuma versão do PocketBase mudou. `go test ./...` não foi rodado na cópia.
  - **Pronto quando (reconfirmar rapidamente):** `go build ./...` e `go test ./...` verdes com a dependência fixada (versão exata) e o diff de versões sem rebaixar o PocketBase (esperado: só acréscimos).
  - **Refs:** R8.11 · design §10.7 e §11.5 item 4.
- [x] **7.3 Teste de arquitetura: `domain`/`app`/`config` sem Prometheus** (dep: 7.2) — `TestDomainAppConfigNaoImportamPrometheus` em `archtest` (proíbe `github.com/prometheus/` nos três). Verde.
  - `TestDomainAppConfigNaoImportamPrometheus`, no padrão dos três testes existentes, proibindo `github.com/prometheus/` nesses pacotes.
  - Arquivo: `internal/archtest/arch_test.go`.
  - **Pronto quando:** `go test ./internal/archtest/...` passa.
  - **Refs:** R8.11 · design §10.1.
- [x] **7.4 Porta `app.Metrics`, `NopMetrics` e `QueueStats`** (dep: 7.3) — `internal/app/metrics.go` (enums fechados `PatientEvent`/`SessionEvent`/`ApptEvent`/`Result`/`CalendarOp`/`CipherOp`; interface do design §10.2; `NopMetrics`; `QueueStats`). Testes: `NopMetrics` não panica e os valores de enum são `[a-z_]+` (nunca identificadores).
  - Tipos fechados (`PatientEvent`, `SessionEvent`, `ApptEvent`, `Result`, `CalendarOp`, `CipherOp`) e a interface do design §10.2/§10.9, sem importar PocketBase nem Prometheus. O campo de `Services` se chama `Obs` (já existe `Metrics *app.SessionMetricsService`, em `internal/app/metrics_service.go`; o arquivo novo é `metrics.go`, sem colisão).
  - Arquivos: `internal/app/metrics.go` (novo), `internal/app/metrics_test.go` (novo; `NopMetrics` não panica).
  - **Pronto quando:** `go test ./internal/app/... ./internal/archtest/...` verdes.
  - **Refs:** R8.4, R8.11 · design §10.2 e §10.9.
- [x] **7.5 `platform/metrics`: registry próprio, coletores e famílias HTTP/segurança/runtime** (dep: 7.4) — `Obs` (`New(Config)`, `Handler()`, `App()`, `HTTP()`, `Drive()` stub); registry próprio; `NewGoCollector`/`NewProcessCollector`; `build_info`, `uptime_segundos`, `identidade_dev_ativa`; `http_requisicoes_total`, `http_duracao_segundos`, `http_em_andamento`, `http_resposta_bytes`, `seguranca_negados_total`, `rate_limit_rejeicoes_total` com os buckets do design. `HTTP()`/`Drive()` devolvem tipos concretos que satisfazem as interfaces estruturalmente (sem importar httpx/drivesync). Testes por scrape do `Handler()`.
  - `New(Config)`, `Handler()`, `App()`, `HTTP()`, `Drive()` (stub com `NopObserver` até a Fase 8); registry próprio com `NewGoCollector`, `NewProcessCollector`, `build_info`, `uptime_segundos`, `identidade_dev_ativa`; famílias `http_*`, `seguranca_negados_total`, `rate_limit_rejeicoes_total` com buckets do design §10.3. Rótulos só de enums fechados.
  - Arquivos: `internal/platform/metrics/metrics.go` (novo), `internal/platform/metrics/http.go` (novo), `internal/platform/metrics/metrics_test.go` (novo).
  - **Pronto quando:** `go test ./internal/platform/metrics/...` cobre cada família com o rótulo esperado e `NopMetrics` sem panic.
  - **Refs:** R8.2, R8.3, R8.8 · design §10.1 e §10.3.
- [x] **7.6 `httpx`: `HTTPRecorder`, `RouteLabels`, `Instrument`, `SetDenyReason`/`DenyReason`** (dep: 7.1, 7.5) — `httpx/metrics.go`; middleware ligado em `api.go` com `e.Router.Bind(&hook.Handler{Priority:-5000})` antes de montar `/v1`; `SetDenyReason` em `RequirePangolinIdentity` (`sem_identidade`), gate do portal (`sem_sessao`/`nao_aprovado`) e `resolvePatient` (`sem_sessao`); recover pós-`Next()` que nunca altera o `err`; `Response` original restaurado. Testes: `RouteLabels` em tabela, contagem+bytes, 401/403 com motivo (`outro`/`pocketbase` sem motivo), recorder que panica não altera a resposta.
  - Middleware raiz registrado com `e.Router.Bind(&hook.Handler[*core.RequestEvent]{Func: httpx.Instrument(rec), Priority: -5000})` em `internal/api/api.go`, antes de montar `/v1`. `RouteLabels` (remove o prefixo de método do `Pattern`, p. ex. `GET /v1/...`; ator por prefixo; `/` → `nao_encontrada`; vazio → `desconhecida`, nunca o `URL.Path`); status via `err` quando `Next()` devolve erro (`router.ToApiError`); `http_em_andamento` por prefixo fixo; `recover` pós-`Next()` que nunca altera o `err` (R8.10). `SetDenyReason` chamado em `RequirePangolinIdentity` (`sem_identidade`), `portal.RequireApproved` em `internal/api/portal/gate.go` (`sem_sessao`/`nao_aprovado`) e em `resolvePatient`. Aplicar o plano B do spike 7.1 se for o caso.
  - Arquivos: `internal/api/httpx/metrics.go` (novo), `internal/api/httpx/metrics_test.go` (novo), `internal/api/httpx/auth.go`, `internal/api/portal/gate.go`, `internal/api/portal/me.go`, `internal/api/api.go`.
  - **Pronto quando:** `go test ./internal/api/httpx/...` cobre: `RouteLabels` em tabela (padrões de cada ator, `/`, vazio, nunca ULID); contagem; status via `err` (404); `denyReason`; recorder que panica não altera a resposta.
  - **Refs:** R8.2, R8.3, R8.9.2, R8.10 · design §10.4 e §10.9.
- [x] **7.7 Config `metrics.addr` e `StartMetricsServer` (fail-closed)** (dep: 7.5) — `MetricsConfig{Addr}`, `EnvMetricsAddr`, env > arquivo com origem, validação `host:porta` 1..65535 (9 casos); `StartMetricsServer` com `listen` injetável (addr vazio não chama `listen`), mux só com `GET /metrics`, timeouts, bind síncrono (falha aborta o boot), `Shutdown` em `OnTerminate`, `Warn` para `0.0.0.0`/`::`/vazio; `Services.Obs` criado SEMPRE em `buildServices`; `obs.Drive()` já ligado ao `driveSyncWirer`. Testes em processo: exposição, fail-closed, falha de bind.
  - `MetricsConfig{Addr}`, `yamlDoc.Metrics.Addr`, `EnvMetricsAddr = "PSICOMAN_METRICS_ADDR"`, precedência env > arquivo com origem logada; validação (`net.SplitHostPort`, porta 1..65535) com erro de config no boot. `StartMetricsServer` com `listen` injetável (`var listen = net.Listen`), `http.Server` com timeouts, mux próprio com **só** `GET /metrics` → `promhttp.HandlerFor(reg, ContinueOnError, Timeout 10s, MaxRequestsInFlight 4)`, `net.Listen` síncrono (falha de bind aborta o boot), `Shutdown` em `OnTerminate`, `Warn` para bind em `0.0.0.0`/`::`/vazio. `addr == ""` retorna sem chamar `listen`. `buildServices` cria `Services.Obs` **sempre**; `Obs.App()` fica disponível para `SetMetrics`. O listener não é instrumentado por si mesmo.
  - Arquivos: `internal/config/config.go`, `internal/config/config_test.go`, `internal/api/metricsserver.go` (novo), `internal/api/metricsserver_test.go` (novo), `internal/api/service/service.go`, `internal/api/wiring.go`, `internal/api/api.go`.
  - **Pronto quando:** `go test ./internal/config/... ./internal/api/...` cobrem: config válida/vazia/inválida/env sobrescreve arquivo; com `Addr == ""` nenhum `listen` é chamado; com `Addr` válido o `GET /metrics` responde em formato Prometheus e o resto 404.
  - **Refs:** R8.1, R8.9.3 · design §10.5 e §10.9.
- [x] **7.8 E2E de isolamento, HTTP, 401/403 e vazamento** (dep: 7.6, 7.7) — `test/e2e/metrics_test.go` (`startServerWithMetrics`): itens 1 a 6 do design (exposição Prometheus; tráfego real com `rota` `{id}`/`nao_encontrada` e `motivo` `sem_identidade`/`sem_sessao`/`nao_aprovado` incluindo o 403 real do portal; vazamento com sentinelas + enums de `ator`/`metodo`/`status`; isolamento API×listener; fail-closed). Espelhado em processo (`internal/api/metrics_test.go`). O item 7 (backup local) migra para a **8.5**, pois as famílias de backup só existem na Fase 8. Sem `t.Skip`.
  - `test/e2e/metrics_test.go` (novo) com helper `startServerWithMetrics` (porta extra via `freePort` e `PSICOMAN_METRICS_ADDR`). Casos do design §10.8: (1) `GET <metricsBase>/metrics` 200, texto Prometheus, contém `go_goroutines` e `psicoman_build_info`; (2) tráfego real (livez 200, `/v1/app/patients` sem header 401, `/v1/portal/sessions` sem sessão 401, `/v1/nao-existe` 404, `/v1/app/patients/<ulid>` com header 404) conferindo `http_requisicoes_total` com `rota="/v1/app/patients/{id}"`, `seguranca_negados_total{motivo="sem_identidade"}` e `rota="nao_encontrada"`; (3) 403 do portal: paciente **não aprovado** com sessão real em `GET /v1/portal/appointment-requests` → `motivo="nao_aprovado"` (permitido e negado juntos); (4) **vazamento:** sentinelas de e-mail/CPF/nome/telefone/nota clínica, o ULID real, o valor de `X-Pangolin-User` e fragmento da chave de cifra **não** aparecem em `/metrics`; valores de rótulos `ator`, `metodo`, `status` restritos aos enums; (5) **isolamento:** `<base>/metrics` e `<base>/v1/metrics` → 404 na API; `<metricsBase>/v1/livez` → 404; (6) **fail-closed:** reserva e fecha uma porta, sobe sem `PSICOMAN_METRICS_ADDR`, `net.Dial` recusado (limitação do flake comentada no teste); (7) `POST /v1/admin/backup` e `backup_total{destino="local",resultado="ok"}` + timestamp > 0 (depende do `Observer` da Fase 5; se a ligação só ocorrer na Fase 8, este item migra para a task 8.5).
  - **Pronto quando:** `go test ./test/e2e/ -run Metrics` passa (itens 1 a 6); sem `t.Skip`.
  - **Refs:** R8.1, R8.2, R8.3, R8.9.1 a R8.9.4 · design §10.8.
- [x] **7.9 ADR do listener (R8.12) e docs da base** (dep: 7.7) — ADR "Fase 7" (encerra o adiamento de 2026-10-24; reconcilia a Opção A; `client_golang` v1.25.0; listener fail-closed; rótulos proibidos; mTLS como evolução); `architecture.md` §0.24 + nota na §0.21; `deploy.md` (seção Métricas + política de rede); `stack-local.md`; `config.example.yaml` e `deploy/config.docker.yaml` (`metrics.addr`); `docker-compose.yml` e `Dockerfile` com 9091 só em `expose`/`EXPOSE` (validado por parser YAML + `docker compose config`: nenhum `ports:` para 9091).
  - ADR novo em `docs/decisions.md` que (a) encerra o adiamento de métricas de 2026-10-24, (b) referencia e reconcilia o ADR "Segregação de exposição na borda, Opção A" (o listener de métricas não segrega atores nem serve a API), (c) registra `client_golang` v1.25.0, listener dedicado fail-closed, rótulos proibidos e a política de rede (mTLS como evolução futura, fora de escopo). `docs/architecture.md`: nova seção de observabilidade e ajuste de §0.21 (o `cmd/psicoman` segue em `127.0.0.1:8090`; o listener de métricas é um segundo endereço interno, só se configurado). `docs/stack-local.md`: porta de métricas só em `expose`, nunca `ports`. `config.example.yaml`, `deploy/config.docker.yaml` (`metrics.addr: "0.0.0.0:9091"` comentado), `docker-compose.yml` e `Dockerfile` (`expose`/`EXPOSE 9091`, documentais).
  - Arquivos: `docs/decisions.md`, `docs/architecture.md`, `docs/stack-local.md`, `config.example.yaml`, `deploy/config.docker.yaml`, `docker-compose.yml`, `Dockerfile`.
  - **Pronto quando:** o ADR cita o ADR de 2026-10-24 e o fim do adiamento; `docker compose config` (ou leitura do compose) mostra `expose` e nenhum `ports:` para 9091.
  - **Refs:** R8.12, R8.9.3 · design §10.5 e §10.6.
- [x] **Fechamento Fase 7:** `make check` verde; `/metrics` e a ausência dele na porta da API provados por E2E; commit `b2cab83` `auditoria-profunda Fase 7: expõe métricas HTTP em listener interno dedicado`.

---

## Fase 8 — R8 famílias: negócio, integrações, auditoria e backup

> Dep. de fase: Fase 7 (base), Fase 3 (audit), Fases 4 e 5 (Drive e retenção), Fase 6 (limitador). Cada serviço ganha `SetMetrics` com default `NopMetrics{}`, então os testes existentes não mudam.

- [x] **8.1 Instrumentação de negócio nos serviços** (dep: 7.4) — `SetMetrics` (default `NopMetrics`, `nil` volta ao no-op) em `PatientService`, `SessionService`, `AppointmentService`, `BillingService`, `PaymentService`, `InvoiceService`; eventos só no sucesso e só na transição real (aprovar/recusar idempotente não conta; `Schedule` não tem evento próprio). Fiação em `buildServices` com `obs.App()`. Testes (`metrics_services_test.go`, fake `recMetrics`): cadastro admin/portal novo/vínculo, aprovação idempotente, recusa, ciclo da sessão (criada/confirmada/finalizada/cancelada/falta), falhas não contam, calendário degradado, pedido (criado/confirmado/recusado), débito idempotente, pagamento, PDF novo+reaproveitado.
  - `SetMetrics(Metrics)` em `PatientService` (cadastro/aprovação/recusa), `SessionService` (criada/confirmada/cancelada/finalizada/falta), `AppointmentService` (criado/confirmado/recusado), `BillingService` (débito gerado), `PaymentService` (pagamento), `InvoiceService` (PDF). Eventos contados só no caminho de sucesso. Fiação em `buildServices` com `obs.App()`.
  - Arquivos: `internal/app/patient_service.go`, `internal/app/session_service.go`, `internal/app/appointment_service.go`, `internal/app/billing_service.go`, `internal/app/payment_service.go`, `internal/app/invoice_service.go`, `internal/api/wiring.go`; testes: `internal/app/patient_service_test.go`, `session_service_test.go`, `appointment_service_test.go`, `billing_service_test.go`, `payment_service_test.go`, `invoice_service_test.go` e um fake `Metrics` em `internal/app/fakes_test.go`.
  - **Pronto quando:** `go test ./internal/app/...` cobre, com `Metrics` fake, os eventos certos nos caminhos ok e erro de cada serviço.
  - **Refs:** R8.4 · design §10.2.
- [x] **8.2 Famílias de negócio e gauges de fila (`QueueStats`)** (dep: 8.1) — `platform/metrics/business.go` (`pacientes/sessoes/agendamentos_eventos_total`, `debitos_gerados`, `pagamentos_registrados`, `pdfs_gerados`); `queues.go` (coletor custom: timeout 2 s, cache 15 s, erro omite a amostra e conta `metrics_coleta_erros_total{fonte}` — emitido pelo próprio coletor para refletir no mesmo scrape); `pb.NewQueueStats` com `CountRecords` (`patient.approval_status=pendente`, `appointment_request.status=pendente`). `Obs.App()` agora é o recorder real. Testes: famílias, cache com relógio fake, erro omite+conta, fila real no SQLite.
  - Famílias `pacientes_eventos_total`, `sessoes_eventos_total`, `agendamentos_eventos_total`, `debitos_gerados_total`, `pagamentos_registrados_total`, `pdfs_gerados_total` em `platform/metrics`. `pb.NewQueueStats(a)` com `CountRecords("patient", dbx.HashExp{"approval_status":"pendente"})` e `CountRecords("appointment_request", dbx.HashExp{"status":"pendente"})` (nomes verificados em `0010_patient_scheduling.go`); `Collector` custom com `context.WithTimeout(2s)`, cache de 15 s, omissão da amostra em erro e `metrics_coleta_erros_total{fonte}`.
  - Arquivos: `internal/platform/metrics/business.go` (novo), `internal/platform/metrics/queues.go` (novo), `internal/platform/metrics/business_test.go` (novo), `internal/platform/pb/queue_stats.go` (novo), `internal/platform/pb/queue_stats_test.go` (novo), `internal/api/wiring.go`.
  - **Pronto quando:** `go test ./internal/platform/...` cobre a família com o rótulo esperado, a fonte que erra omitindo a amostra e contando o erro, e o cache de 15 s com relógio fake.
  - **Refs:** R8.4 · design §10.2 e §10.3.
- [x] **8.3 Decoradores de Calendar, Gemini e cifra; `CalendarDegraded`; OAuth e `gemini_ligada`** (dep: 7.5) — `decorators.go` (`WrapCipher`, `WrapAI` — `nil` continua `nil`, `WrapConflictChecker` com `ok|conflito|erro`, `WrapSessionCalendar`); aplicados em `buildServices` (cifra decorada; `SessionService.WrapCalendar` sobre o que o seam injetou, sem mudar a assinatura de `calendarWirer`; `NewAIService(WrapAI(aiSummarizerBuilder(a)))`); `CalendarDegraded()` no ramo `s.calendar == nil`; `google_oauth_ok` (mesmo critério do `/readyz`) e `gemini_ligada`. Variável local `metrics` renomeada p/ `sessionMetrics` (colidia com o pacote). Testes: valor/erro preservados, latência medida, opt-in da IA preservado; em processo com fakes nos seams.
  - `WrapCipher`, `WrapAI`, `WrapConflictChecker` (sobre `app.ConflictChecker`, op `freebusy`) e `WrapSessionCalendar` (sobre `app.SessionCalendar`, op `create_event`) aplicados no wiring (**verificado em 2026-11-05:** não existe `CalendarGateway`); famílias `calendar_*`, `gemini_*`, `cifra_operacoes_total`, `google_oauth_ok` (via `GoogleAuth.Status`, mesmo critério do `/readyz`), `gemini_ligada`. `SessionService` chama `CalendarDegraded()` no ramo `if s.calendar == nil` de `createCalendarEvent` (`session_service.go:293`, **verificado em 2026-11-05**; chamado por `Create` e `Schedule`), via `SetMetrics`. Rótulos sem segredo e sem texto livre.
  - Arquivos: `internal/platform/metrics/decorators.go` (novo), `internal/platform/metrics/decorators_test.go` (novo), `internal/app/session_service.go`, `internal/api/wiring.go`, `internal/api/calendar_test.go`.
  - **Pronto quando:** `go test ./internal/platform/metrics/... ./internal/api/...` cobrem: cada decorador conta ok/erro/conflito e latência; `google_oauth_ok` coerente com `/readyz`; Calendar/Gemini com os fakes reatribuídos nos seams `calendarWirer` e `aiSummarizerBuilder`.
  - **Refs:** R8.5, R8.7, R8.8 · design §10.2, §10.3 e §10.9.
- [x] **8.4 Auditoria: `AuditService.SetMetrics`, `audit_log_total`, `audit_log_falhas_total`** (dep: 3.2, 7.4) — `SetMetrics`; `Record` conta sucesso e falha com a constante de ação; ação fora do enum vira `outro` no rótulo. Testes: sucesso e falha contam a ação certa; `nil` volta ao no-op.
  - `SetMetrics(Metrics)` em `AuditService`; `Record` chama `AuditRecorded(action)` em sucesso e `AuditFailed(action)` em falha; `action` sempre constante de `domain.AuditAction*`. Famílias em `platform/metrics`. Conecta a lacuna deixada na task 3.2.
  - Arquivos: `internal/app/audit_service.go`, `internal/app/audit_service_test.go`, `internal/platform/metrics/business.go`, `internal/api/wiring.go`.
  - **Pronto quando:** `go test ./internal/app/ ./internal/platform/metrics/` cobre sucesso e falha de gravação contando a ação certa.
  - **Refs:** R8.7, R1 · design §1 e §10.3.
- [x] **8.5 Backup e Drive: adapter `Obs.Drive()` ligado ao `Syncer`** (dep: 5.4, 7.5) — `DriveMetrics` real (`backup_total`, `backup_duracao_segundos`, `backup_tamanho_bytes`, `backup_ultimo_sucesso_timestamp_segundos{destino}` com série pré-criada em 0, `drive_espelhamento_total`, `drive_dr_pronto` derivado, `drive_snapshots_mantidos` ausente/removido quando a listagem falha, `drive_retencao_falhas_total{etapa}`); rótulos fora do enum viram `outro`. Já ligado ao `driveSyncWirer` desde a Fase 7. Item 7 do E2E de métricas migrado para cá: `TestMetricsBackupLocal` (backup real: `local ok`, tamanho e timestamp > 0, espelhamento `desativado`, sem destino drive). Em processo (`TestMetricsFamilias_BackupEDrive`): retenção, gauge some com `ListErr`, `dr_pronto` cai com `UploadErr`.
  - Implementar `drivesync.Observer` em `platform/metrics` (`backup_total`, `backup_duracao_segundos`, `backup_tamanho_bytes`, `backup_ultimo_sucesso_timestamp_segundos{destino}`, `drive_espelhamento_total`, `drive_dr_pronto`, `drive_snapshots_mantidos`, `drive_retencao_falhas_total`); `drive_dr_pronto` e o timestamp são **derivados** no adapter (sem acrescentar métodos ao `Observer`, regra do design §3.1). `wireDriveSync` recebe `obs.Drive()`. Incluir aqui o caso 7 do E2E de métricas (backup local) em `test/e2e/metrics_test.go`.
  - Arquivos: `internal/platform/metrics/drive.go` (novo), `internal/platform/metrics/drive_test.go` (novo), `internal/api/wiring.go`, `test/e2e/metrics_test.go`.
  - **Pronto quando:** `go test ./internal/platform/metrics/... ./internal/api/... ./test/e2e/ -run 'Metrics|DriveDR'` passam; `snapshots_mantidos` ausente quando a listagem falha.
  - **Refs:** R8.6 · design §3.1 e §10.3.
- [x] **8.6 Rate limit ligado ao recorder** (dep: 6.2, 7.6) — `portal.Mount` liga `OnReject` a `svc.Obs.HTTP().RateLimited` (nil-safe p/ `Services` montado à mão). Teste em processo: 6º cadastro = 429 => `rate_limit_rejeicoes_total{escopo="cadastro"} 1`.
  - `portal.Mount` recebe o `HTTPRecorder` (ou `RateLimitHooks`) e monta os limitadores com `OnReject: obs.RateLimited`; família `rate_limit_rejeicoes_total{escopo}`.
  - Arquivos: `internal/api/portal/me.go`, `internal/api/wiring.go`, `internal/api/api.go`, `internal/platform/metrics/http.go`.
  - **Pronto quando:** teste em processo com 429 por estouro e por saturação incrementando `rate_limit_rejeicoes_total{escopo}`; `go test ./internal/api/...` verde.
  - **Refs:** R8.3, R7.1 · design §7 e §10.3.
- [x] **8.7 Testes em processo e E2E de vazamento ampliado** (dep: 8.1 a 8.6) — `internal/api/metrics_families_test.go`: negócio+auditoria+cifra+filas+degradado com tráfego real; Calendar com fakes (ok e conflito) e sem Google; Gemini ligada/desligada (api_key sentinela não vaza); Backup/Drive; rate limit; `google_oauth_ok` coerente com o `/readyz` (client_secret, client_id e refresh token sentinelas não vazam). `identidade_dev_ativa` e o vazamento base já cobertos na Fase 7 (em processo e E2E).
  - Gauge `identidade_dev_ativa` com o seam `configLoader` devolvendo `DevIdentity`; ampliar o caso de vazamento da task 7.8 para cobrir as novas famílias (sentinelas de segredo: client secret, refresh token, api key do Gemini, chave de cifra) e conferir que os rótulos novos usam só enums fechados.
  - Arquivos: `internal/api/metrics_test.go` (novo), `test/e2e/metrics_test.go`.
  - **Pronto quando:** `go test ./internal/api/ -run Metrics` e `go test ./test/e2e/ -run Metrics` passam.
  - **Refs:** R8.9.1, R8.9.4, R8.3 · design §10.8.
- [x] **8.8 Docs de R8: nomes, coleta e alertas** (dep: 8.7) — `deploy.md` (tabela das famílias, como coletar, 8 alertas sugeridos, §8 sem "sem endpoint de métricas"); `architecture.md` (§0.24 com as famílias; §0.15 corrigida); `requirements.md` (§3.8 snapshot inteiro/pasta automática/retenção/`drive_dr`; §4.1 rastreabilidade LGPD, rate limit, `state`; §4.3 métricas entregues); `README.md`; ADR da Fase 8 em `decisions.md` (fecha OBS-1 e R5.2 da `pocketbase-finish`). `/metrics` fica fora de `/v1` e do Swagger.
  - `docs/deploy.md`: `metrics.addr`, tabela de famílias, como coletar, política de rede (firewall só do IP do coletor; TLS/mTLS como evolução), alertas sugeridos do design §10.6; `docs/architecture.md`; `docs/requirements.md` (requisito não funcional de observabilidade); `docs/decisions.md` (nota de fechamento do OBS-1 e do R5.2 da pocketbase-finish). `/metrics` fica fora de `/v1` e do Swagger.
  - Arquivos: `docs/deploy.md`, `docs/architecture.md`, `docs/requirements.md`, `docs/decisions.md`.
  - **Pronto quando:** os documentos listam as famílias e os alertas; `go test ./internal/api/swagger/...` continua verde.
  - **Refs:** R8, R10 · design §10.6 e §10.8.
- [x] **Fechamento Fase 8:** `make check` verde, incluindo `go test -race ./internal/...`; docs atualizados; commit `e6d8e89` `auditoria-profunda Fase 8: instrumenta negócio, integrações, auditoria e backup`.

---

## Fase 9 — R6 + R7.3: docs, steering e tabela da pocketbase-finish

> Dep. de fase: Fases 5 e 7 (os docs de Drive e métricas já foram escritos nelas; esta fase consolida e fecha as pendências documentais).

- [x] **9.1 Steering sem Gmail (R6.1)** (dep: 5.2) — `psicoman-google-api.md`: título `(Calendar, Meet, Drive)`, escopo `gmail.send` removido, seção Gmail vira "Envio de e-mail" só com "Gmail: fora de escopo", nota de IA e Testes ajustados (clientes em `internal/integration/google`, portas em `internal/app`, sem `GmailClient`); `psicoman-golang.md` (Calendar/Drive); `psicoman-testes-e2e.md` (binário único `cmd/psicoman`, sem `GmailClient`, fakes em processo). `grep -ri gmail .kiro/steering/` só devolve a linha "fora de escopo".
  - `psicoman-google-api.md`: título `Integração Google (Calendar, Meet, Drive)`; remover o escopo `gmail.send` e a seção `## Gmail`; ajustar a nota de IA ("Calendar/Gmail/Drive" → "Calendar/Drive") e a seção Testes (remover `GmailClient`; as interfaces vivem em `internal/integration/google`, não em `internal/app`/`internal/domain`); acrescentar a linha "Gmail: fora de escopo (ver `docs/requirements.md` §3.7/§5)". `psicoman-golang.md` (linha 33, "Calendar/Gmail/Drive"). `psicoman-testes-e2e.md` (linha 17: remover `GmailClient` e trocar o modelo de dois binários "`admin` ou `portal`" pelo binário único `cmd/psicoman`).
  - Arquivos: `.kiro/steering/psicoman-google-api.md`, `.kiro/steering/psicoman-golang.md`, `.kiro/steering/psicoman-testes-e2e.md`.
  - **Pronto quando:** `grep -ri gmail .kiro/steering/` devolve só a linha "fora de escopo".
  - **Refs:** R6.1 · design §9.
- [x] **9.2 Steering de testes: regra do E2E de métricas e de rate limit** (dep: 7.8, 6.3) — seção "Métricas (Prometheus) e rate limit" em `psicoman-testes-e2e.md`: listener dedicado, isolamento, não-vazamento, permitido+negado; regra de 429; fluxo OAuth authorize→cookie→callback. Cita `test/e2e/metrics_test.go`.
  - Acrescentar em `psicoman-testes-e2e.md` a regra do E2E de métricas (listener dedicado, isolamento, vazamento) e a do E2E de 429, conforme o design §9 (R6.1).
  - Arquivo: `.kiro/steering/psicoman-testes-e2e.md`.
  - **Pronto quando:** o steering descreve os dois casos e cita `test/e2e/metrics_test.go`.
  - **Refs:** R6.1, R8 · design §9.
- [x] **9.3 Coerência de `docs/architecture.md`, `docs/deploy.md`, `README.md` e `docs/requirements.md` (R6.2)** (dep: 5.7, 7.9, 8.8) — revisão cruzada: `drive_folder` consistentemente "opcional" (nenhuma menção a obrigatório fora de contexto histórico); menções a "métricas adiadas" restantes são só ADRs/release-notes históricos (preservados). `README.md` ganhou a onda `auditoria-profunda` em Documentação e Status (make tools, metrics.addr, drive_folder opcional, OAuth state, rate limit). Docs de §0.10/§0.15/§0.21/§0.22/§0.23/§0.24, deploy (métricas+rede), requirements §3.8/§4.1/§4.3 já alinhados nas fases anteriores.
  - Revisão final de consistência: §0.10 (Drive DR real), §0.21, seção de auditoria, observabilidade, `google.*`, `metrics.addr`, instalação do linter, duas camadas de rate limit. `README.md` com o uso novo (`make tools`, `metrics.addr`, `drive_folder` opcional). Corrigir divergências entre o que cada fase escreveu.
  - Arquivos: `docs/architecture.md`, `docs/deploy.md`, `README.md`, `docs/requirements.md`.
  - **Pronto quando:** leitura cruzada não encontra contradição (ex.: `grep -n "drive_folder" docs/` não descreve mais a pasta como obrigatória).
  - **Refs:** R6.2, R10 · design §9.
- [x] **9.4 Consolidar a tabela "Status ao vivo" da `pocketbase-finish` (R6.3)** (dep: —) — merge reconfirmado (`5aeeabe Merge pull request #4`, ancestral da `main`). Em `pocketbase-finish/tasks.md`: removida a linha duplicada `10 — Fechamento ⬜ pendente` (sobra 1, com "mergeada no PR #4"); `10.1/10.2/10.3` marcados `[x]`; Fase 7 e 10 com commit `5aeeabe`; "Agora" atualizado. `grep -c '| 10 — Fechamento'` = 1; Fase 10 sem `[ ]`.
  - **Pré-verificado em 2026-11-05:** `git log --oneline` mostra `5aeeabe Merge pull request #4 from RafaelFino/pocketbase-finish`. Antes de marcar, reconfirmar rapidamente com `git log --oneline main | head` / `gh pr view 4`. Em `.kiro/specs/pocketbase-finish/tasks.md`: remover a linha duplicada `10 — Fechamento ... ⬜ pendente`, manter a `✅ concluída` com a observação "mergeada no PR #4"; marcar `[x]` em `10.1`, `10.2` e `10.3`; atualizar a linha **Agora**.
  - Arquivo: `.kiro/specs/pocketbase-finish/tasks.md`.
  - **Pronto quando:** `grep -c "10 — Fechamento" .kiro/specs/pocketbase-finish/tasks.md` mostra uma única linha na tabela e a seção da Fase 10 não tem `[ ]`. Se o merge não puder ser confirmado, parar e perguntar ao Rafael.
  - **Refs:** R6.3 · design §9 e §11.5 item 7.
- [x] **9.5 Checklist de rotação de segredos para o Rafael (R7.3)** (dep: —) — §9.1 "Segurança: rotação de segredos" em `docs/deploy.md`: rotacionar `google.client_secret` + refazer refresh token pelo authorize, `llm.gemini.api_key`, chave de cifra (mantendo as antigas no mapa), `git check-ignore config.dev.yaml`, testes não leem o config dev. Operacional, sem valores. Repetido como item no checklist final (§10) e será marcável pelo Rafael na task 10.3.
  - Seção de segurança em `docs/deploy.md` com o checklist **operacional** (sem valores): rotacionar `google.client_secret` e refazer o refresh token por `/v1/admin/google/authorize`; rotacionar `llm.gemini.api_key`; confirmar `git check-ignore config.dev.yaml`; não ler nem copiar `config.dev.yaml` em testes. **Item de checklist a cargo do Rafael**, fora do código; a task só documenta, não rotaciona.
  - Arquivo: `docs/deploy.md`.
  - **Pronto quando:** a seção existe e a task 10.3 a repete como item de fechamento marcável pelo Rafael.
  - **Refs:** R7.3 · design §9.
- [x] **Fechamento Fase 9:** `make check` verde (docs-only: sem mudança de código); steering e docs coerentes; commit `<fase9>` `auditoria-profunda Fase 9: alinha steering e docs ao código e fecha pendências documentais`.

---

## Fase 10 — Fechamento da onda

- [ ] **10.1 Release notes visuais** (dep: Fases 1 a 9)
  - Entrada `auditoria-profunda` no topo de `docs/release-notes.md`: título, data, resumo em uma frase e listas **Novidades** (auditoria do prontuário, pasta de DR automática, retenção de snapshots, limite de tentativas, `state` do OAuth, métricas Prometheus em listener interno), **Melhorias** (ator real no `audit_log`, linter ativo no gate e no CI, `/readyz` com `checks.drive_dr`), **Correções** (steering sem Gmail, tabela da Fase 10) e **Notas de migração/operação** (`drive_folder` opcional: instâncias que tinham o DR desativado por falta dele passam a ativá-lo; `drive_retention` default 14; `metrics.addr` ausente = sem listener; o callback do OAuth exige o cookie do authorize, quem tinha o fluxo aberto reinicia; snapshots antigos sem `appProperties` ficam fora da retenção; rotação de segredos). Linguagem orientada ao usuário/terapeuta, sem valores de segredo.
  - Arquivo: `docs/release-notes.md`.
  - **Pronto quando:** a entrada existe, com todas as seções, no topo do arquivo.
  - **Refs:** R10 · `psicoman-processo-entrega.md` (Release notes).
- [ ] **10.2 Verificação final da branch** (dep: 10.1)
  - `make check > /tmp/check-final.log 2>&1; echo "exit=$?"` e `go test -race ./internal/... > /tmp/race-final.log 2>&1; echo "exit=$?"`, ambos `exit=0`, com o lint real. Conferir `git status` limpo, `git log --oneline main..HEAD` com um commit por fase, nenhum segredo/`config.dev.yaml`/artefato temporário versionado, e a matriz de cobertura abaixo toda marcada.
  - **Pronto quando:** os dois comandos terminam com `exit=0` e o log de commits bate com a tabela de status.
- [ ] **10.3 Checklist de fechamento (para o Rafael)** (dep: 10.2)
  - [ ] Segredos do Google (`client_secret`, refresh token) e do Gemini (`api_key`) **rotacionados** (R7.3).
  - [ ] `config.dev.yaml` segue git-ignored (`git check-ignore config.dev.yaml`).
  - [ ] Firewall/rede: porta de métricas acessível somente do coletor; nada publicado na borda.
  - [ ] Proxy/Pangolin repassa `X-Forwarded-Proto` (cookie `Secure` do OAuth).
  - [ ] Decisões abertas do design aceitas ou ajustadas (retenção N=14, `127.0.0.1:9091`/`0.0.0.0:9091`, audit best-effort, limites 5/h e 10/h, baseline do linter).
  - [ ] Backlog aberto: enumeração por `409` no `register`; login nativo do paciente sem limite próprio.
  - [ ] Spike manual de `drive.file` (task 4.1) com resultado registrado, ou a retenção está no fallback "manual".
- [ ] **10.4 Push e PR somente com OK do Rafael** (dep: 10.3)
  - **PARAR e pedir o OK do Rafael.** Só com a confirmação explícita: `git push -u origin auditoria-profunda` e abrir o PR (`gh pr create`, título curto, descrição com resumo, o que foi testado e o que ficou pendente). Nunca push para a `main`.
  - **Pronto quando:** o Rafael confirmou; a branch está no remoto; o PR aberto.
- [ ] **Fechamento Fase 10:** tabela "Status ao vivo" toda `✅ concluída`, com os commits; commit final `auditoria-profunda Fase 10: fecha a onda com release notes e checklist`.

---

## Matriz de cobertura (requisito → tarefa → design)

| Requisito | Tarefas | Seção do `design.md` |
|-----------|---------|----------------------|
| R1.1, R1.2, R1.3, R1.4 | 3.4, 3.5, 3.6, 3.7, 3.8 | §1, §6 |
| R2.1, R2.2 | — **descartados por decisão do Rafael; nada a implementar** | §12 (linha "R2.1, R2.2, R3.2") |
| R2.3 | 5.1, 5.2, 5.7 | §2, §9 (R6.1) |
| R2.4 | 5.1, 5.3, 5.4, 5.5, 5.6, 5.7 | §2, §3.1 |
| R3.1 | 4.1, 4.2, 4.3, 4.4, 4.5, 4.6 | §3, §3.2 |
| R3.2 | — **descartado por decisão do Rafael; nada a implementar** (a documentação do comportamento real segue em R6.2: 5.7, 9.3) | §12 |
| R3.3 | 5.4, 5.5, 5.6, 5.7 | §3, §3.1 |
| R3.4 | 4.3, 4.4, 4.5, 4.6 | §3, §3.1 |
| R4.1 | 1.1, 1.2, 1.5, 2.1, 2.2 | §4 |
| R4.2 | 1.3, 1.4, 1.5, 2.2 | §4 |
| R5.1 | 3.6 | §1, §5 |
| R5.2 | 3.7 | §1, §5 |
| R6.1 | 5.2, 9.1, 9.2 | §9 |
| R6.2 | 5.7, 7.9, 8.8, 9.3 | §9 |
| R6.3 | 9.4 | §9, §11.5 |
| R7.1 | 6.1, 6.2, 6.3, 6.7, 8.6 | §7 |
| R7.2 | 6.4, 6.5, 6.6, 6.7 | §8 |
| R7.3 | 9.5, 10.3 | §9 |
| R7.4 | 3.1, 3.2, 3.3, 3.7, 3.8 | §6 |
| R8.1 | 7.7, 7.8 | §10.5 |
| R8.2 | 7.1, 7.5, 7.6, 7.8 | §10.3, §10.4 |
| R8.3 | 7.5, 7.6, 7.8, 8.6, 8.7 | §10.3, §10.4 |
| R8.4 | 7.4, 8.1, 8.2 | §10.2, §10.3 |
| R8.5 | 8.3 | §10.2, §10.3 |
| R8.6 | 5.4, 8.5 | §3.1, §10.3, §2, §3 |
| R8.7 | 8.3, 8.4, 8.7 | §10.2, §10.3, §1 |
| R8.8 | 7.5, 8.3 | §10.1, §10.3 |
| R8.9.1, R8.9.2, R8.9.4 | 7.6, 7.8, 8.7 | §10.3, §10.4, §10.8 |
| R8.9.3 | 7.7, 7.8, 7.9 | §10.5, §10.6, §10.8 |
| R8.10 | 7.6 | §10.4 |
| R8.11 | 7.2, 7.3, 7.4 | §10.1, §10.7 |
| R8.12 | 7.9 | §10.5 |
| R9 (rastreabilidade) | — nada a implementar; o único toque é a extensão do `/readyz` com `checks.drive_dr` (5.5) | §12 |
| R10 (DoD) | todas as tasks de fechamento de fase, 9.3, 10.2 | §11.4 |
