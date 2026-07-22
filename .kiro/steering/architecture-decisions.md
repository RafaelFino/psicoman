---
inclusion: auto
---

# Decisões Arquiteturais — Psicoman

## Frontend: Go templates + htmx + Alpine.js (substituindo React)

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

- **Sem React**: eliminar `embedfrontend` build tag
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
- Binário final: ~20MB (vs ~25MB com React bundle)
- Zero dependências externas em runtime
