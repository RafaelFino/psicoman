# mvp-audit1 — Requisitos

Segundo ciclo de implementação do Psicoman, derivado de uma auditoria de **requisitos × interface** sobre o MVP. Cobre dois grupos:

1. **Segurança de acesso do portal** — novo requisito: todo paciente que se cadastra pela internet fica **pendente de aprovação** do terapeuta antes de acessar qualquer recurso.
2. **Fechamento de gaps de UI** — telas que existem no backend mas não têm interface usável, e melhorias de usabilidade para usuários leigos.

> Fonte de verdade durável: [`docs/requirements.md`](../../../docs/requirements.md). Esta spec **acrescenta** e **detalha** requisitos; onde houver conflito, o comportamento aqui descrito prevalece para o escopo `mvp-audit1`.

Convenções:
- Requisitos em EARS ("O sistema DEVE …", "QUANDO … o sistema DEVE …").
- Toda mensagem ao usuário em **PT-BR**, tom acolhedor, zero jargão (público leigo — `docs/requirements.md §4.5`).
- Valores monetários em centavos (BRL); datas no fuso `America/Sao_Paulo`.

---

## R1. Aprovação de paciente (gate de acesso do portal)

**Contexto:** o portal fica exposto na internet, atrás do Pangolin que **não** aplica controle de acesso (só TLS — `docs/requirements.md §2, §4.1`). Qualquer pessoa com uma conta Google pode se autenticar. Para mitigar abuso e acesso indevido, nenhum recurso do portal deve ser liberado antes do aceite explícito do terapeuta.

### R1.1 Estado de aprovação
- O sistema DEVE associar a cada paciente um estado de aprovação: `pendente`, `aprovado` ou `rejeitado`.
- QUANDO um paciente é cadastrado pelo próprio portal (auto-cadastro), o sistema DEVE criá-lo com estado `pendente`.
- QUANDO um paciente é cadastrado pelo terapeuta no admin, o sistema DEVE criá-lo já como `aprovado` (o terapeuta é a autoridade de aprovação).
- QUANDO um auto-cadastro do portal usa um email que já corresponde a um paciente `aprovado` (vínculo por email — `docs/requirements.md §3.1`), o sistema DEVE manter o estado `aprovado` (não rebaixar) e vincular ao registro existente.
- O estado de aprovação DEVE ser persistido e versionado por migration, com valor padrão `pendente` para novos registros do portal e `aprovado` para os registros pré-existentes na base (migração de dados idempotente).

### R1.2 Bloqueio de recursos enquanto pendente
- ENQUANTO um paciente estiver `pendente` ou `rejeitado`, o sistema DEVE negar o acesso às rotas de recurso do portal (agenda aberta, solicitar sessão, listar pedidos, sessões, débitos, atualização de perfil que não seja o cadastro inicial).
- O bloqueio DEVE ocorrer no servidor (middleware/serviço), não apenas na UI, retornando um erro coerente (HTTP 403) com mensagem PT-BR explicando que o cadastro está em análise.
- O paciente `pendente`/`rejeitado` DEVE continuar podendo: consultar o próprio estado de aprovação e sair (logout). Nenhuma outra ação.
- O sistema NÃO DEVE vazar dados clínicos, financeiros ou de agenda a paciente não aprovado.

### R1.3 Experiência do paciente pendente
- QUANDO um paciente `pendente` acessa o portal, a interface DEVE exibir uma página de "cadastro em análise" — acolhedora, sem jargão — informando que o terapeuta precisa aprovar o acesso e que ele será avisado.
- QUANDO um paciente `rejeitado` acessa o portal, a interface DEVE exibir uma mensagem clara de que o acesso não foi liberado, com orientação de contato (sem expor motivo técnico).
- A página de status DEVE oferecer um botão de "sair" e um de "atualizar" (reconsultar o estado).

### R1.4 Fila de aprovação no admin
- O admin DEVE ter uma tela/fila listando os pacientes `pendentes`, com nome, email, telefone e data de cadastro.
- O terapeuta DEVE poder **aprovar** ou **rejeitar** cada pendência, individualmente, com confirmação amigável.
- QUANDO o terapeuta aprova, o sistema DEVE mudar o estado para `aprovado` e liberar imediatamente o acesso do paciente.
- QUANDO o terapeuta rejeita, o sistema DEVE mudar o estado para `rejeitado`.
- Aprovar e rejeitar DEVEM registrar entrada no **audit log** (operação sensível — `docs/requirements.md §4.1`).
- A contagem de pendências DEVE ser visível de forma destacada no painel (para o terapeuta não perder pedidos).

