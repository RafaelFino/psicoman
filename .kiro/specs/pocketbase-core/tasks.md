# pocketbase-core — Plano de Implementação (Tasks)

Reescrita incremental da base técnica sobre PocketBase-como-framework. Cada task é demonstrável e testável, apoia-se nas anteriores e integra ao todo (sem código órfão). Refs apontam para [`requirements.md`](./requirements.md), [`design.md`](./design.md) e, ao fechar a onda, a fonte durável [`docs/requirements.md`](../../../docs/requirements.md) / [`docs/architecture.md`](../../../docs/architecture.md).

Convenções:
- `[ ]` pendente · `[~]` em progresso · `[x]` concluído.
- Cada task só é concluída com **build verde** (`make build`), **testes verdes** (`make check` quando aplicável) e **demo funcional** (smoke via HTTP e/ou tela).
- Toda mensagem ao usuário em PT-BR; valores em centavos; fuso `America/Sao_Paulo`.
- Toda regra sensível é imposta no servidor (defense-in-depth), mesmo com o `Pangolin` na frente; a UI apenas reflete.

**Definição de pronto (obrigatória em toda task — ver [`.kiro/steering/psicoman-processo-entrega.md`](../../steering/psicoman-processo-entrega.md)):**
- **Docs junto do código**: toda mudança de comportamento atualiza, no mesmo ciclo, `docs/requirements.md` (o quê), `docs/architecture.md` (como), `README.md` (se muda uso), Swagger (se muda contrato) e `docs/decisions.md` (se houve decisão relevante). Nunca "documentar depois".
- **Testes cobrem a mudança**: regra nova → unit no domínio/`internal/app`; fluxo/contrato novo → E2E sobre o binário único (Google/Gemini mockados); segurança/acesso (gate de aprovação) → E2E permitido **e** negado (2xx + 403/401); cripto → round-trip e não-vazamento do `SealedText`. Nenhum `t.Skip` sem justificativa no próprio teste.
- **Git por fase**: trabalhar na **branch da onda** (`pocketbase-core`). Ao concluir cada **Fase**, `make check` verde → **commit** em PT-BR imperativo referenciando a fase. **Push só no fim da onda** (após a Fase 10 verde); PR só então.

**Ordem das fases (dependências):** 0 (fundação) → 1 (crypto + `SealedText`) → 2 (domínio rico) → 3 (`platform/pb` + coleções) → 4 (API `/v1`) → 5 (auth + gate) → 6 (Calendar) → 7 (Drive fallback) → 8 (Gemini + tags) → 9 (portal do paciente) → 10 (fechamento: reescrever docs + release notes + push/PR).

---

## 📊 Status ao vivo da onda

> Atualizado pelo agente a cada virada de etapa. `✅ concluída` · `🔄 em progresso` · `⬜ pendente`.

| Fase | Estado | Commit | Observação |
|------|--------|--------|------------|
| 0 — Fundação (binário único + PocketBase) | ✅ concluída | `d11fb6d` | admin UI + smoke de boot verdes |
| 1 — `SealedText` + cripto por campo | ✅ concluída | `b56deb6` | AES-256-GCM, envelope, não-vazamento |
| 2 — Domínio rico por bounded context | ✅ concluída | `4940c5f` | regras portadas do legado |
| 3 — Adapter `platform/pb` + coleções | ✅ concluída | `c486b30` | cifra na borda comprovada por teste |
| 4 — Contrato `/v1` (envelope, DTOs, Swagger) | ✅ concluída | `a8248d0` | DTOs sem vazar clínico; D13 superuser |
| 5 — Auth `Pangolin` + gate de aprovação | ✅ concluída | `c30295a` | 401/403 permitido+negado + hook defesa-em-profundidade |
| 6 — Google Calendar síncrono + PDF cobrança | ✅ concluída | `05f3def` | 6.0 config + 6.1 Calendar + 6.2 PDF no GED; `make check` verde |
| 7 — Drive fallback/DR (hook de backup) | ✅ concluída | `f2b31a4` | `drivesync` + hook `OnBackupCreate`, snapshot cifrado (`BlobSealer`), `drive.file`; backup sem Google = 202 |
| 8 — IA Gemini + índice de tags | ✅ concluída | `1f57e06` | Gemini (interface+fake), resumo revisável, opt-in pela chave, auditoria minimizada, `tag_index` isolado por paciente |
| 9 — Portal do paciente | ✅ concluída | `0c46723` | coleção auth `patient_user` (1:1 patient), SEAM → sessão real, gate permitido/negado com auth real |
| 10 — Fechamento (docs + release notes) | ✅ concluída (aguarda push) | `4db533e` | docs canônicos reescritos, release notes, config.example, pendências a/b/c tratadas; `make check` verde na branch — **push/PR aguarda OK do Rafael** |

