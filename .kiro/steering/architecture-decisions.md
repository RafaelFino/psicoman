---
inclusion: auto
---

# Decisões Arquiteturais — Psicoman

## Frontend: Go templates + htmx + Alpine.js

### Stack de renderização

| Componente | Uso | Arquivo/embed |
|------------|-----|---------------|
| Go `html/template` | Renderização server-side de todas as páginas | `internal/web/templates/` |
| htmx 2.x | AJAX parcial (criar, editar, deletar sem full reload) | embed via `static/js/htmx.min.js` |
| Alpine.js 3.x | Estado client-side local (modais, tabs, dropdowns) | embed via `static/js/alpine.min.js` |
| CSS custom | Design system completo, variáveis, responsivo | `static/css/styles.css` |

### Estrutura de templates

```
internal/web/
├── templates/
│   ├── layouts/
│   │   ├── base.html          -- HTML head, scripts, CSS
│   │   ├── psych.html         -- Layout psicólogo (sidebar + header + main)
│   │   └── patient.html       -- Layout paciente (header + main)
│   ├── psych/
│   │   ├── dashboard.html
│   │   ├── patients.html
│   │   ├── patient_detail.html
│   │   ├── appointments.html
│   │   ├── session_notes.html
│   │   ├── anamnesis.html
│   │   ├── contracts.html
│   │   ├── finance.html
│   │   ├── supervisions.html
│   │   ├── spaces.html
│   │   └── settings.html
│   ├── patient/
│   │   ├── login.html
│   │   ├── dashboard.html
│   │   ├── book.html
│   │   ├── anamnesis.html
│   │   ├── contracts.html
│   │   └── documents.html
│   └── partials/
│       ├── appointment_row.html   -- htmx swap targets
│       ├── patient_card.html
│       ├── session_note_form.html
│       └── toast.html
├── static/
│   ├── css/styles.css
│   ├── js/htmx.min.js
│   └── js/alpine.min.js
```

### Build tags

- Templates sempre embutidos via `//go:embed templates/* static/*`
- Único binário, sem Node.js, sem npm

### Padrões htmx

```html
<!-- Criar recurso via POST, swap na tabela -->
<form hx-post="/api/psych/patients" hx-target="#patient-list" hx-swap="beforeend">
  <input name="name" required>
  <button type="submit">Salvar</button>
</form>

<!-- Deletar com confirmação -->
<button hx-delete="/api/psych/patients/{{.ID}}" 
        hx-confirm="Excluir paciente?" 
        hx-target="closest tr" 
        hx-swap="outerHTML">
  Excluir
</button>
```

### Padrões Alpine.js

```html
<!-- Tabs -->
<div x-data="{ tab: 'templates' }">
  <button @click="tab = 'templates'" :class="tab === 'templates' ? 'active' : ''">Templates</button>
  <button @click="tab = 'responses'" :class="tab === 'responses' ? 'active' : ''">Respostas</button>
  <div x-show="tab === 'templates'">...</div>
  <div x-show="tab === 'responses'">...</div>
</div>

<!-- Modal -->
<div x-data="{ open: false }">
  <button @click="open = true">Abrir</button>
  <div x-show="open" @click.outside="open = false" class="modal">...</div>
</div>
```

## API: mantém REST JSON para endpoints de dados

- Endpoints `/api/psych/*` e `/api/patient/*` continuam retornando JSON
- Novas rotas de renderização: `/psych/*` retornam HTML (templates)
- htmx pode consumir tanto JSON (com `hx-ext="json-enc"`) quanto HTML fragments
- Para operações CRUD via htmx: retornar HTML partial (fragmento da lista atualizada)

## Autenticação: sem mudanças

- Psicólogo: Pangolin headers (produção) / X-Dev-Auth (dev)
- Paciente: Google OAuth → JWT → cookie ou header
- Middleware existente não muda

## Deploy: simplificado

- Dockerfile de 1 stage builder (sem Node)
- Binário final: ~20MB
- Zero dependências externas em runtime


## Decisões tomadas neste projeto (registro)

### 1. Eliminação completa do React/npm/Node.js
- **Contexto**: O projeto iniciou com React 18 + Vite como SPA separado
- **Decisão**: Migrar para Go html/template + htmx + Alpine.js
- **Razão**: Sistema usado por 1 psicólogo, complexidade desnecessária, 2 linguagens, build lento, node_modules pesado
- **Status**: ✅ Implementado e React removido do repositório

### 2. Tema escuro (dark mode)
- **Decisão**: Suportar tema claro e escuro
- **Implementação**: CSS variables redefinidas em `[data-theme="dark"]` + `@media (prefers-color-scheme: dark)`
- **Toggle**: Botão no header com persistência em localStorage
- **Cobertura**: Todas as páginas (psicólogo e paciente)

### 3. Visão semanal na agenda
- **Decisão**: Dashboard do psicólogo tem tabs "Hoje" e "Semana"
- **Implementação**: Grid de 7 colunas com dados do WeekDay struct, renderizado via Go template
- **UX**: Dia atual destacado, consultas com cores por status

### 4. Calendário mensal na página do paciente
- **Decisão**: Página 360° do paciente inclui calendário mensal interativo
- **Implementação**: htmx para navegação entre meses (partial render), Alpine.js para modais
- **Interação**: Clicar em dia vazio → criar consulta; clicar em consulta → editar

### 5. Infraestrutura de dados de teste
- **Decisão**: Prefixo "TEST " em nomes + @test.com em emails para identificação
- **Seed**: Script bash `scripts/seed-test-data.sh` (3 pacientes + 12 consultas)
- **Cleanup**: `DELETE /api/dev/test-data` com cascata (remove tudo associado)
- **Isolamento**: Endpoint só funciona com DEV_MODE=true + X-Dev-Auth válido

### 6. Sem retrocompatibilidade (pre-1.0)
- **Decisão**: Projeto está em pre-release, não temos compromisso com APIs ou schemas anteriores
- **Implicação**: Podemos remover código legado, mudar schemas, renomear endpoints livremente
- **Regra**: Ao encontrar código legado ou redundante, remover imediatamente

### 7. Diagramas em Mermaid
- **Decisão**: Todos os diagramas em documentação devem usar sintaxe Mermaid
- **Razão**: ASCII art é difícil de manter, fica ilegível, e não renderiza bem em todas as fontes
- **Onde**: README.md, docs/, steering files quando necessário
- **Tipos**: flowchart (arch), erDiagram (MER), sequenceDiagram (fluxos), gantt (roadmap)

### 8. Documentação estruturada
- **release-notes.md**: O que está pronto (substitui AUDIT.md para features concluídas)
- **next-steps.md**: O que falta (substitui AUDIT.md para roadmap)
- **testing.md**: Como testar o sistema
- **Sem AUDIT.md**: Documento eliminado — informações redistribuídas

### 9. Security headers implementados
- **Decisão**: Middleware `SecurityHeaders()` adicionado ao router
- **Headers**: X-Content-Type-Options: nosniff, X-Frame-Options: DENY, Referrer-Policy, X-XSS-Protection
- **Status**: ✅ Implementado