### R1.5 Rate limiting (reforço)
- As rotas públicas do portal (cadastro/login) DEVEM manter o rate limiting por IP e por email já existente; o gate de aprovação é complementar, não substitui.

---

## R2. Locais de atendimento (UI)

Backend pronto (`/v1/admin/locations`, `/availability`); sem tela hoje. Sem locais, a agenda aberta do portal e o rateio de custo não funcionam (`docs/requirements.md §3.2`).

- O admin DEVE ter uma tela para **cadastrar, listar, editar e remover** locais (nome, endereço, modalidade `presencial`/`online`, custo em centavos, periodicidade `por_sessao`/`diario`/`mensal`/`anual`).
- Para cada local, o admin DEVE permitir **gerenciar a disponibilidade** (janelas por dia da semana: dia, horário início/fim, capacidade), com adicionar e remover.
- O seletor de dia da semana DEVE usar rótulos em PT-BR (Segunda…Sábado), respeitando a operação seg–sáb.
- Valores de custo DEVEM ser exibidos e informados em reais com máscara amigável (o sistema converte para centavos).

## R3. Origem do paciente (UI)

Backend pronto (`/v1/admin/origins`, campo `origin_id` no paciente); sem uso na UI. Base do ROI (`docs/requirements.md §3.1, §3.5`).

- O admin DEVE ter um cadastro simples de **origens** (canais de aquisição): criar, listar, renomear, remover.
- O formulário de cadastro e de edição de paciente DEVE incluir um seletor de **origem** (lista das origens cadastradas), opcional.
- QUANDO não houver origens cadastradas, a UI DEVE orientar o terapeuta a cadastrar ao menos uma (sem impedir o cadastro do paciente).

## R4. Planos por paciente (UI)

Backend pronto (`/v1/admin/plans`). Decide como o débito é gerado (`docs/requirements.md §3.4`).

- No detalhe do paciente, o admin DEVE ter uma aba/área de **planos**: listar, criar e remover planos.
- O formulário DEVE oferecer os tipos em rótulos claros PT-BR: pagamento por consulta, pagamento por mês, plano fechado mensal, plano fechado trimestral, atendimento social.
- Para tipos com valor fixo, o admin DEVE informar o **valor** (em reais, convertido para centavos) e a **vigência** (início e, opcional, fim).
- A UI DEVE deixar explícito, em linguagem simples, o efeito de cada tipo na cobrança (ex.: "valor fixo por mês, independente do número de sessões").

## R5. Agendar sessão direto pelo terapeuta (UI)

Backend pronto (`POST /v1/admin/sessions`, `/schedule`). Hoje a agenda só exibe sessões e o único caminho de criação é confirmar pedido do paciente (`docs/requirements.md §3.3`).

- A tela de agenda DEVE permitir **criar uma sessão** informando paciente, local, modalidade e horário início/fim.
- QUANDO o terapeuta cria/agenda uma sessão, o sistema DEVE respeitar a verificação de conflito de agenda já existente no backend e exibir a mensagem de erro PT-BR quando houver conflito.
- A criação DEVE estar acessível a partir do dia da agenda (ex.: botão "nova sessão" no dia), pré-preenchendo a data selecionada.

## R6. Custos e ROI (UI)

Backend pronto (`/v1/admin/costs/*`, `/reports/costs`, `/reports/roi`). Hoje só há "apurar custo por sessão" (`docs/requirements.md §3.5`).

- O admin DEVE ter uma área de **custos**: cadastrar categorias (local, CRP, infraestrutura, plataforma) e itens de custo (nome, valor, periodicidade, origem/local quando aplicável), listar e remover.
- O admin DEVE ter um **relatório de custos** por período (visão por tipo e por paciente).
- O admin DEVE ter um **relatório de ROI por canal**, cruzando receita por origem com custo da plataforma correspondente, apresentado de forma legível (não JSON cru).
- Os relatórios DEVEM permitir escolher o período (mês atual por padrão) e exibir os valores em reais.

## R7. Perfil do terapeuta (UI)