**Agora:** Fase 10 **concluída e verde na branch inteira — aguardando OK do Rafael para push/PR.** Entregue: reescrita integral de `docs/requirements.md` (arquitetura nova: binário único, `/v1` por ator, Calendar síncrono, Drive DR, Gemini+tags, portal com auth; Gmail fora de escopo) e `docs/architecture.md` (§0 canônica 0.1–0.12; seções legadas 1–9 removidas); `README.md` reescrito; `docs/release-notes.md` com a entrada `pocketbase-core` (orientada ao usuário); `config.example.yaml` no formato novo (sem segredos). Pendências tratadas: (a) links `.agents/*-plan.md` git-ignored removidos de `decisions.md`; (b) redação de `SealedText`/`MarshalJSON` alinhada ao código no `design.md` e no steering `psicoman-golang.md` (com OK do Rafael — SealedText sem MarshalJSON, cifra na borda + DTO/`Reveal`); (c) mensagens ao usuário conferidas e uniformes (capitalizadas) — nenhuma fora do padrão. `make check` verde. **Débito sinalizado ao Rafael:** `docs/deploy.md` e `docs/stack-local.md` descrevem o modelo antigo (dois binários/Gmail/dev_mode) e receberam nota de "desatualizado" no topo apontando para a arquitetura vigente; a reescrita operacional completa deles não estava no escopo do fechamento e fica pendente de decisão. **Próximo passo é só seu:** confirmar o push da branch `pocketbase-core` e abertura do PR.

**(histórico) Fase 9 concluída e verde.** Entregue: migração `0040_patient_auth` (coleção auth `patient_user`, email+senha nativo, relation UNIQUE 1:1 ao `patient`, sem migração destrutiva — Opção A); identidade REAL substituiu o SEAM `X-Portal-Email` (`httpx.PortalAuthRecordID` lê `e.Auth` da sessão do PocketBase, confirma coleção `patient_user`); porta `app.PortalAuth` (+adapter pb) resolve `patient_id` da sessão; `POST /v1/portal/register` cria patient pendente + credencial (senha mín. 8); login nativo `auth-with-password`; gate e handlers resolvem pela sessão real. Testes: migrations (patient_user é auth), gate em processo reescrito para token real (`Record.NewAuthToken`), E2E (cadastro + login real + gate negado pendente 403 / permitido aprovado 2xx pela mesma sessão; sem sessão 401). Docs: `architecture.md §0.12`, `decisions.md` (ADR Fase 9), Swagger (`portalSession` → bearer, register com password). `make check` verde. **Próximo:** Fase 10 — fechamento: reescrita integral de `docs/requirements.md` + `docs/architecture.md` (seções legadas), `README.md`, `docs/release-notes.md`, `config.example.yaml`; limpeza de pendências (links `.agents` git-ignored, redação D13/steering, nit do erro do hook); `make check` final → **PARAR e confirmar com o Rafael antes de push/PR**. PONTO DE ATENÇÃO: perguntar ao Rafael antes de editar o steering `psicoman-golang.md` (D13/SealedText MarshalJSON).

