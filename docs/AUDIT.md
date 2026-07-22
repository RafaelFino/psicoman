# Psicoman — Auditoria Completa e Plano de Evolução

Data: 2026-07-22 (revisão 2)

---

## 1. Estado Atual — O que já existe e funciona

### Backend Go (testes passando: `go test ./...`)

| Feature | Status | Camadas |
|---------|--------|---------|
| Pacientes (CRUD, busca email/Google sub) | ✅ Completo | storage + service + web |
| Agendamentos (CRUD, conflitos, regras cancel/reschedule) | ✅ Completo | domain + storage + service + web |
| Google Calendar + Meet (OAuth, eventos, links) | ✅ Completo | service + web |
| Regras de agendamento configuráveis | ✅ Completo | domain + storage + web |
| Financeiro (pagamentos, custos, relatórios por paciente) | ✅ Completo | storage + service + web |
| GED (upload/download, organizado por tenant/paciente) | ✅ Completo | service + web |
| Auth dupla (Pangolin + JWT/Google OAuth) | ✅ Completo | service + middleware |
| Multi-tenant (DB por tenant via header) | ✅ Completo | middleware |
| Logs estruturados (zerolog + lumberjack, JSON, rotação) | ✅ Completo | web/logger |
| Docker multi-stage (frontend embed + Go) | ✅ Completo | Dockerfile + compose |
| DEV_MODE (rotas dev, seed, login sem Google) | ✅ Completo | web/handlers_dev |
| Session Notes (evoluções com métricas de tempo) | ✅ Completo | migration 002 + storage + service + web |
| Anamnese estruturada (templates adulto/criança + respostas) | ✅ Completo | migration 003 + storage + service + web |
| Contrato terapêutico (templates, envio, assinatura digital) | ✅ Completo | migration 004 + storage + service + web |

### Frontend React (SPA)

| Tela | Status | Observações |
|------|--------|-------------|
| Psych Dashboard (agenda do dia) | ✅ Funcional | Mostra apenas consultas de hoje |
| Psych Appointments (CRUD consultas) | ✅ Funcional | Form + tabela + notas |
| Psych Patients (CRUD pacientes) | ✅ Funcional | Básico, sem link para detalhes |
| Psych SessionNotes (evoluções) | ✅ Funcional | Criar/editar/filtrar |
| Psych AnamnesisTemplates | ✅ Funcional | CRUD templates + ver respostas |
| Psych Contracts | ✅ Funcional | Templates + envio + revogação |
| Psych Finance | ✅ Funcional | Resumo + relatórios |
| Psych Settings (regras + Google) | ✅ Funcional | Config regras + conectar Google |
| Patient Login (Google + Dev) | ✅ Funcional | OAuth + fallback dev |
| Patient Dashboard | ✅ Funcional | Lista consultas |
| Patient Book (agendar) | ✅ Funcional | Slots disponíveis |
| Patient Anamnesis | ✅ Funcional | Formulário dinâmico + free-text |
| Patient Contracts | ✅ Funcional | Ler + assinar |
| Patient Documents | ✅ Funcional | Lista + download |

---

## 2. Problemas Identificados

### 2.1 Bugs e Code Smells

