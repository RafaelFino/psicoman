---
inclusion: fileMatch
fileMatchPattern: "frontend/src/**"
---

# Frontend — Padrões e Boas Práticas

## Estrutura de pastas

```
frontend/src/
├── api.js              # Todas as chamadas HTTP centralizadas
├── App.jsx             # Rotas (React Router)
├── main.jsx            # Entry point
├── styles.css          # CSS global (variáveis + utilitários)
├── components/         # Componentes reutilizáveis (a criar)
├── patient/            # Telas do paciente
└── psych/              # Telas do psicólogo
```

## Convenções de componentes

- Um componente por arquivo, export default
- Nome do arquivo = nome do componente (PascalCase)
- State management: useState/useEffect locais (sem Redux/Context global por enquanto)
- Data fetching: direto no componente via `api.js`

## Responsividade (mobile-first)

- Usar CSS Grid/Flexbox para layouts
- Breakpoint principal: `768px` (tablet/mobile vs desktop)
- Todos os elementos interativos devem ter `min-height: 44px` (touch target)
- Tabelas: usar layout responsivo com cards em mobile (media queries)
- Testar com viewport 375px (iPhone SE) como base mínima

## Padrão de fetch e loading

```jsx
const [data, setData] = useState(null)
const [loading, setLoading] = useState(true)
const [error, setError] = useState('')

useEffect(() => {
  api.endpoint()
    .then(setData)
    .catch(e => setError(e.message))
    .finally(() => setLoading(false))
}, [])

if (loading) return <Spinner />
if (error) return <ErrorMessage message={error} />
```

## Navegação e links

- Nomes de pacientes devem ser links clicáveis para `/psych/patients/:id`
- Usar `<Link>` do React Router (nunca `<a href>` para rotas internas)
- Breadcrumbs em páginas profundas

## Formulários

- Labels sempre associados a inputs (acessibilidade)
- Validação client-side básica (required, type) + feedback de erro do servidor
- Loading state nos botões de submit (evitar double-submit)
- `<form onSubmit>` com `preventDefault` (nunca submit nativo)

## Internacionalização

- Textos fixos em português brasileiro
- Datas: `toLocaleString('pt-BR')` ou `toLocaleDateString('pt-BR')`
- Moeda: `(cents / 100).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })`
- Sem biblioteca de i18n — sistema é mono-idioma

## Acessibilidade mínima

- Todos os inputs com `<label>` associado
- Contraste mínimo WCAG AA (4.5:1 para texto normal)
- Aria-labels em botões de ícone
- Focus visible em elementos interativos
- Semântica HTML: `<header>`, `<main>`, `<nav>`, `<table>` (não `<div>` para tudo)

## Estilo visual

- Paleta: tons leves, fundo claro (`#f8fafc`), cards brancos
- Tipografia: system-ui (sem fontes externas para performance)
- Bordas suaves: `border-radius: 8-12px`
- Espaçamento consistente: múltiplos de 0.25rem
- Cores semânticas via CSS variables (--primary, --danger, --success, --muted)
- Adequado para uso diário profissional: limpo, sem distrações

## Dependências frontend

- **Mínimas**: React, React Router, Vite — sem mais nada
- Não adicionar bibliotecas de componentes (MUI, Chakra, etc.) sem discussão
- Se precisar de ícone, usar SVG inline ou emoji
- Se precisar de rich text editor, avaliar Tiptap ou similar (leve)