**(histórico) Fase 8 concluída e verde.** Entregue: `internal/integration/gemini` (Summarizer interface + fake + REST `generateContent`, minimização do envio, api_key da config nunca logada); `app.AIService` (opt-in pela presença da chave → off recusa 422; `Suggest` grava resumo `sugerido` cifrado e devolve tags para revisão sem indexar; `Consolidate` marca `consolidado` e indexa só as tags revisadas; auditoria `ai.process` sem conteúdo clínico); `tag_index` (tags em claro, corpo `SealedText`, isolado por paciente, merge idempotente); adapters pb (cifra na borda do `ai_summary`); rotas no ator app. Testes: unit (IA off recusa, minimização, auditoria sem clínico, consolidação/indexação, isolamento por paciente; cifra na borda não-vazamento; merge/isolamento do índice; REST parse/minimização) + E2E (IA off → 422). Docs: `architecture.md §0.11`, `decisions.md` (ADR Fase 8), Swagger (+3 rotas). `make check` verde. **Próximo:** Fase 9 — Portal do paciente: coleção auth do paciente (`core.NewAuthCollection`, adiada da Fase 5), substituir o SEAM `X-Portal-Email` pela identidade real da sessão, recursos mínimos respeitando o gate; E2E cadastro/login + gate permitido (2xx) e negado (403) com auth real. **PRÓXIMO PONTO DE ATENÇÃO (Fase 10):** validar com o Rafael o ajuste da redação de D13/steering sobre `SealedText` sem `MarshalJSON` antes de editar o steering.

**(histórico) Fase 7 concluída e verde.** Entregue: cifra de blob dedicada `crypto.BlobSealer` (AES-256-GCM, mesmas chaves do `KeyProvider`, envelope binário com rotação e AAD); `google.DriveClient` (interface + fake + REST multipart, escopo mínimo `drive.file`); `internal/drivesync.Syncer` com hook `OnBackupCreate` (deixa o backup local rodar, lê o snapshot, cifra e sobe à pasta fria `drive_folder`); wiring opcional (`wireDriveSync` degrada para no-op sem credenciais); `POST /v1/admin/backup` agora dispara `CreateBackup` real. Testes: unit (cifra blob round-trip/rotação/não-vazamento, Drive upload, Syncer cifra+envia) + E2E (backup sem Google = 202, sem 500). Docs no mesmo ciclo: `architecture.md §0.10`, `decisions.md` (ADR Fase 7, Opção A), Swagger (summary do backup). `make check` verde. **Próximo:** Fase 8 — IA Gemini (`internal/integration/gemini`, interface + fake): resumo + tags, disparo explícito, revisável, opt-in pela presença de `llm.gemini.api_key`, auditoria `ai.process` sem clínico; índice invertido de tags por paciente/ano (tags em claro, corpo `SealedText`, isolado por paciente). Nenhuma ação pendente do Rafael.

---

## Fase 0 — Fundação pocketbase-core (R1)

- [x] **0.1. Binário único com PocketBase embarcado** (dep: —)
  - `cmd/psicoman/main.go` com `app := pocketbase.New()` + registro de rotas no hook `app.OnServe()`.
  - `go.mod`: adicionar o PocketBase como dependência; remover os antigos `cmd/admin` e `cmd/portal`.
  - **Aceite:** `make build` produz **um** executável que sobe o PocketBase e serve a admin UI.
  - **Testes:** smoke de boot (sobe e responde health na porta configurada).
  - **Refs:** R1.1, R1.2 · D1.

