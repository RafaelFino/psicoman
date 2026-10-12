# prontuario-ux — Design

Design técnico da onda `prontuario-ux`. Referência de requisitos: [`requirements.md`](./requirements.md). Fontes de verdade: [`docs/architecture.md`](../../../docs/architecture.md), [`docs/requirements.md`](../../../docs/requirements.md).

Princípios do projeto respeitados aqui (de `docs/architecture.md`):
- Camadas: `domain` (structs magras + `Validate`) → `service` (orquestração, define interfaces de repositório) → `repository/sqlite`. API REST em `internal/api/admin/*` com envelope JSON. UI é SSR (`html/template`) + Alpine.js inline em `internal/web/templates/admin_app.html`, com CSS puro em `internal/web/static/app.css` (tokens em `:root`, sem build).
- Datas ISO-8601 (`clock.Format`/`Parse`), fuso America/Sao_Paulo. Dinheiro em centavos (não relevante nesta onda).
- Toda escrita de dado sensível (prontuário, sessão) passa pelo `AuditService`.

Visão geral das mudanças por camada:

| Área | Backend | Frontend | Testes |
|---|---|---|---|
| R1 Prontuário | — (reusa endpoints) | nova aba "Prontuário" (timeline) | smoke de template |
| R2 Edição de sessão | `PUT /sessions/{id}` (service+api) | painel da sessão + registros | unit (service) + e2e (contrato) |
| R2 Registros por sessão | — (reusa `POST notes` c/ session_id) | form com `session_id` | e2e (nota vinculada) |
| R3 WhatsApp | `domain.NormalizePhone` + chamada no service | link `wa.me` | unit (normalização) + e2e |
| R4 Paleta | — | tokens CSS `:root` | smoke visual (manual) |
| R5 Calendário fluido | — | layout da semana no CSS/markup | smoke de template |

---

## 1. R3 — Normalização de telefone (domínio)

**Onde:** `internal/domain/patient.go` (nova função) e `internal/service/patient.go` (invocação).

Espelha o padrão já existente de `NormalizeCPF` (chamado em `Create`, `Update`, `RegisterFromPortal`). O telefone passa a ter forma canônica: **somente dígitos, com DDI `55`** quando ausente.

```go
// NormalizePhone devolve o telefone em forma canônica: só dígitos, com DDI do
// Brasil (55). Preserva números que já cheguem com DDI; descarta máscara.
// Retorna "" se não houver dígitos. A validação de plausibilidade fica no
// Validate (telefone obrigatório + comprimento mínimo).
func NormalizePhone(phone string) string {
    digits := onlyDigits(phone)          // remove +, (), -, espaços
    if digits == "" {
        return ""
    }
    // Já tem DDI 55 + (DDD 2 + 8/9 dígitos) => 12 ou 13 dígitos.
    if strings.HasPrefix(digits, "55") && (len(digits) == 12 || len(digits) == 13) {
        return digits
    }
    // Número nacional (DDD + 8/9) => 10 ou 11 dígitos: prefixa 55.
    if len(digits) == 10 || len(digits) == 11 {
        return "55" + digits
    }
    // Caso ambíguo: devolve só os dígitos; o Validate decide se é aceitável.
    return digits
}
```

**Validação (R3.1):** `Patient.Validate` já exige telefone não vazio. Acrescentar checagem de **plausibilidade** do canônico: após normalização, exigir comprimento entre 12 e 13 dígitos (DDI 55 + DDD + número). Mensagem PT-BR clara: "O telefone precisa ter DDD e número (ex.: 11 91234-5678)."

**Decisão sobre onde normalizar:** no **service**, antes de `Validate`, como já é feito com CPF (`p.CPF = domain.NormalizeCPF(in.CPF)`). Assim o dado chega canônico ao `Validate` e ao repositório, valendo para admin (`Create`/`Update`) e portal (`RegisterFromPortal`). Não é feito no handler (regra de negócio pertence ao domínio/serviço, R3.1).

