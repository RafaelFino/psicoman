---
inclusion: fileMatch
fileMatchPattern: "**/web/**,**/*.html,**/*.css,**/*.tmpl"
---

# Psicoman — Front-end e UX

Convenções da interface web. Princípios de UX em `docs/requirements.md §4.5`; stack em `docs/architecture.md §9`.

## Stack

- **Server-side rendering** com templates Go (`html/template`), sem SPA e sem build step de JS pesado.
- **CSS**: framework leve (Pico.css ou Tailwind via CDN/build simples) — decisão a fixar na Task 20, mas em qualquer caso: sem framework JS pesado (React/Vue/Angular).
- **JS**: apenas o necessário via htmx (requisições parciais) e/ou Alpine.js (interatividade local). Nada de bundlers complexos (Webpack/Vite) a menos que a Task 20 justifique explicitamente.
- Assets embutidos no binário via `embed.FS` — nada de servir estático de fora do binário em produção.

## Responsividade (mobile-first)

- Escrever CSS mobile-first: estilos base para telas pequenas, `@media (min-width: ...)` para expandir em telas maiores. Nunca o inverso.
- `psicoman-portal`: layout linear de coluna única mesmo em desktop — não adicionar navegação lateral ou densidade de admin nesse binário.
- `psicoman-admin`: navegação lateral colapsa para menu inferior/hambúrguer abaixo do breakpoint mobile.
- Testar todo componente novo em pelo menos dois breakpoints (mobile ~375px, desktop ~1280px) antes de considerar pronto.

## Acessibilidade (WCAG AA)

- Contraste mínimo AA (4.5:1 texto normal, 3:1 texto grande) — validar cor de texto sobre fundo antes de fixar na paleta.
- Todo elemento interativo (botão, link, campo) precisa de foco visível (não remover `outline` sem substituir por alternativa visível).
- Todo `<input>`/`<select>`/`<textarea>` tem `<label>` associado (via `for`/`id`, não apenas `placeholder`).
- Navegação por teclado funcional em todo fluxo (Tab/Enter/Esc) — testar sem mouse os fluxos críticos (login, agendar, pagar).
- Imagens/ícones informativos com `alt` ou `aria-label`; ícones puramente decorativos com `aria-hidden="true"`.

## Tom e conteúdo

- Todo texto de interface em **PT-BR**, direto, sem jargão técnico. Mensagens de erro explicam o que aconteceu e o que fazer, nunca só um código.
- Paleta suave/acolhedora (evitar vermelho puro para erro — preferir tons que comuniquem atenção sem agressividade; ver `docs/requirements.md §4.5`).
- Feedback de toda ação assíncrona (loading, sucesso, erro) visível — nenhuma ação do usuário fica sem resposta visual.