- [x] **0.2. Esqueleto de pacotes e Makefile** (dep: 0.1)
  - Criar `internal/platform/{crypto,pb}`, `internal/app`, `internal/domain/{patient,clinical,billing,scheduling,costs,therapist}`, `internal/integration/{google,gemini}`, `internal/drivesync`, `internal/api` (pacotes vazios/esboço).
  - Makefile: `build` de **binário único** (saída = arquivo, não diretório), `check` (fmt + vet + build + testes), `test-unit`, `e2e`.
  - **Aceite:** `make check` passa no esqueleto; a regra de dependência (domínio não importa PB) está documentada no pacote.
  - **Testes:** `go vet`/build do esqueleto; teste trivial de que `internal/domain` não importa `platform/pb`.
  - **Refs:** R1.1, R5.1 · D0, D5.

- [x] **Fechamento Fase 0:** `docs/decisions.md` já cobre a arquitetura (2026-10-21); conferir estrutura de pastas; `make check` verde → commit "pocketbase-core Fase 0: fundação do binário único com PocketBase embarcado".

---

## Fase 1 — `platform/crypto` + `SealedText` (R6)

- [x] **1.1. Pacote `platform/crypto` (AES-256-GCM + `KeyProvider`)** (dep: 0.2)
  - `internal/platform/crypto`: `Seal`/`Open` com `crypto/aes`+`crypto/cipher`; `KeyProvider` (resolve chave por `id`, informa `CurrentID()`); montagem/parse do envelope `versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext` (base64).
  - **Aceite:** cifrar/decifrar round-trip; decifrar com `id_da_chave` antigo após trocar o corrente (rotação).
  - **Testes:** unit de round-trip, rotação por `id_da_chave`, nonce único por operação, falha ao decifrar com chave errada.
  - **Refs:** R6.2, R6.4 · D6, D11.

- [x] **1.2. Tipo de domínio `SealedText`** (dep: 1.1)
  - `internal/domain` (tipo de valor compartilhado): `MarshalJSON` emite sempre o envelope cifrado; `UnmarshalJSON` faz parse e mantém cifrado; `Reveal(ctx)` decifra sob demanda; `Seal(ctx, plaintext)`.
  - **Aceite:** `MarshalJSON` nunca emite o claro; `Reveal` devolve o claro; `UnmarshalJSON(MarshalJSON(x))` preserva o envelope.
  - **Testes:** round-trip `Seal → MarshalJSON → UnmarshalJSON → Reveal`; **não-vazamento** (`MarshalJSON` só envelope); log do tipo não expõe o claro.
  - **Refs:** R6.1, R6.3, R6.5 · D6.

- [x] **Fechamento Fase 1:** `docs/architecture.md` (cifra por campo), `docs/decisions.md` (se refinar envelope), Swagger não muda ainda; `make check` verde → commit "pocketbase-core Fase 1: SealedText e cripto por campo".

---

## Fase 2 — Domínio rico por bounded context (R4)

- [x] **2.1. Agregados dos bounded contexts** (dep: 1.2)
  - `internal/domain/{patient,clinical,billing,scheduling,costs,therapist}`: agregados conforme o **MER canônico (D4b)**, com comportamento, validação, serialização e formatação de log (sem vazar clínico). Campos clínicos sensíveis em `clinical` usam `SealedText` (D6).
  - Padrões: ULID em toda entidade; dinheiro `int64` centavos; instantes `America/Sao_Paulo` ISO-8601; idempotência de efeito financeiro.
  - **Aceite:** cada agregado valida-se e transita de estado por conta própria; serviços não concentram regra de domínio.
  - **Testes:** unit por context (validação, transições da sessão em `scheduling`, idempotência em `billing`, `SealedText` em `clinical`).
  - **Refs:** R4.1, R4.2, R4.3, R11.2 · D4, D4b, D6.