**Edge cases tratados por teste unitário:**
- `"11999990001"` → `"5511999990001"` (11 dígitos → +55).
- `"(11) 99999-0001"` → `"5511999990001"` (máscara).
- `"5511999990001"` → inalterado (já tem DDI).
- `"+55 11 3333-4444"` → `"551133334444"` (fixo 10 díg + DDI).
- `"123"` → rejeitado no `Validate` (implausível).

**Compatibilidade:** registros antigos no banco com telefone sem DDI continuam exibíveis; o link `wa.me` (R3.2) usa o número como está persistido. Como a normalização passa a valer na escrita, novos cadastros/edições já saem canônicos; não há migração destrutiva (uma migração de dados para normalizar o legado é possível, mas fica como melhoria opcional — registrar em decisions se adotada).

---

## 2. R2 — Edição de sessão (`PUT /sessions/{id}`)

**Backend já pronto na persistência:** `SessionRepo.Update` (em `internal/repository/sqlite/session.go`) já faz `UPDATE session SET location_id, modality, starts_at, ends_at, status, ...`. Portanto **não há mudança de schema nem SQL novo**. Falta expor no service e na API.

### Service (`internal/service/session.go`)

```go
// SessionUpdateInput são os campos editáveis de uma sessão (nunca o paciente).
type SessionUpdateInput struct {
    LocationID string
    Modality   string
    StartsAt   time.Time
    EndsAt     time.Time
}

// Update altera os dados de agendamento de uma sessão existente. Não troca o
// paciente. Bloqueia edição de horário/local em estados terminais.
func (s *SessionService) Update(ctx context.Context, id string, in SessionUpdateInput) (*domain.Session, error) {
    sess, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, err
    }
    if sess.IsTerminal() {
        return nil, NewValidation("Esta sessão já foi concluída e não pode ser reagendada.")
    }
    sess.LocationID = in.LocationID
    sess.Modality = in.Modality
    sess.StartsAt = in.StartsAt
    sess.EndsAt = in.EndsAt
    sess.UpdatedAt = s.clock.Now()
    if err := sess.Validate(); err != nil {   // fim>início, modalidade válida, status válido
        return nil, NewValidation(err.Error())
    }
    // Conflito de agenda: reusa ensureNoConflict se a sessão está agendada.
    if sess.Status == domain.SessionAgendada {
        if err := s.ensureNoConflict(ctx, sess); err != nil {
            return nil, err
        }
    }
    if err := s.repo.Update(ctx, sess); err != nil {
        return nil, err
    }
    return sess, nil
}
```

Notas de design:
- **Paciente imutável** (R2.2): `SessionUpdateInput` não tem `PatientID`; o handler não lê esse campo.
- **Estados terminais** (R2.2): `IsTerminal()` já existe no domínio (realizada/cancelada/falta). Edição de horário/local bloqueada nesses estados; o registro clínico (R2.3) é outra rota (notas) e continua permitido.
- **Conflito de agenda:** reusa `ensureNoConflict` (no-op sem Google; freebusy quando configurado), só quando a sessão está `agendada` — coerente com `Create`.

### Google Calendar (R2.2, decisão)

Edição de horário de uma sessão com `GoogleEventID` **não** reconcilia o evento no Calendar nesta onda. Justificativa: o `SessionCalendar` atual só expõe `CreateEvent` (não há `UpdateEvent`), e implementar atualização/remoção remota amplia o escopo e o risco. **Decisão:** no MVP desta onda, a edição altera apenas o registro local; a UI avisa o terapeuta, quando a sessão tem evento no Google, que o convite no Google deve ser ajustado manualmente se o horário mudar. Registrar em `docs/decisions.md`. Um `UpdateEvent` fica como melhoria futura (fora de escopo, ver requirements).

### API (`internal/api/admin/session.go`)

