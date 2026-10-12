# Psicoman — Registro de Decisões (ADR leve)

Lastro das decisões relevantes de produto e arquitetura. Formato: data, contexto, decisão, consequências. Política em [`.kiro/steering/psicoman-processo-entrega.md`](../.kiro/steering/psicoman-processo-entrega.md).

Ordem: mais recente no topo.

---

## 2026-10-12 — Onda "prontuario-ux": prontuário unificado, edição de sessão, WhatsApp, paleta e calendário fluido

**Contexto.** Pedido de produto do terapeuta focado em usabilidade do acompanhamento clínico e identidade visual. O detalhe do paciente fragmentava o conteúdo clínico em abas separadas (Anamnese, Sessões, Notas) e não permitia abrir uma sessão para editá-la nem registrar o relatório daquela sessão; o telefone era texto puro (sem link e sem padronização); o tema branco+verde/teal soava genérico; e a agenda semanal não acomodava bem um dia com ~10 pacientes. Detalhado na spec [`prontuario-ux`](../.kiro/specs/prontuario-ux/requirements.md).

**Decisões.**
1. **Prontuário unificado.** O detalhe do paciente ganha uma aba única "Prontuário" (timeline por data: sessões com seus registros + notas livres), substituindo as abas "Sessões" e "Notas". A "Anamnese" permanece separada (documento único). O ato de agendar continua na Agenda; o Prontuário trata do registro clínico. Reusa `domain.Note.SessionID` (já existente). Spec R1, R2.4.
2. **Edição de sessão (`PUT /v1/admin/sessions/{id}`).** Nova operação para editar data/hora, local e modalidade. Não troca o paciente. Bloqueada em estados terminais (realizada/cancelada/falta); auditada. A persistência (`SessionRepo.Update`) já cobria esses campos — sem mudança de schema. Spec R2.2.
3. **Edição de sessão NÃO reconcilia o Google Calendar nesta onda.** O `SessionCalendar` atual só expõe `CreateEvent`; implementar `UpdateEvent`/remoção remota amplia escopo e risco. A edição altera o registro local e a UI avisa o terapeuta para ajustar o convite no Google manualmente quando o horário mudar. `UpdateEvent` fica como melhoria futura.
4. **Registros vinculados à sessão.** A UI passa a enviar `session_id` ao criar nota a partir de uma sessão (contrato já existente em `POST /v1/admin/patients/{id}/notes`), correlacionando relatório ↔ sessão no Prontuário. Spec R2.3.
5. **Normalização de telefone no domínio.** `domain.NormalizePhone` canoniza para só dígitos com DDI `55`, aplicada no serviço de paciente (admin e portal), espelhando `NormalizeCPF`. O telefone vira link `https://wa.me/<numero>` (WhatsApp Web) no admin. Decisão: normalizar na entrada (não só a massa de testes), para o dado chegar correto. Spec R3.
6. **Nova paleta.** Azul royal como primária e lilás como acento, sobre fundos claros derivados e branco; cores de estado (ok/atenção/perigo) preservadas. Troca centralizada nos tokens `:root` de `app.css`. Spec R4.
7. **Calendário semanal fluido (dia cresce com a lotação), não grade por horas.** Escolhido o modelo em que a coluna do dia cresce conforme o número de sessões (fluido para ~10/dia), mantendo seg–sáb (sem domingo). A grade por eixo de horas (estilo Google com linha do tempo vertical) fica fora de escopo por custo/complexidade, dado o público leigo. Spec R5.

**Consequências.**
- Novo endpoint `PUT /v1/admin/sessions/{id}` (Swagger a atualizar); `sessionView` inalterado.
- `domain.NormalizePhone` + validação de plausibilidade do telefone; valor persistido de `phone` passa a ser canônico (comportamento, não shape).
- UI do admin: aba "Prontuário" (remove "Sessões"/"Notas"), painel de sessão com edição + registros, telefone como link WhatsApp, nova paleta, semana fluida.
- `scripts/seed.sh` passa a criar várias sessões no mesmo dia (exercita calendário lotado e links).
- Esta onda integra o mesmo PR grande para `main` que a `mvp-audit1` (push ao final, confirmado com o Fino).
- Execução planejada em fases (A–F) com commits por fase; ver [`tasks.md`](../.kiro/specs/prontuario-ux/tasks.md).

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
1. **Gate de aprovação de paciente (segurança).** Todo paciente que se auto-cadastra pelo portal nasce `pendente` e não acessa nenhum recurso até o terapeuta aprovar. Enforcement no servidor (middleware `ApprovalGate`, HTTP 403), não só na UI. Pacientes cadastrados pelo terapeuta nascem `aprovado`; vínculo por email não rebaixa quem já é aprovado. Detalhado na spec [`mvp-audit1`](../.kiro/specs/mvp-audit1/requirements.md) R1.
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