- [x] **2.1b. Portar as regras de negócio existentes para os contexts** (dep: 2.1)
  - Reimplementar no domínio rico, por bounded context, as regras já maduras do Psicoman (fonte: `docs/requirements.md §3.3–§3.6` e os services atuais) — **não** reinventar um billing pobre:
    - `billing`: geração de débito por sessão realizada (opt-out via `bill`), fechamento de ciclo de planos fechados por `due_day` com idempotência (`plan_id + billing_period`), `atendimento_social` nunca gera débito, cálculo de status de pagamento (parcial/pago).
    - `costs`: rateio de custo por período, custo por sessão/paciente, ROI por origem.
    - `scheduling`: ciclo de vida completo da sessão (confirmação, cancelamento com autor/motivo, `SessionCategory`), remarcação rastreável.
    - `clinical`: timeline/história do paciente, métricas de sessão, feed do dia (derivados).
  - **Aceite:** as regras portadas têm paridade de comportamento com a documentação atual; nenhum caso de borda conhecido regrediu.
  - **Testes:** unit por regra (feliz + borda: dupla geração idempotente, período vazio, social sem débito, rateio por período).
  - **Refs:** R4.1, R11.2 · D4, D4b.

- [x] **2.2. Portas de repositório + serviços enxutos** (dep: 2.1)
  - Definir no domínio/app as **interfaces de porta** (repositórios) por context; `internal/app` com serviços enxutos que orquestram agregados + portas.
  - **Aceite:** o domínio não importa o PocketBase; os serviços dependem só das portas.
  - **Testes:** unit dos serviços com **fakes** das portas.
  - **Refs:** R4.1, R5.1 · D4, D5.

- [x] **Fechamento Fase 2:** `docs/architecture.md` (bounded contexts + domínio rico); `make check` verde → commit "pocketbase-core Fase 2: domínio rico por bounded context".

---

## Fase 3 — `platform/pb` adapter + coleções/migrations (R5, R11)

> **Fonte das coleções:** o **MER canônico (D4b)** é a referência única de entidades, campos, constraints e relations. Cada sub-task abaixo materializa um grupo do MER como coleções do PocketBase (migrations).

- [x] **3.1a. Coleções `patient` + `scheduling`** (dep: 2.2)
  - Coleções `patient` (UNIQUE `email`; UNIQUE parcial `cpf`; `approval_status`; relation `origin`), `origin`, `appointment_request`, `session` (relations `patient`/`location`; status + metadados de cancelamento), `availability`, `location`. Conforme D4b.2.
  - **Aceite:** boot cria as coleções; UNIQUE de email/CPF e relations ativos.
  - **Testes:** migração no boot (base nova, idempotente); violar UNIQUE de email/CPF falha.
  - **Refs:** R11.1 · D4b, D5.

- [x] **3.1b. Coleções `billing` + `costs`** (dep: 3.1a)
  - Coleções `contract`, `plan` (relation `contract`), `debt` (UNIQUE `idempotency_key`; relations `patient`/`session`/`plan`), `payment` (relation `debt`), `cost_category`, `cost_item` (relation `origin`), `session_cost` (UNIQUE `session_id`). Conforme D4b.2/D4b.3.
  - **Aceite:** chave de idempotência de débito e 1:1 de `session_cost` ativos; reprocessar não duplica.
  - **Testes:** violar `ux_debt_idempotency` falha; dupla geração idempotente de débito.
  - **Refs:** R11.1, R11.2 · D4b, D5.

- [x] **3.1c. Coleções `clinical` + `therapist` + transversais** (dep: 3.1a)
  - Coleções `anamnesis` (UNIQUE `patient_id`; campo 🔒), `note` (campo 🔒; relation `session` nullable), `ai_summary` (campo 🔒; relation `session`), `tag_index` (tags em claro), `therapist_profile`, `therapist_platform_link`, `ged_file`, `audit_log`, `google_token` (token cifrado). Conforme D4b.2.
  - **Aceite:** campos 🔒 guardam envelope (nunca claro); `tag_index` com tags em claro; 1:1 de anamnese.
  - **Testes:** migração no boot; conferir que coluna clínica aceita o envelope string.
  - **Refs:** R6.5, R11.1 · D4b, D5, D6.

