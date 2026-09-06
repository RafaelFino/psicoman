# mvp-audit1 — Plano de Implementação (Tasks)

Execução incremental. Cada task é demonstrável e testável, apoia-se nas anteriores e integra ao todo (sem código órfão). Refs apontam para [`requirements.md`](./requirements.md), [`design.md`](./design.md) e a fonte durável [`docs/requirements.md`](../../../docs/requirements.md).

Convenções:
- `[ ]` pendente · `[~]` em progresso · `[x]` concluído.
- Cada task só é concluída com **build verde**, **testes verdes** e **demo funcional** (smoke via HTTP e/ou tela).
- Toda mensagem ao usuário em PT-BR; sem `window.prompt/confirm`; WCAG AA nos controles novos.
- Backend imposto no servidor (defense in depth); UI apenas reflete.

**Definição de pronto (obrigatória em toda task — ver [`.kiro/steering/psicoman-processo-entrega.md`](../../steering/psicoman-processo-entrega.md)):**
- **Docs junto do código**: toda mudança de comportamento atualiza, no mesmo ciclo, `docs/requirements.md` (o quê), `docs/architecture.md` (como), `README.md` (se muda uso), Swagger (se muda contrato) e `docs/decisions.md` (se houve decisão relevante). Nunca "documentar depois".
- **Testes cobrem a mudança**: regra nova → unit no `service`; fluxo/contrato novo → E2E; segurança/acesso → E2E do caminho permitido **e** negado; UI → smoke de parse + verificação registrada; lógica de UI (dinheiro, markdown) → teste onde couber.
- **Git por fase**: trabalhar numa **branch dedicada da onda** (`mvp-audit1`). Ao concluir cada **Fase**, `make check` verde → **commit** em PT-BR referenciando a fase. **Push só no fim da onda** (após a Fase E verde). Ver "Fechamento de fase" abaixo.

---

## Fase A — Segurança de acesso (R1)

- [x] **A1. Domínio + persistência do estado de aprovação** (dep: —)
  - `domain.Patient.ApprovalStatus` + constantes (`pendente`/`aprovado`/`rejeitado`); helpers `IsApproved`, `CanTransitionApproval`; `Validate` aceita só estados válidos.
  - Migration aditiva: coluna `approval_status` default `pendente` + `UPDATE … 'aprovado'` para os pré-existentes.
  - Repositório SQLite: coluna em insert/select/update; `ListByApproval`.
  - **Aceite:** boot aplica migration em base existente sem erro; pré-existentes ficam `aprovado`.
  - **Testes:** unit de transição/validação; migration em base com e sem dados.
  - **Refs:** R1.1 · D1.1–D1.2.

- [x] **A2. Serviço de aprovação** (dep: A1)
  - `PatientService`: `RegisterFromPortal` → `pendente` (mantém `aprovado` no vínculo por email); `Create` (admin) → `aprovado`; `Approve`/`Reject` com transição; `ListPending`.
  - **Aceite:** auto-cadastro nasce pendente; cadastro admin nasce aprovado; approve/reject mudam estado.
  - **Testes:** unit dos quatro caminhos + vínculo por email não rebaixa.
  - **Refs:** R1.1, R1.4 · D1.3.

- [x] **A3. Enforcement no portal (403 até aprovar)** (dep: A2)
  - `httpx.ErrForbidden` (se ausente); middleware `RequireApproved`; reorganizar `RegisterAuthenticated` (liberadas vs. atrás do gate); endpoint `GET /v1/portal/approval-status`.
  - **Aceite:** paciente pendente recebe 403 nas rotas de recurso e 200 em `/me` e `/approval-status`; aprovado acessa tudo.
  - **Testes:** E2E portal — pendente (403) e aprovado (200); ajustar fixtures existentes.
  - **Refs:** R1.2 · D1.4.

- [x] **A4. Fila de aprovação no admin (API)** (dep: A2)
  - `GET /v1/admin/patients/pending`, `POST /patients/{id}/approve`, `POST /patients/{id}/reject` (+ audit log); `patientView` inclui `approval_status`.
  - **Aceite:** endpoints respondem; aprovar/rejeitar gera audit.
  - **Testes:** E2E admin dos três endpoints + verificação de audit.
  - **Refs:** R1.4 · D1.5.

- [x] **A5. UI — página de status do paciente pendente** (dep: A3)
  - `portal_app.html`: ramificar por `approval-status`; cards "em análise" e "não liberado"; botões Atualizar/Sair.
  - **Aceite:** login de pendente mostra a página de análise; aprovado mostra a área normal; rejeitado mostra o aviso.
  - **Testes:** smoke manual (dev) dos três estados.
  - **Refs:** R1.3 · D1.6.