Nova rota no `Register`: `g.Handle("PUT", "/sessions/{id}", h.update)`.

```go
type sessionUpdateBody struct {
    LocationID string `json:"location_id"`
    Modality   string `json:"modality"`
    StartsAt   string `json:"starts_at"` // RFC3339
    EndsAt     string `json:"ends_at"`   // RFC3339
}
```

Handler `update`: decodifica, faz `time.Parse(time.RFC3339, ...)` (422 em formato inválido, como no `create`), chama `svc.Update`, audita (`AuditActionUpdate`, "session"), responde `sessionView(s)`. Erros do service mapeados por `respondServiceError` (404 not found, 422 validação, 409 conflito).

**Contrato de resposta:** reusa `sessionView` (já inclui `location_id`, `modality`, `starts_at`, `ends_at`, `status`, `meet_url`, `updated_at`). Sem mudança de contrato de leitura.

---

## 3. R1 + R2 — Frontend: Prontuário e painel de sessão

Tudo em `internal/web/templates/admin_app.html` (markup + componente Alpine `patientDetail()`), reusando helpers de `internal/web/static/api.js` (`Api`, `Fmt`, `Md`).

### 3.1 Abas
- Remover as abas **"Sessões"** e **"Notas"**; adicionar **"Prontuário"**. Manter **"Anamnese"**, **"Plano"**, **"Débitos"**, **"Custos"**, **"Arquivos"**.
- `go(tab)` passa a tratar `tab === "prontuario"` carregando sessões + notas (reusa `loadSessions` e `loadNotes`, que já existem).

### 3.2 Montagem da timeline (cliente)
Dados já disponíveis no componente: `this.sessions` (de `GET /sessions?patient_id=`) e `this.notes` (de `GET /patients/{id}/notes`, cada nota com `session_id`/`is_free`). Construir uma estrutura de dias:

```
buildProntuario():
  itens = []
  para cada sessão s: itens.push({ tipo:'sessao', data:s.starts_at, sessao:s,
                                    registros: notes.filter(n => n.session_id === s.id) })
  para cada nota n livre (n.is_free): itens.push({ tipo:'nota', data:n.created_at, nota:n })
  ordena itens por data desc
  agrupa por dia (YYYY-MM-DD) -> this.timeline = [{ dia, label, itens }]
```

- Agrupamento por dia com rótulo PT-BR (`Fmt` já formata datas/horas). Dia mais recente no topo (R1.2).
- Registros de sessão aninhados sob o marco da sessão (R2.4); notas livres como itens próprios.
- Markdown via `Md.toHtml` (R1.2).

### 3.3 Resumo (R1.3)
KPIs no topo do Prontuário derivados de `this.sessions`: total, realizadas (`status==='realizada'`), faltas (`status==='falta'`), data da última sessão. Reaproveita `realizadas()` existente.

### 3.4 Painel da sessão (R2.1–2.3)
- Clicar num marco de sessão (no Prontuário) ou num slot (na Agenda) abre o painel da sessão. Na Agenda, o slot hoje chama `openPatient(s.patient_id)`; passa a abrir a sessão específica (navegação via store ou estado local do `patientDetail`, abrindo o paciente e focando a sessão).
- Painel mostra: dados da sessão (horário, local, modalidade, status), ações de transição já existentes (finalizar/cancelar/falta quando aplicável), um formulário de **edição** (data/hora, local, modalidade) que chama `Api.put('/v1/admin/sessions/'+id, body)` e, ao sucesso, recarrega; e um bloco de **registros** com editor Markdown (reusa o visual do editor de notas) cujo "Adicionar" chama `Api.post('/v1/admin/patients/'+pid+'/notes', { session_id: id, content })`.
- Edição desabilitada/escondida quando `IsTerminal` (status realizada/cancelada/falta), com dica PT-BR; registros continuam permitidos (R2.2/R2.4).
- Aviso de Google: quando `s.meet_url` existe e o horário for alterado, exibir hint "ajuste o convite no Google se necessário" (R2.2 decisão).