- [x] **3.2. Adapter `platform/pb` (domínio↔Record) + cifra na borda** (dep: 3.1a, 3.1b, 3.1c)
  - `internal/platform/pb` implementa as portas de repositório, mapeando agregado↔`core.Record`. Ao gravar campo clínico, cifra via `Cipher` e persiste o **envelope** como string; ao ler, decifra para `SealedText` em memória (D6.3). Único pacote que importa o PocketBase.
  - **Aceite:** CRUD por context round-trip via adapter; nada clínico em claro no banco (só envelope).
  - **Testes:** integração do adapter contra o PB embarcado (round-trip, incl. cifra→envelope→decifra); teste de que o domínio não importa `platform/pb`.
  - **Refs:** R5.1, R5.2, R6.3 · D5, D6.

- [x] **Fechamento Fase 3:** `docs/architecture.md` (coleções/migrations + adapter); `make check` verde → commit "pocketbase-core Fase 3: adapter platform/pb e coleções com constraints".

---

## Fase 4 — API `/v1` + envelope + Swagger + segregação por ator (R2, R7)

- [x] **4.1. Router `/v1` + envelope + middleware chain** (dep: 3.2)
  - `internal/api`: grupos por ator `/v1/app/*` (terapeuta), `/v1/portal/*` (paciente), `/v1/admin/*` (operação) sobre o router do PB (via `app.OnServe()`); envelope `{ message, elapsed_ms, data, error }` (PT-BR); middleware recover/request-id/timing/logging.
  - Restringir a REST automática do PocketBase para que **não** seja o contrato público.
  - **Aceite:** rotas `/v1` respondem no envelope padrão; `elapsed_ms` preenchido; paths por ator separados; REST automática não expõe o contrato.
  - **Testes:** E2E do envelope (sucesso e erro); teste de que a REST automática não serve dado clínico em claro.
  - **Refs:** R2.1, R7.1, R7.2 · D2, D7, D13.

- [x] **4.1b. DTOs de resposta (leitura de prontuário sem vazar o claro)** (dep: 4.1)
  - DTOs explícitos por resposta: o DTO do terapeuta (`/v1/app/*`) inclui o campo clínico em claro via `Reveal()`; **nenhum** DTO do portal inclui campo clínico (D6.3). Nenhuma resposta serializa `SealedText` direto.
  - **Aceite:** `GET /v1/app/...` devolve o prontuário legível; o portal nunca recebe campo clínico.
  - **Testes:** E2E lê prontuário no app (claro) e confirma ausência de clínico em qualquer rota do portal; teste de não-vazamento.
  - **Refs:** R6.3 · D6.3.

- [x] **4.2. Rotas de negócio + Swagger** (dep: 4.1b)
  - Rotas por context nos paths certos (ex.: `POST /v1/app/sessions`, `GET /v1/app/patients/{id}/debts`, `GET /v1/portal/me`), handlers finos que orquestram os serviços; Swagger/OpenAPI do `/v1`.
  - **Aceite:** rotas principais exercitadas por E2E; Swagger descreve o contrato.
  - **Testes:** E2E de contrato das rotas principais; validação do Swagger.
  - **Refs:** R7.1, R7.3, R11.1 · D7.

- [x] **Fechamento Fase 4:** `docs/architecture.md` (contrato `/v1` por ator + DTOs), `docs/requirements.md`, Swagger; `make check` verde → commit "pocketbase-core Fase 4: contrato /v1 com envelope, DTOs e segregação por ator".

---

## Fase 5 — Auth atrás do `Pangolin` + gate de aprovação (R3)

- [x] **5.1. Auth da terapeuta/operação via header do `Pangolin`** (dep: 4.1)
  - Middleware nos grupos `/v1/app/*` e `/v1/admin/*` que lê a identidade do header injetado pelo `Pangolin`; recusa (401) se ausente.
  - **Aceite:** requisição sem header é negada; com header é aceita.
  - **Testes:** E2E com e sem header (2xx/401).
  - **Refs:** R3.1 · D3.