- [x] **A6. UI — fila de aprovação no admin** (dep: A4)
  - Seção `#aprovacoes` + `approvalsPanel()` com aprovar/rejeitar por **confirmação inline**; badge de contagem; item no menu.
  - **Aceite:** terapeuta aprova/rejeita pela tela; contagem atualiza; paciente liberado imediatamente.
  - **Testes:** smoke manual; verificação do fluxo ponta a ponta com A5.
  - **Refs:** R1.4, R10.2 · D1.7.

- [x] **A7. Fechamento da Fase A** (dep: A1–A6)
  - Atualizar docs: `docs/requirements.md` (novo requisito de aprovação), `docs/architecture.md` (gate/endpoints/migration), Swagger (novos endpoints), `docs/decisions.md` (já iniciado — revisar/confirmar).
  - `make check` verde (build + vet + unit + E2E). **Commit** PT-BR: "mvp-audit1 Fase A: gate de aprovação de paciente". **Sem push.**
  - **Aceite:** docs coerentes com o implementado; suíte verde; commit criado.
  - **Refs:** processo de entrega (steering).

## Fase B — Destravar o núcleo (R2–R5)

- [x] **B1. Util de dinheiro + helper de data** (dep: —)
  - `Money.toCents/fromCents` e `Fmt.short(iso)` em `api.js`.
  - **Aceite:** conversões corretas em casos limpos e com vírgula/ponto; `short` gera "Seg, 08/09 · 14:00".
  - **Testes:** smoke manual no console/preview.
  - **Refs:** R2, R10.4 · D2.1, D3.4.

- [x] **B2. Locais + disponibilidade (UI)** (dep: B1)
  - Seção `#locais` + `locationsPanel()`: CRUD de locais e disponibilidade (dias PT-BR, capacidade), valores em reais.
  - **Aceite:** criar/editar/remover local; add/remove janela; agenda aberta do portal passa a listar as janelas.
  - **Testes:** smoke HTTP + verificação no portal (availability).
  - **Refs:** R2 · D2.1.

- [x] **B3. Origens + campo origem no paciente (UI)** (dep: —)
  - Seção `#origens` + `originsPanel()`; `<select>` de origem no cadastro e na edição do paciente.
  - **Aceite:** origem selecionável persiste em `origin_id`; sem origens, UI orienta cadastro.
  - **Testes:** smoke HTTP (paciente com origem) .
  - **Refs:** R3 · D2.2.

- [x] **B4. Planos por paciente (UI)** (dep: —)
  - Aba "Plano" no `patientDetail`: listar/criar/remover; tipos com rótulos e explicação; valor/vigência para tipos fixos.
  - **Aceite:** criar cada tipo de plano; remover; textos explicativos visíveis.
  - **Testes:** smoke HTTP de criação por tipo.
  - **Refs:** R4 · D2.3.

- [x] **B5. Agendar sessão pela agenda (UI)** (dep: B2, B3)
  - Botão "Nova sessão" (global e por dia) com formulário inline (paciente aprovado, local, modalidade, início/fim); trata conflito.
  - **Aceite:** cria sessão que aparece na semana; conflito exibe mensagem PT-BR.
  - **Testes:** smoke HTTP (criação) + verificação na grade.
  - **Refs:** R5 · D2.4.

- [x] **B6. Fechamento da Fase B** (dep: B1–B5)
  - Atualizar docs (requisitos/arquitetura/README/Swagger conforme o que mudou) e `docs/decisions.md` se houve decisão.
  - `make check` verde. **Commit** PT-BR: "mvp-audit1 Fase B: locais, origem, planos e agendamento direto". **Sem push.**
  - **Refs:** processo de entrega (steering).

## Fase C — Usabilidade do portal e ações (R10.1, R10.2, R10.3)

- [x] **C1. Portal sem JSON** (dep: A5)
  - Substituir `<pre>` de sessões/débitos por listas legíveis (data, status PT-BR, valor).
  - **Aceite:** portal do aprovado mostra listas amigáveis, sem JSON.
  - **Testes:** smoke manual no portal.
  - **Refs:** R10.1 · D3.1.

- [x] **C2. Fim de prompt/confirm + pagamento inline** (dep: —)
  - Pagamento com formulário inline (valor + forma); exclusões com confirmação inline em todas as telas.
  - **Aceite:** nenhum `window.prompt/confirm` remanescente; pagamento e exclusões funcionam por UI.
  - **Testes:** grep garante ausência de prompt/confirm; smoke dos fluxos.
  - **Refs:** R10.2 · D3.2.