| # | Arquivo | Problema | Severidade | Fix |
|---|---------|----------|------------|-----|
| 1 | `storage/document.go` | Variável `rows` declarada e nunca usada (dead code no branch sem patientID). Código confuso com interface genérica desnecessária. | Baixa | Reescrever `ListDocuments` de forma direta |
| 2 | `web/handlers_psych.go:downloadDocument` | Abre arquivo via `GED.Open()`, depois chama `c.File(doc.Path)` que abre novamente. O `defer f.Close()` do primeiro open é inútil, e pode causar leak se o file descriptor não for GC'd a tempo. | Média | Remover `GED.Open()` e usar apenas `c.File(doc.Path)` com verificação de existência inline |
| 3 | `service/google.go:fallback()` | Gera URL fake `https://meet.google.com/lookup/<8chars>` quando Google Calendar não está disponível. Essa URL não é funcional e confunde o usuário. | Baixa | Retornar string vazia ou mensagem "Meet indisponível" |
| 4 | `web/middleware.go:StaffAuth()` | Em produção (sem DEV_MODE), se nenhum header Pangolin estiver presente, aceita o request com `DefaultTenantID` e email genérico. Se a porta 8080 for acidentalmente exposta, qualquer pessoa acessa as rotas do psicólogo. | **Alta** | Em produção, rejeitar requests sem `X-User-Email` com 401 |
| 5 | `domain/scheduling.go` | Cálculo `now.Sub(appt.ScheduledAt)` retorna valor negativo quando o appointment é futuro. O check `hours < 0 && -hours < MinHours` funciona mas é counter-intuitive. | Baixa | Reescrever como `appt.ScheduledAt.Sub(now).Hours() < MinHours` (positivo = futuro) |
| 6 | `service/patient.go:FullReport` | Usa `from.AddDate(10, 0, 0)` como "to" para buscar todos os appointments. Funciona, mas carrega appointments de 10 anos desnecessariamente se o patient_id filter não existisse. | Baixa | Usar `time.Now().AddDate(1, 0, 0)` como limite superior |
| 7 | `web/config.go` | Função `envInt` definida e nunca usada em lugar nenhum. | Info | Remover ou usar para configurações futuras |
| 8 | `service/calendar_adapter.go:DBCalendar` | Campo `DB *storage.DB` é mutado pelo handler via `setCalendarDB(c)` — isso torna o `DBCalendar` compartilhado entre requests com race condition potencial se houver concorrência. | **Alta** | Passar `db` como parâmetro nos métodos ou usar context |
| 9 | Frontend `PatientLayout` | Verifica existência de token em localStorage mas não valida expiração. Paciente pode ter token expirado e ver tela vazia com erros 401 em cada request. | Média | Verificar claims.exp client-side, redirect para login se expirado |
| 10 | Frontend `PsychPatients` | Nomes dos pacientes na tabela não são links clicáveis. Não há rota `/psych/patients/:id` no React Router. | Média | Adicionar rota de detalhes + links |

### 2.2 Gaps de Segurança