- [x] **5.2. Gate de aprovação do paciente (middleware do portal + hook de reforço)** (dep: 5.1, 3.2)
  - Estado de aprovação (`pendente`/`aprovado`/`rejeitado`) no `patient`. O enforcement vive no **middleware do grupo `/v1/portal/*`** (nega 403 paciente não-`aprovado` além do próprio cadastro/estado); um **hook do PocketBase** nas coleções sensíveis reforça (defesa em profundidade).
  - **Aceite:** paciente `pendente`/`rejeitado` é barrado (403) pelo middleware; `aprovado` acessa; o hook também barra escrita direta.
  - **Testes:** E2E permitido (2xx) e negado (403); unit do serviço de aprovação.
  - **Refs:** R3.2, R3.3 · D3, D13.

- [x] **Fechamento Fase 5:** `docs/architecture.md`/`docs/requirements.md` (auth + gate), `docs/decisions.md` (gate via hook), Swagger; `make check` verde → commit "pocketbase-core Fase 5: auth atrás do Pangolin e gate de aprovação".

---

## Fase 6 — Integração Google Calendar síncrona (R8)

- [x] **6.1. Calendar síncrono (interface + fake)** (dep: 4.2)
  - `internal/integration/google`: Calendar atrás de interface + fake; `freebusy` antes de confirmar; evento + Meet quando online; degrada sem panic sem credenciais. **Sem Gmail** — a integração cobre só Calendar e Drive.
  - **Aceite:** agendar/confirmar cria evento (mock); sem Google não dá erro fatal.
  - **Testes:** unit com fake; E2E agendar com Google mockado e sem Google.
  - **Refs:** R8.1 · D8.

- [x] **6.2. PDF de cobrança (sem envio por email)** (dep: 4.2)
  - Gerar o PDF de cobrança a partir do débito e armazená-lo no GED (file storage do PocketBase). **Nenhuma** integração de envio por email (Gmail fora de escopo); o envio é por WhatsApp, fora da plataforma.
  - **Aceite:** gerar cobrança produz um PDF recuperável no GED; não há caminho de envio por email.
  - **Testes:** unit/E2E de geração do PDF e persistência no GED.
  - **Refs:** R9.3 · D9b.

- [x] **Fechamento Fase 6:** `docs/architecture.md`/`docs/requirements.md` (Calendar síncrono + PDF de cobrança sem Gmail), Swagger; `make check` verde → commit "pocketbase-core Fase 6: Google Calendar síncrono e PDF de cobrança".

---

## Fase 7 — Drive fallback/DR via hook de backup (`drivesync`) (R9)

- [x] **7.1. Sync de backup para o Drive** (dep: 3.1, 6.1)
  - `internal/drivesync`: hook no ciclo de **backup** do PocketBase dispara o upload do snapshot (cifrado) para pasta fria no Drive; escopo OAuth `drive.file`; fora do caminho de request; pode ser 2º processo.
  - **Aceite:** ao gerar backup, o snapshot é enviado ao Drive (mock); nenhuma rota de negócio toca o Drive.
  - **Testes:** unit com fake de Drive (upload chamado no hook de backup); E2E de backup sem Google não dá 500.
  - **Refs:** R9.1, R9.2 · D9.

- [x] **Fechamento Fase 7:** `docs/architecture.md`/`docs/requirements.md` (Drive fallback/DR), `docs/decisions.md` (Drive fora do caminho de request); `make check` verde → commit "pocketbase-core Fase 7: Drive como fallback/DR via hook de backup".

---

## Fase 8 — IA Gemini + índice de tags (R10)