Edição estruturada do markup: manter a estética de abas/itens existente (`.tabs`, `.item-list`, `.md-editor`), sem introduzir dependências novas.

---

## 4. R3.2 — Link de WhatsApp (frontend)

Helper em `api.js` (`Fmt` ou novo `Wa`) para montar o href e um rótulo legível:

```js
// Monta o link do WhatsApp Web a partir do telefone canônico (só dígitos, DDI).
Wa.link(phone) => phone ? `https://wa.me/${onlyDigits(phone)}` : null
Wa.pretty(phone) => formata para exibição amigável (ex.: +55 (11) 99999-0001)
```

Nos três pontos do `admin_app.html` que mostram telefone (cabeçalho do detalhe, aba Dados, lista de pacientes `patientsPanel`), trocar o texto por:

```html
<template x-if="patient && patient.phone">
  <a :href="Wa.link(patient.phone)" target="_blank" rel="noopener"
     x-text="Wa.pretty(patient.phone)"></a>
</template>
<template x-if="!(patient && patient.phone)"><span>—</span></template>
```

- `target="_blank"` + `rel="noopener"` (R3.2).
- Sem telefone válido → "—", nunca link quebrado (R3.2).
- O `patientView` do backend já expõe `phone`; nenhuma mudança de contrato.

---

## 5. R4 — Nova paleta (tokens CSS)

Em `internal/web/static/app.css`, bloco `:root`. Substituir a família teal/indigo por **azul royal (primary)** + **lilás (accent)**, mantendo a estrutura de tokens (as classes de UI já referenciam `var(--primary)` etc., então a troca propaga). Proposta de valores (ajustáveis), todos verificados para contraste AA:

```css
--bg:            #f5f4fb;  /* lilás/azul muito claro (fundo app) */
--surface:       #ffffff;  /* branco (cartões) */
--surface-2:     #faf9fe;  /* branco levemente lilás */

--primary:       #3730a3;  /* indigo-800 / azul royal — branco sobre ele ~8:1 */
--primary-strong:#312e81;  /* indigo-900 */
--primary-soft:  #e0e7ff;  /* indigo-100 (fundo suave) */

--accent:        #7c3aed;  /* violet-600 — lilás */
--accent-soft:   #ede9fe;  /* violet-100 */

--ring: 0 0 0 3px rgba(124, 58, 237, .35);  /* foco em lilás */
```

- Estados preservados: `--ok` (verde), `--warn` (âmbar), `--danger` (vermelho) e seus `-soft` ficam, pois são semânticos (R4.1).
- Gradientes que usam `var(--primary)`/`var(--accent)` (logo/hero) herdam a nova paleta automaticamente; revisar se há cor teal/verde **hardcoded** fora da semântica de estado e substituir (R4.3).
- Contraste (R4.2): branco (#fff) sobre `--primary #3730a3` ≈ 8:1 (AA/AAA ok); texto slate-900 sobre `--bg`/`--surface` mantém ~15:1. Validar com conferência rápida ao aplicar; ajustar tom do primary se algum botão específico ficar < 4.5:1.

Os valores exatos podem ser afinados na implementação; o terapeuta aprovou a direção (azul royal + lilás).

---

## 6. R5 — Calendário semanal fluido

Em `admin_app.html` (markup `.week-grid`/`.week-day`/`.slot` + `buildWeek()`) e `app.css`.

### Estratégia: dia que cresce com a lotação (não grade de horas)
- Manter a grade de 6 colunas (seg–sáb) no desktop, mas remover a `min-height` fixa restritiva: a coluna do dia cresce em altura conforme a quantidade de `.slot`, alinhando as colunas pelo topo (`align-items: start`) para que um dia cheio não force os vizinhos.
- Cartões (`.slot`) com layout compacto porém legível (horário em destaque, nome do paciente, ícone de modalidade, status por cor). Garantir alvo de toque ≥ 44px (R5.1).
- Com ~10 sessões, o dia simplesmente estende para baixo; a página rola. Não há sobreposição por horário (decisão: sem eixo de horas nesta onda).
- Densidade: considerar um limite visual com rolagem interna por coluna **somente** se necessário; preferir crescimento natural da página (mais simples e previsível para leigos).

