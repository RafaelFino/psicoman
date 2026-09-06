# mvp-audit1 — Design

Como implementar os requisitos de [`requirements.md`](./requirements.md), respeitando a arquitetura vigente ([`docs/architecture.md`](../../../docs/architecture.md)): Go puro, SQLite, SSR com `html/template` + Alpine.js (sem SPA), envelope de API padrão, domínio magro + services.

Princípios que guiam este ciclo:
- **Defense in depth**: o gate de aprovação é imposto no servidor; a UI só reflete.
- **Reaproveitar o que existe**: envelope, `panel()`, `Fmt`, `Md.toHtml`, design system CSS, middleware de portal.
- **Nada de código órfão**: cada peça de backend nova é exercida por uma tela; cada tela consome endpoints reais.

---

## D1. Gate de aprovação de paciente (R1)

### D1.1 Modelo de domínio
- Acrescentar ao `domain.Patient` o campo `ApprovalStatus string` com constantes:
  - `PatientPending  = "pendente"`
  - `PatientApproved = "aprovado"`
  - `PatientRejected = "rejeitado"`
- Helpers no domínio: `(*Patient) IsApproved() bool`, `CanTransitionApproval(next string) bool` (pendente → aprovado|rejeitado; estados finais não retrocedem por padrão — reavaliação futura fora de escopo).
- `Validate()` aceita apenas estados conhecidos.

### D1.2 Persistência
- Migration nova `000X_patient_approval.sql`:
  - `ALTER TABLE patients ADD COLUMN approval_status TEXT NOT NULL DEFAULT 'pendente';`
  - `UPDATE patients SET approval_status = 'aprovado';` — os pré-existentes eram cadastrados pelo terapeuta ⇒ aprovados (R1.1). Idempotente: a coluna só é criada uma vez pelo framework de migrations no boot.
- Repositório SQLite (`repository/sqlite/patient.go`): incluir a coluna em insert/select/update; novo método `ListByApproval(ctx, status)`.

### D1.3 Serviço
- `PatientService`:
  - `RegisterFromPortal` cria com `pendente` (auto-cadastro). Se o email já existe e está `aprovado`, mantém `aprovado` e faz o upsert dos dados (não rebaixa — R1.1).
  - `Create` (admin) cria com `aprovado`.
  - `Approve(ctx, id)` / `Reject(ctx, id)`: aplicam transição válida, persistem e devolvem o paciente. O handler admin registra audit log.
  - `ListPending(ctx)` para a fila.

### D1.4 Enforcement no portal (defense in depth)
- Novo middleware no portal, encadeado **depois** do `Authenticator` nas rotas de recurso: `RequireApproved`.
  - Resolve o paciente pelo email da sessão (`patients.GetByEmail`).
  - Se `IsApproved()` → segue. Senão → `httpx.ErrForbidden("Seu cadastro está em análise. Você poderá acessar assim que o terapeuta aprovar.")` (HTTP 403).
- Reorganizar `Handlers.RegisterAuthenticated`:
  - **Sempre liberadas (só exigem sessão):** `GET /me`, `GET /approval-status`, `POST /logout`.
  - **Exigem aprovação (sessão + RequireApproved):** `PUT /me` (edição pós-cadastro), `GET /availability`, `POST /appointment-requests`, `GET /appointment-requests`, `GET /sessions`, `GET /debts`.
  - O cadastro inicial permanece na rota pública `PUT /register` (cria pendente); a edição de perfil migra para trás do gate.
- Novo endpoint `GET /v1/portal/approval-status` → `{ status, name, email }`, usado pela UI para decidir o que mostrar.
- Se `httpx` não tiver `ErrForbidden`, adicionar (padrão dos demais helpers de erro em `internal/api/httpx/errors.go`), com código HTTP 403 e mensagem PT-BR.

### D1.5 Fila de aprovação no admin
- `AppointmentHandlers` já cuida de pedidos; a aprovação de **paciente** é distinta. Adicionar em `PatientHandlers`:
  - `GET /v1/admin/patients/pending` → lista `pendentes`.
  - `POST /v1/admin/patients/{id}/approve` → aprova (+ audit).
  - `POST /v1/admin/patients/{id}/reject` → rejeita (+ audit).