- [x] **8.1. Gemini: resumo + tagueamento (opt-in, revisável)** (dep: 4.2, 2.1)
  - `internal/integration/gemini` (interface + fake): resumo clínico + tags; disparo explícito; resultado revisável antes de consolidar; opt-in por instância (default off); auditoria `ai.process` sem conteúdo clínico; minimização.
  - **Aceite:** com IA off, nada roda; com IA on, resumo/tags gerados e revisáveis; auditoria sem clínico.
  - **Testes:** unit com fake (minimização, auditoria sem clínico); E2E opt-in on/off.
  - **Refs:** R10.1, R10.3, R10.4 · D10.

- [x] **8.2. Índice invertido de tags por paciente/ano** (dep: 8.1, 3.2)
  - Coleção/estrutura `tag -> lista de IDs de sessão`, por paciente e ano, isolada por paciente; tags em **claro** (indexáveis) enquanto o corpo é `SealedText`.
  - **Aceite:** consultar por tag devolve as sessões do paciente/ano; sem busca cross-paciente; corpo clínico permanece cifrado.
  - **Testes:** unit do índice (inserção/consulta por tag, isolamento por paciente); E2E de busca por tag.
  - **Refs:** R10.2, R6.5 · D10.

- [x] **Fechamento Fase 8:** `docs/architecture.md`/`docs/requirements.md` (IA + índice de tags), `docs/decisions.md` (tags em claro, corpo cifrado), Swagger; `make check` verde → commit "pocketbase-core Fase 8: Gemini e índice invertido de tags".

---

## Fase 9 — Portal do paciente (R2, R3)

- [x] **9.1. Portal mínimo (cadastro/login + recursos)** (dep: 5.2)
  - `/v1/portal/*`: cadastro/login (auth do PocketBase), recursos mínimos do paciente (próprios dados, agendamento quando previsto), respeitando o gate de aprovação.
  - **Aceite:** paciente cadastra, loga e acessa só o permitido; não-`aprovado` barrado.
  - **Testes:** E2E do fluxo de cadastro/login; permitido e negado pelo gate.
  - **Refs:** R2.2, R3.2, R3.3 · D2, D3.

- [x] **Fechamento Fase 9:** `docs/requirements.md`/`README.md` (portal do paciente), Swagger; `make check` verde → commit "pocketbase-core Fase 9: portal do paciente".

---

## Fase 10 — Fechamento da onda

- [x] **10.1. Reescrever docs canônicos para a nova arquitetura** (dep: 0–9)
  - Reescrever `docs/requirements.md` (o quê/por quê na arquitetura pocketbase-core) e `docs/architecture.md` (binário único, bounded contexts, `platform/pb`, `SealedText`, contrato `/v1`, Calendar síncrono, Drive fallback, Gemini + índice de tags). Atualizar `README.md` (uso: um binário atrás do `Pangolin`).
  - **Aceite:** docs canônicos descrevem a arquitetura da onda, sem resíduo das explorações descartadas.
  - **Testes:** revisão de coerência (links, nomenclatura canônica idêntica à ADR).
  - **Refs:** processo de entrega · todas as R#.

- [~] **10.2. Release notes, `make check` e push/PR** (dep: 10.1) — release notes escritas e `make check` verde na branch; **push + PR aguardam o OK do Rafael**.
  - `docs/release-notes.md` com a entrada `pocketbase-core` (Novidades: binário único com PocketBase embarcado, cifra por campo `SealedText`, contrato `/v1`, Drive como DR, Gemini opt-in com índice de tags; Melhorias: auth simplificada atrás do `Pangolin`; Notas de operação: um artefato, `data dir` do PB, chave de cifra via `KeyProvider`).
  - `make check` verde na branch inteira. **Push da branch `pocketbase-core`** (fim da onda) e abertura de PR para `main`.
  - **Aceite:** release notes publicadas; branch verde; PR aberto.
  - **Refs:** processo de entrega.

- [ ] **Fechamento Fase 10:** `docs/requirements.md`/`docs/architecture.md`/`README.md`/`docs/release-notes.md` atualizados; `make check` verde → commit "pocketbase-core Fase 10: reescrita das docs canônicas e fechamento da onda" → push + PR.