- [x] **C3. Copy do login do portal** (dep: —)
  - Textos coerentes com o `IdentityVerifier` real; sem promessa de fluxo inexistente.
  - **Aceite:** login descreve com honestidade o que faz.
  - **Testes:** revisão visual.
  - **Refs:** R10.3 · D3.3.

- [x] **C4. Fechamento da Fase C** (dep: C1–C3)
  - Atualizar docs (`README.md`/arquitetura sobre o portal) e `docs/decisions.md` se aplicável.
  - `make check` verde. **Commit** PT-BR: "mvp-audit1 Fase C: portal legível e ações inline". **Sem push.**
  - **Refs:** processo de entrega (steering).

## Fase D — Gestão e operação (R6–R9)

- [ ] **D1. Custos + relatórios (UI)** (dep: B1)
  - Seção de custos (categorias/itens) + relatórios de custo e ROI com seletor de período, legíveis.
  - **Aceite:** cadastrar categoria/item; ver relatório de custos e ROI do período.
  - **Testes:** smoke HTTP dos endpoints + render.
  - **Refs:** R6 · D2.5.

- [ ] **D2. Perfil do terapeuta (UI)** (dep: B2)
  - Seção de perfil: dados + bio Markdown + foto + locais + links de plataforma.
  - **Aceite:** salvar perfil, subir foto, adicionar/remover link.
  - **Testes:** smoke HTTP (profile, photo, links).
  - **Refs:** R7 · D2.6.

- [ ] **D3. Backup/Restore (UI)** (dep: —)
  - Seção de backup: gerar backup (mostra resultado); restaurar com dupla confirmação + aviso de reinício; audit.
  - **Aceite:** backup executa e mostra resultado; restore exige dupla confirmação.
  - **Testes:** smoke HTTP do backup (restore validado com cuidado em dev).
  - **Refs:** R8 · D2.7.

- [ ] **D4. Auditoria (UI)** (dep: —)
  - Seção de auditoria: tabela legível de `GET /audit-log`.
  - **Aceite:** entradas recentes visíveis e legíveis.
  - **Testes:** smoke HTTP.
  - **Refs:** R9 · D2.8.

- [ ] **D5. Fechamento da Fase D** (dep: D1–D4)
  - Atualizar docs (requisitos/arquitetura/Swagger conforme telas de gestão) e `docs/decisions.md` se houve decisão.
  - `make check` verde. **Commit** PT-BR: "mvp-audit1 Fase D: custos/ROI, perfil, backup e auditoria". **Sem push.**
  - **Refs:** processo de entrega (steering).

## Fase E — Fechamento (R10.4, R10.5)

- [ ] **E1. Navegação e acessibilidade final** (dep: todas de UI)
  - Menu do admin agrupado com todas as áreas; datas curtas nas listas; revisão de teclado/foco/labels nos controles novos.
  - **Aceite:** todas as telas alcançáveis pelo menu; navegação por teclado funcional.
  - **Testes:** revisão manual de acessibilidade; smoke geral.
  - **Refs:** R10.4, R10.5 · D3.4, D3.5.

- [ ] **E2. Regressão, docs finais e fechamento da onda** (dep: todas)
  - `make check` (build + fmt + vet + unit + E2E) verde; parse dos templates; limpeza de artefatos temporários.
  - Revisão final de coerência: `docs/requirements.md`, `docs/architecture.md`, `README.md`, Swagger e `docs/decisions.md` refletindo toda a onda.
  - Atualizar `docs/release-notes.md` com a entrada visual da onda `mvp-audit1` (Novidades/Melhorias/Correções/Notas de migração).
  - **Commit** de fechamento (Fase E) e, com a onda inteira verde, **`push` único** a partir da branch da onda `mvp-audit1` (sem push direto na main).
  - **Aceite:** suíte completa verde; documentação coerente; histórico com commits por fase; push realizado uma única vez ao final.
  - **Testes:** `go test ./...`.
  - **Refs:** `docs/requirements.md §4.3` · processo de entrega (steering).

---

## Rastreabilidade (requisito → task)

| Requisito | Tasks |
|-----------|-------|
| R1 Aprovação | A1, A2, A3, A4, A5, A6 |
| R2 Locais | B1, B2 |
| R3 Origem | B3 |
| R4 Planos | B4 |
| R5 Agendar sessão | B5 |
| R6 Custos/ROI | D1 |
| R7 Perfil terapeuta | D2 |
| R8 Backup/Restore | D3 |
| R9 Auditoria | D4 |
| R10 Usabilidade | B1, C1, C2, C3, E1 |