- `patientView` passa a incluir `approval_status`.

### D1.6 UI — portal
- `portal_app.html`: após login, ramificar por `approval_status` obtido em `GET /approval-status`:
  - `aprovado` → área normal (perfil, agenda, sessões, débitos).
  - `pendente` → card "Cadastro em análise" (ícone acolhedor, texto claro, botões Atualizar/Sair).
  - `rejeitado` → card informando que o acesso não foi liberado, com orientação de contato.
- O `portalApp()` guarda `status` e expõe `approved`/`pending`/`rejected` computados.

### D1.7 UI — admin
- Nova seção `#aprovacoes` no topo do `admin_app.html` (acima ou junto de "Pedidos de agendamento"), com badge de contagem.
- Componente `approvalsPanel()`: lista pendentes; botões Aprovar/Rejeitar com **confirmação inline** (R10.2); recarrega ao concluir.
- Item no menu (`layout.html`) "Aprovações" com contagem.

---

## D2. Telas de gap (R2–R9)

Todas seguem o mesmo padrão: `<section class="card">` + componente Alpine com `...panel()`, consumindo endpoints já existentes. Reuso de `Fmt`, `Md.toHtml`, e do CSS do design system.

### D2.1 Locais (R2) — `#locais`
- `locationsPanel()`: CRUD via `/v1/admin/locations`; para o local aberto, sub-lista de disponibilidade via `/locations/{id}/availability` (add/remove).
- Máscara de reais → centavos em util compartilhado (`Money.toCents`/`Money.fromCents` em `api.js`, ao lado de `Fmt.brl`).
- Seletor de dia da semana com rótulos PT-BR (Segunda…Sábado) mapeados ao inteiro `weekday` do backend.

### D2.2 Origens (R3) — `#origens` + campo no paciente
- `originsPanel()`: CRUD via `/v1/admin/origins`.
- Formulário de paciente (cadastro em `patientsPanel` e edição em `patientDetail`) ganha `<select>` de origem, carregando `/origins`. Envia `origin_id`.

### D2.3 Planos (R4) — aba no detalhe do paciente
- Nova aba "Plano" no `patientDetail`: lista `/plans?patient_id=`, cria (`POST /plans`) e remove.
- `<select>` de tipo com rótulos claros + textos explicativos por tipo; valor em reais (→ centavos) e datas de vigência para tipos fixos.

### D2.4 Agendar sessão (R5)
- Na agenda, botão "Nova sessão" (global e por dia) abre formulário inline: paciente (`<select>` de `/patients` aprovados), local (`<select>` de `/locations`), modalidade, início/fim (datetime-local, convertido para RFC3339).
- `POST /v1/admin/sessions` e, se necessário, `POST /sessions/{id}/schedule`. Erros de conflito exibidos via feedback do `panel()`.
- Pré-preenche a data ao acionar a partir de um dia da semana.

### D2.5 Custos e ROI (R6) — `#custos-gestao`
- `costsPanel()`: categorias (`/costs/categories`) e itens (`/costs/items`), listar/criar/remover.
- Relatórios: `/reports/costs` e `/reports/roi` com seletor de período (from/to ISO; default mês atual via `parsePeriod` do backend). Render em KPIs/tabelas legíveis (nunca JSON).

### D2.6 Perfil do terapeuta (R7) — `#perfil-terapeuta`
- `therapistPanel()`: `GET/PUT /profile`; foto via multipart `POST /profile/photo` (mesma técnica do upload GED já implementado); bio com editor Markdown (`mdField`/preview); locais associados via multi-seleção de `/locations`.
- Links: `GET/POST/DELETE /profile/links` com origem opcional.

### D2.7 Backup/Restore (R8) — `#backup`
- `backupPanel()`: `POST /backup` mostra resultado; `POST /restore` com **dupla confirmação inline** (digitar/confirmar) e aviso de reinício.