| # | Gap | Impacto | Recomendação |
|---|-----|---------|--------------|
| 1 | Sem headers de segurança | Clickjacking, MIME sniffing | Adicionar `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, CSP |
| 2 | Sem rate limiting | Brute force em login, DoS | Middleware com token bucket (10 req/min em /auth/*, 60/min em /patient/*) |
| 3 | Upload sem validação de tamanho | DoS via uploads grandes | `gin.MaxMultipartMemory = 10 << 20` (10MB) + reject |
| 4 | Upload sem validação real de MIME | Executables disfarçados | Usar `http.DetectContentType` no server (não confiar no header) |
| 5 | `DBCalendar.DB` race condition | Data corruption em cenário concorrente | Redesenhar para receber db por parâmetro |
| 6 | `StaffAuth` inseguro em prod | Acesso não autorizado se porta exposta | Rejeitar sem header de email |

### 2.3 Dívida Técnica

| # | Item | Impacto |
|---|------|---------|
| 1 | Frontend React é um build separado que requer npm/Node | Complexidade operacional, 2 linguagens, build mais lento |
| 2 | `setCalendarDB` muta estado global compartilhado | Race condition |
| 3 | Testes cobrem happy path mas faltam edge cases (token expirado, tenant inexistente, upload com MIME inválido) | Bugs em produção |
| 4 | `report_html` no Appointment e `content_html` no SessionNote são redundantes | Dados podem ficar dessincronizados |
| 5 | Frontend usa `dangerouslySetInnerHTML` para contratos sem sanitização | XSS se template malicioso |

---

## 3. Validação da Stack — Decisão Arquitetural

### Análise: React SPA é adequado para este projeto?

**Argumentos contra React (para este caso)**:
- Sistema usado por 1 psicólogo + alguns pacientes (low traffic)
- Build requer Node.js 20+ como dependência extra
- 2 processos em dev (Vite + Go), complexidade operacional
- Frontend minimalista — nenhum componente justifica React (nenhum state management complexo, nenhum real-time, nenhum virtualização de lista)
- Bundle JS de ~150KB para páginas que são essencialmente formulários e tabelas
- O Dockerfile precisa de 2 stages extras por causa do React

**Recomendação: eliminar React, adotar Go templates + htmx + Alpine.js**:

| Critério | React (atual) | Go templates + htmx | Vantagem |
|----------|--------------|---------------------|----------|
| Build | 2 etapas (npm + go) | 1 etapa (go) | htmx |
| Binário final | Go + JS bundle embed | Go + templates embed | htmx |
| Dev | 2 terminais (vite + go) | 1 terminal (go com hot-reload) | htmx |
| Dependências | node_modules (300MB+) | Zero (htmx é 1 arquivo JS de 14KB) | htmx |
| Responsividade | Manual CSS | Manual CSS (igual) | Empate |
| Interatividade | SPA completo | htmx (AJAX parcial) + Alpine.js (state local) | htmx simplifica |
| SEO | Irrelevante (app privado) | Irrelevante | Empate |
| Performance percebida | SPA load + API calls | Server-rendered HTML | htmx (menos round trips) |
| Complexidade | Média | Baixa | htmx |

### Decisão: Migrar para Go templates + htmx + CSS moderno

**Stack frontend proposta**:
- **Go `html/template`** — Server-side rendering, embed dos templates no binário
- **htmx 2.x** — Interações AJAX (criar/editar/deletar sem full page reload), 14KB gzipped
- **Alpine.js 3.x** — Estado client-side local (modais, tabs, toggles), 15KB gzipped
- **CSS custom** — Design system com CSS variables, Grid, Flexbox, responsive
- **Zero npm, zero Node.js** — Tudo embarcado no Go binary

**Transição**: Reimplementar as telas progressivamente, mantendo a API REST existente. O backend não muda (só o layer de rendering).

---

## 4. Features Faltantes — Plano de Evolução

### Prioridade 1: Já implementado, precisa de polish

| Feature | Estado | Ação necessária |
|---------|--------|-----------------|
| Session Notes | Backend completo, frontend funcional | Migrar UI para novo design |
| Anamnese estruturada | Backend completo, frontend funcional | Migrar UI |
| Contrato terapêutico | Backend completo, frontend funcional | Migrar UI |

### Prioridade 2: Features novas a implementar

#### E4: Supervisões do Terapeuta
- Tabelas: `supervisors`, `supervision_sessions`
- CRUD completo (backend + frontend)
- Métricas de horas de supervisão integradas nos relatórios

#### E5: Espaços/Consultórios
- Tabelas: `therapy_spaces`, `space_bookings`
- Vincular consultas presenciais a espaços
- Custos de espaço no módulo financeiro
- Disponibilidade afeta slots

#### E6: Google Calendar — Teste em produção
- Código existe, falta configurar credenciais e testar fluxo real
- Tratar refresh token automaticamente (SDK já faz)
- Considerar webhook para sync reverso

### Prioridade 3: Relatórios e Métricas

#### E7: Relatórios Mensais Consolidados
- Endpoint: `GET /api/psych/reports/monthly?month=&year=`
- Dados: sessões, evoluções, horas, financeiro, exportar PDF
- Depende de: session notes com campos de duração (já implementado)

#### E8: Métricas de Horas
- Horas com paciente (session_notes.duration_patient_min)
- Horas de análise (session_notes.duration_analysis_min)
- Horas admin (session_notes.duration_admin_min)
- Horas supervisão (supervision_sessions.duration_minutes)
- Consolidação mensal no dashboard

### Prioridade 4: UX e Interface (parte da migração para htmx)

#### E9: Responsividade Mobile-First
- CSS Grid/Flexbox com breakpoints mobile-first
- Touch targets >= 44px
- Menu lateral colapsável em mobile
- Tabelas → cards em telas pequenas

#### E10: Página 360° do Paciente
- Clicar em qualquer nome de paciente → visão completa
- Seções: dados, consultas, evoluções, documentos, contratos, pagamentos, métricas

#### E11: Agenda como Dashboard Principal
- Vista semanal/diária como tela inicial
- Slots clicáveis para agendar
- Consultas clicáveis para detalhes
- Cores por status

#### E12: Menu lateral + Header moderno
- Sidebar fixa com navegação
- Header com data/hora, nome do sistema, paciente em atendimento
- Design limpo, tons leves, tipografia clara

---

## 5. Schema de Banco — Migrations necessárias

Já existentes: 001 (init), 002 (session_notes), 003 (anamnesis), 004 (contracts)

### Migration 005: Supervisões
```sql
CREATE TABLE IF NOT EXISTS supervisors (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT DEFAULT '',
    specialty TEXT DEFAULT '',
    crp TEXT DEFAULT '',
    notes TEXT DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS supervision_sessions (
    id TEXT PRIMARY KEY,
    supervisor_id TEXT NOT NULL REFERENCES supervisors(id),
    scheduled_at TEXT NOT NULL,
    duration_minutes INTEGER DEFAULT 60,
    notes_html TEXT DEFAULT '',
    topics TEXT DEFAULT '',
    cost_cents INTEGER DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'scheduled',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_supervision_scheduled ON supervision_sessions(scheduled_at);
```

### Migration 006: Espaços/Consultórios
```sql
CREATE TABLE IF NOT EXISTS therapy_spaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    address TEXT DEFAULT '',
    type TEXT NOT NULL DEFAULT 'fixed',  -- 'fixed', 'rented', 'temporary'
    cost_cents_per_use INTEGER DEFAULT 0,
    cost_cents_monthly INTEGER DEFAULT 0,
    is_available INTEGER DEFAULT 1,
    notes TEXT DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS space_bookings (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL REFERENCES therapy_spaces(id),
    appointment_id TEXT REFERENCES appointments(id),
    booking_date TEXT NOT NULL,
    start_time TEXT NOT NULL,
    end_time TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_space_bookings_date ON space_bookings(booking_date);
CREATE INDEX IF NOT EXISTS idx_space_bookings_space ON space_bookings(space_id);
```

---

## 6. Ordem de Implementação Recomendada

### Fase 1: Migração arquitetural (eliminar React)
1. Criar sistema de templates Go + CSS design system
2. Implementar layout (sidebar, header, main area)
3. Migrar Dashboard (agenda) com htmx
4. Migrar Pacientes (CRUD + link para detalhes)
5. Migrar Appointments + Session Notes
6. Migrar Anamnese + Contratos
7. Migrar Finance + Settings
8. Migrar área do Paciente (login, dashboard, forms)
9. Remover `frontend/` inteiramente

### Fase 2: Novas features
10. Implementar Supervisões (backend + frontend)
11. Implementar Espaços (backend + frontend)
12. Implementar Relatórios consolidados + export

### Fase 3: Segurança e produção
13. Headers de segurança
14. Rate limiting
15. Validação de uploads
16. Fix race condition no DBCalendar
17. Fix StaffAuth em produção
18. Deploy Pangolin real

### Fase 4: Polish
19. Responsividade mobile completa
20. Página 360° do paciente
21. Agenda visual como dashboard principal
22. Dark mode (opcional)

---

## 7. Resumo Executivo

### Veredicto geral: código funcional com bugs pontuais e dívida técnica gerenciável

**Pontos fortes**:
- Arquitetura em camadas bem separada e consistente
- Testes existentes e passando (domain, service, storage, web)
- Contrato JSON snake_case respeitado em toda a API
- Google Calendar integration com fallback gracioso
- Migrations SQL idempotentes e corretas
- Dev mode funcional com seed

**Ação principal recomendada**:
- Eliminar React/npm — simplifica build, deploy e manutenção
- Corrigir 2 bugs de severidade alta (StaffAuth + race condition DBCalendar)
- Implementar headers de segurança (trivial, 5 linhas)
- Adicionar features E4 (supervisões) e E5 (espaços) que são as maiores lacunas funcionais

**Esforço estimado total**: ~40-60h de trabalho para migrar frontend + implementar features faltantes + corrigir bugs.