### Clique abre a sessão (R2.1)
- `.slot` deixa de só abrir o paciente; passa a abrir o **painel da sessão** (seção 3.4). Mantém `openNewOn(day)` para criar nova sessão no dia.

### Status e dia atual (R5.3)
- Classes `.slot.st-agendada|st-solicitada|st-realizada|st-cancelada|st-falta` reusadas; cores derivam dos tokens novos (R4). `.week-day.is-today` mantém o destaque.

### Responsivo (R5.4)
- Media query existente (`max-width: 720px`) já colapsa para coluna única; manter e revisar para que os cartões continuem legíveis empilhados.

Domingo permanece **fora** da semana (seg–sáb), como já é hoje e como o terapeuta pediu (R5.2). A visão mensal não muda, exceto herdar a paleta.

---

## 7. R3.3 — Massa de testes (seed)

Em `scripts/seed.sh`:
- Telefones dos pacientes passam a ter DDD válido de forma que, após `NormalizePhone`, virem `55…` (ex.: enviar `11999990001`; o backend normaliza para `5511999990001`). Como a normalização passa a ocorrer no serviço, basta garantir que os números tenham 10–11 dígitos.
- **Adicionar várias sessões no mesmo dia** para exercitar o calendário lotado (R5.1): criar ~8–10 sessões num mesmo dia (ex.: amanhã) em horários sequenciais (09:00, 10:00, …), além de algumas em outros dias da semana. Status variados (`solicitada`/`agendada`) para exibir cores distintas. Usar a função `iso <dias> <HH:MM>` já existente.

---

## 8. Impacto em contrato de API / Swagger

- **Novo endpoint:** `PUT /v1/admin/sessions/{id}` (body `location_id`, `modality`, `starts_at`, `ends_at`; resposta `sessionView`). Atualizar o Swagger (`internal/api/swagger`).
- Sem novos campos em `patientView`/`noteView`/`sessionView`. `POST /patients/{id}/notes` já aceita `session_id` (sem mudança).
- Normalização de telefone muda o **valor persistido** de `phone` (comportamento), não o shape do contrato.

## 9. Riscos e mitigação

- **Telefone legado sem DDI:** link `wa.me` pode abrir número incompleto para registros antigos. Mitigação: normalização na próxima edição do paciente; opcionalmente migração de dados (decidir e registrar). Novos dados já saem corretos.
- **Edição de sessão vs Google Calendar:** divergência entre horário local e convite no Google. Mitigação: aviso explícito ao terapeuta (seção 2) + decisão registrada; `UpdateEvent` fica para onda futura.
- **Timeline com histórico longo:** render de muitas sessões/notas. Mitigação: agrupamento por dia e render de Markdown client-side já são O(n); se necessário, paginar por período (fora do escopo inicial).
- **Contraste da nova paleta:** validar AA ao aplicar; ajustar tom do primary se algum componente ficar abaixo do limite.

## 10. Verificação (resumo; detalhe em tasks.md)

- Unit (domínio/serviço): `NormalizePhone` + `Validate` de telefone; `SessionService.Update` (edição, bloqueio terminal, conflito).
- E2E (`test/e2e/`): `PUT /sessions/{id}` (sucesso, paciente imutável, estado terminal → erro, datas inválidas → 422); nota vinculada a sessão aparecendo com `session_id`; telefone normalizado no cadastro.
- Smoke: parse/render dos templates do admin após as mudanças de markup.
- Manual documentado: aparência da nova paleta e fluidez do calendário com dia lotado (seed com ~10 sessões/dia).