### D2.8 Auditoria (R9) — `#auditoria`
- `auditPanel()`: `GET /audit-log`, tabela legível (ator, ação, entidade, data via `Fmt.datetime`).

---

## D3. Usabilidade transversal (R10)

### D3.1 Portal sem JSON
- Substituir os `<pre class="code">` de "Minhas sessões" e "Meus débitos" por listas (`item-list`) com `Fmt.datetime`, status PT-BR e `Fmt.brl`.

### D3.2 Diálogos inline no lugar de prompt/confirm
- Componente utilitário Alpine `confirmable()` (estado de confirmação embutido no item) ou padrão de "linha de confirmação" já usado nas aprovações.
- **Pagamento**: trocar o `window.prompt` do `patientDetail.payDebt` por formulário inline (valor pré-preenchido com o saldo, `<select>` de forma de pagamento: pix/dinheiro/cartão/transferência).
- **Exclusões** (nota, template, plano, local, item de custo, link): confirmação inline (sem `confirm()`).

### D3.3 Login do portal coerente
- Ajustar textos do `portal_app.html` para refletir o mecanismo real do `IdentityVerifier`. Se o fluxo real é envio de credencial/token do Google, o texto deve descrevê-lo com honestidade; não prometer redirecionamento OAuth que não ocorre. (Decisão de copy validada com o time; sem mudar o backend de auth.)

### D3.4 Navegação e datas
- Menu do admin (`layout.html`) atualizado com todas as áreas novas, agrupadas (Atendimento / Financeiro / Gestão).
- `Fmt.short(iso)` novo helper: "Seg, 08/09 · 14:00" (usa `America/Sao_Paulo` implícito pelo browser; formato pt-BR).

### D3.5 Acessibilidade
- Todo novo controle: `<button>`/`<label for>`/`role`/`tabindex`/`@keydown.enter` conforme o padrão já adotado; foco visível herdado do CSS.

---

## D4. Impacto e compatibilidade

- **Migration de aprovação** é a única mudança de schema; aditiva e idempotente. Pré-existentes viram `aprovado` (não quebra ninguém).
- **Contrato de API**: adições apenas (novos endpoints e um campo em `patientView`); nada removido.
- **E2E**: a suíte existente do portal (`internal/api/portal/portal_test.go`) precisa considerar o gate — os testes de recurso passam a exigir paciente aprovado (ajuste de fixture: aprovar o paciente de teste). Isso é esperado e faz parte do escopo.

## D5. Riscos e mitigação

- **Regressão nos testes do portal** por causa do gate → ajustar fixtures e adicionar casos de 403 (pendente) e 200 (aprovado).
- **UI grande em um arquivo** (`admin_app.html` já é extenso) → manter componentes Alpine bem separados; se necessário, avaliar split de templates parciais (fora do escopo se não bloquear).
- **Máscara de dinheiro** (reais↔centavos) é fonte comum de bug → centralizar em `Money` com testes manuais no smoke.

## D6. Sequência recomendada de execução

R1 → (R2, R3) → R4 → R5 → R10.1/R10.2 → R6 → R7 → R8 → R9 → R10 remanescente. Cada bloco compila, sobe e é fumado antes do próximo.

## D7. Documentação, testes e lastro (processo)

Segue [`.kiro/steering/psicoman-processo-entrega.md`](../../steering/psicoman-processo-entrega.md), que é a política durável:
- **Docs junto do código**: toda entidade/endpoint/tela/migration/regra nova entra em `docs/requirements.md`, `docs/architecture.md`, Swagger e (uso) `README.md` no mesmo ciclo da task.
- **Testes**: cobrem cada mudança conforme `psicoman-testes-e2e.md`; segurança de acesso (R1) exige E2E de permitido e negado.
- **Decisões**: registradas em [`docs/decisions.md`](../../../docs/decisions.md); esta spec já tem a entrada da onda `mvp-audit1`.
- **Git**: commit por fase (Aceite de cada task de "Fechamento"), push único ao final (task E2).