Backend pronto (`/v1/admin/profile`, `/profile/photo`, `/profile/links`). Sem tela (`docs/requirements.md §3.9`).

- O admin DEVE ter uma tela de **perfil do terapeuta**: nome, CRP, email, contatos, bio (Markdown), foto, locais associados.
- O admin DEVE permitir gerenciar **links de plataforma** (rótulo + URL, opcionalmente referenciando uma origem).
- A bio DEVE usar o editor Markdown com pré-visualização (consistente com o restante do sistema).

## R8. Backup e restore (UI)

Backend pronto (`/v1/admin/backup`, `/restore`). Operações sensíveis (`docs/requirements.md §3.8, §4.1`).

- O admin DEVE ter uma área de **backup**: botão para gerar backup sob demanda, exibindo resultado (arquivos enviados/pulados).
- O admin DEVE ter a opção de **restaurar**, com **dupla confirmação** explícita (ação destrutiva) e aviso claro de que a aplicação precisa ser reiniciada.
- Backup e restore DEVEM registrar audit log.

## R9. Registro de auditoria (UI)

Backend pronto (`/v1/admin/audit-log`).

- O admin DEVE ter uma tela que lista as entradas de auditoria recentes (ator, ação, entidade, data), legível e paginável/limitada, para transparência das operações sensíveis.

## R10. Usabilidade para leigos (transversal)

Foco explícito em usuários com pouco conhecimento técnico (`docs/requirements.md §4.5`).

### R10.1 Portal do paciente sem dados crus
- O portal NÃO DEVE exibir JSON cru em nenhuma tela. QUANDO listar sessões e débitos, o sistema DEVE apresentar listas legíveis (data/hora formatada, status em PT-BR, valores em reais).

### R10.2 Fim dos prompts nativos do navegador
- O sistema NÃO DEVE usar `window.prompt`/`window.confirm` para entrada de dados ou confirmação de ações. Em vez disso, DEVE usar formulários/diálogos inline consistentes com o design system.
- Especificamente: o registro de pagamento DEVE ter formulário inline (valor e forma de pagamento); exclusões e ações destrutivas DEVEM ter confirmação inline.

### R10.3 Login do portal coerente
- O texto e o fluxo do login do portal DEVEM ser coerentes com o mecanismo real (login social Google). A UI NÃO DEVE prometer um fluxo que não executa.

### R10.4 Navegação clara
- A navegação principal do admin DEVE ser fácil de encontrar e refletir todas as áreas disponíveis (incluindo as novas telas desta spec).
- Datas e horas na agenda e listas DEVEM usar formato curto e legível em PT-BR (ex.: "Seg, 08/09 · 14:00").

### R10.5 Acessibilidade
- Todos os novos controles interativos DEVEM manter acessibilidade WCAG AA: navegação por teclado, foco visível, labels associados, contraste adequado (`docs/requirements.md §4.5`).

---

## Prioridade sugerida

1. **R1** (segurança de acesso — bloqueia risco em produção).
2. **R2, R3, R4, R5** (destravam o núcleo: locais, origem, planos, agendar).
3. **R10** (usabilidade do portal e fim dos prompts — maior impacto para leigos).
4. **R6, R7, R8, R9** (gestão e operação).

## Processo desta onda (obrigatório)

Aplica-se a política de entrega do projeto ([`.kiro/steering/psicoman-processo-entrega.md`](../../steering/psicoman-processo-entrega.md)):
- **Documentação junto do código**: cada requisito implementado atualiza, no mesmo ciclo, `docs/requirements.md`, `docs/architecture.md`, `README.md` (quando muda uso) e o Swagger (quando muda contrato). Nada de "documentar depois".
- **Testes cobrem o que foi feito**: regra de negócio → unit; fluxo/contrato → E2E; segurança/acesso (R1) → E2E do caminho permitido e do negado.
- **Registro de decisões**: as decisões desta onda ficam em [`docs/decisions.md`](../../../docs/decisions.md) para lastro futuro.
- **Git**: commit ao fim de cada fase; push único ao final da onda.

## Não-objetivos

- Não altera o mecanismo de autenticação (Pangolin no admin; login social no portal).
- Não introduz canal de notificação próprio (segue via Google Calendar — `docs/requirements.md §5`).
- Não implementa fluxos fora do escopo do MVP listados em `docs/requirements.md §5`.
