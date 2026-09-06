# Psicoman — Registro de Decisões (ADR leve)

Lastro das decisões relevantes de produto e arquitetura. Formato: data, contexto, decisão, consequências. Política em [`.kiro/steering/psicoman-processo-entrega.md`](../.kiro/steering/psicoman-processo-entrega.md).

Ordem: mais recente no topo.

---

## 2026-09-06 — Padrão de documentação e política de branch por onda

**Contexto.** Os documentos estavam com nomes inconsistentes (`DEPLOY.md`, `DECISIONS.md` em maiúsculas; `requirements.md`/`architecture.md` em minúsculas) e espalhados entre a raiz e `docs/`. Faltava também um lugar para notas de versão legíveis e uma política clara de branch para trabalhos longos.

**Decisões.**
1. **Nomenclatura única de docs**: raiz contém apenas `README.md`; toda documentação técnica em `docs/` com nomes em `kebab-case` minúsculo. Movidos `DEPLOY.md` → `docs/deploy.md` e `DECISIONS.md` → `docs/decisions.md`.
2. **Release notes**: novo `docs/release-notes.md`, visual e orientado ao usuário, atualizado no fechamento de cada onda.
3. **Branch por onda de specs**: uma branch dedicada por conjunto grande de trabalho (não por task), com commits por fase e push único ao final.

**Consequências.** Links atualizados em `README.md`, `Makefile`, `scripts/deploy.sh` e nas specs. Política registrada em [`.kiro/steering/psicoman-processo-entrega.md`](../.kiro/steering/psicoman-processo-entrega.md).

---

## 2026-09-06 — Onda "mvp-audit1": auditoria de interface e segurança de acesso

**Contexto.** Após o MVP, uma auditoria de **requisitos × interface** revelou que várias funcionalidades existiam no backend sem tela usável (locais, origens, planos, custos/ROI, perfil do terapeuta, backup, auditoria) e que o portal do paciente expunha JSON cru e usava diálogos nativos do navegador — ruim para o público-alvo (usuários leigos). Além disso, o portal fica exposto na internet atrás do Pangolin sem controle de acesso, dependendo apenas do login social.

**Decisões.**
1. **Gate de aprovação de paciente (segurança).** Todo paciente que se auto-cadastra pelo portal nasce `pendente` e não acessa nenhum recurso até o terapeuta aprovar. Enforcement no servidor (middleware `RequireApproved`, HTTP 403), não só na UI. Pacientes cadastrados pelo terapeuta nascem `aprovado`; vínculo por email não rebaixa quem já é aprovado. Detalhado na spec [`mvp-audit1`](../.kiro/specs/mvp-audit1/requirements.md) R1.
2. **Fechar os gaps de UI** identificados (locais, origem, planos, agendar sessão direto, custos/ROI, perfil, backup/restore, auditoria) — spec `mvp-audit1` R2–R9.
3. **Usabilidade para leigos** como requisito de primeira classe: portal sem JSON, fim de `window.prompt/confirm`, copy de login coerente, navegação clara, datas curtas, WCAG AA — spec `mvp-audit1` R10.
4. **Processo de entrega** formalizado em steering: toda mudança atualiza docs + testes no mesmo ciclo; commit ao fim de cada grupo de tasks e push só no fim da onda; decisões relevantes registradas aqui.

**Consequências.**
- Migration aditiva de `approval_status` em `patients` (pré-existentes → `aprovado`, idempotente).
- Novos endpoints: `GET /v1/portal/approval-status`; `GET /v1/admin/patients/pending`, `POST /v1/admin/patients/{id}/approve|reject`; `patientView` ganha `approval_status`.
- A suíte E2E do portal passa a exigir paciente aprovado nas rotas de recurso (ajuste de fixtures + casos 403/200).
- Execução planejada em fases (A–E) com commits por fase.

---

## 2026-09-06 — Telas de Agenda e Detalhe do Paciente no admin

**Contexto.** O painel admin só tinha lista/cadastro de pacientes, pedidos de agendamento e resumo financeiro. Clicar num paciente não abria nada (o item não tinha ação) e não existia visão de agenda — sendo essas as duas telas mais importantes para o dia a dia do terapeuta.

**Decisão.** Implementar, em SSR + Alpine (sem SPA, conforme `architecture.md §9`):
- **Agenda semanal** em destaque, de **segunda a sábado** (o terapeuta não atende domingo), com navegação por semana e sessões clicáveis. Consome `GET /v1/admin/sessions`.
- **Detalhe do paciente** com abas (dados, anamnese, sessões, notas, débitos/pagamentos/recibos, custos, arquivos), consumindo os endpoints já existentes por paciente.
- Store Alpine `nav` controla o paciente aberto; navegação por âncora mantida.

**Consequências.** Nenhuma mudança de backend; só UI (`internal/web`). Fluxo validado ponta a ponta em modo dev.

---

## 2026-09-06 — Markdown nos campos de texto e alinhamento cliente/servidor

**Contexto.** Requisito de que campos de texto livre (anamnese, notas, templates) sejam escritos em Markdown e exibidos formatados. O render do servidor (`internal/platform/markdown`) suportava só cabeçalhos, listas, negrito e itálico; o preview no admin precisava bater com o e-mail efetivamente enviado ao paciente.

**Decisão.** **Alinhar por cima**: estender o Markdown do servidor para também suportar **código inline**, **blocos de código cercados por ```** e **links** (apenas `http`/`https`/`mailto`, bloqueando `javascript:`), espelhado por um render client-side (`Md.toHtml` em `api.js`) com o mesmo subconjunto e escape anti-XSS. Editores no admin ganham alternância "Escrever/Visualizar".

**Consequências.** Preview no admin é fiel ao enviado. `internal/platform/markdown` ganhou testes para os novos casos (incluindo rejeição de `javascript:`). O subconjunto suportado é deliberadamente mínimo e deve ser mantido em paridade entre servidor e cliente.
