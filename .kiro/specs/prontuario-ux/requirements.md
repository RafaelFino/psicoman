# prontuario-ux — Requisitos

Terceira onda de implementação do Psicoman, derivada de um pedido de produto do terapeuta (Fino) focado em **usabilidade do acompanhamento clínico** e **identidade visual**. Cobre cinco grupos:

1. **Prontuário unificado** — reunir notas, relatórios e registros de sessão num único diário por data, com boa leitura e organização cronológica.
2. **Edição e enriquecimento da sessão** — abrir uma sessão, editar seus dados e registrar notas/relatórios vinculados a ela, com visão correlacionada entre sessão e registros.
3. **Contato por WhatsApp** — telefone do paciente vira link para o WhatsApp Web, com o número normalizado já na entrada do dado.
4. **Nova paleta visual** — trocar o tema branco+verde/teal por azul royal + lilás sobre fundos claros derivados e branco.
5. **Calendário fluido** — a visão semanal cresce conforme a lotação do dia (estilo Google Calendar), fluida para ~10 pacientes/dia, mantendo seg–sáb (sem domingo).

> Fonte de verdade durável: [`docs/requirements.md`](../../../docs/requirements.md) e [`docs/architecture.md`](../../../docs/architecture.md). Esta spec **acrescenta** e **detalha** requisitos; onde houver conflito, o comportamento aqui descrito prevalece para o escopo `prontuario-ux`. Decisão de origem registrada em [`docs/decisions.md`](../../../docs/decisions.md).

Convenções:
- Requisitos em EARS ("O sistema DEVE …", "QUANDO … o sistema DEVE …").
- Toda mensagem ao usuário em **PT-BR**, tom acolhedor, zero jargão (público leigo — `docs/requirements.md §4.5`).
- Datas no fuso `America/Sao_Paulo`; conteúdo clínico em Markdown.
- Terapeutas são o público do admin e **não** são fluentes em tecnologia: cada tela precisa ser óbvia, com alvos de toque ≥ 44px e foco visível (WCAG AA).

---

## R1. Prontuário unificado (diário de acompanhamento)

**Contexto:** hoje o detalhe do paciente tem três abas tocando conteúdo clínico de forma fragmentada — **Anamnese** (documento único), **Sessões** (lista de agendamentos) e **Notas** (notas livres em Markdown). O terapeuta quer **um lugar só**, o "Prontuário", que funcione como um diário cronológico do acompanhamento, misturando o que aconteceu em cada sessão com anotações livres, com leitura e visualização boas. O backend já modela isso: `domain.Note` tem `SessionID` (vínculo com a sessão) ou vazio (nota livre), sempre ordenada por `created_at` (`internal/domain/prontuario.go`).

### R1.1 Aba Prontuário
- O detalhe do paciente DEVE ter uma aba única **"Prontuário"** que substitui as abas atuais **"Sessões"** e **"Notas"**.
- A aba **"Anamnese"** DEVE permanecer separada (é um documento único do paciente, não uma entrada de diário).
- O agendamento de sessões (criar/alterar horário) permanece na **Agenda**; o Prontuário trata do **registro clínico**, não do ato de agendar. (Ver R2 para a edição de uma sessão a partir do Prontuário.)

### R1.2 Timeline cronológica
- O Prontuário DEVE apresentar uma linha do tempo **agrupada por data**, do mais recente para o mais antigo.
- Cada **sessão** do paciente DEVE aparecer como um marco na timeline, no dia/horário em que ocorre, exibindo: horário, modalidade (presencial/online), status (solicitada/agendada/realizada/falta/cancelada) e os registros clínicos vinculados a ela.
- Cada **nota livre** (sem sessão) DEVE aparecer na mesma linha do tempo, posicionada pela sua data de criação.
- QUANDO não houver nenhum registro nem sessão, o sistema DEVE exibir um estado vazio acolhedor, convidando a registrar a primeira anotação.
- A timeline DEVE renderizar o conteúdo Markdown dos registros já formatado (reusando o renderizador cliente existente, `Md.toHtml`).

### R1.3 Resumo e usabilidade
- O topo do Prontuário DEVE trazer um resumo rápido do acompanhamento: total de sessões, realizadas, faltas, e data da última sessão/registro.
- A visualização DEVE priorizar a leitura: datas destacadas, separação visual clara entre dias, e os registros de cada sessão agrupados sob o marco da sessão.
- A navegação por datas DEVE ser fluida mesmo com histórico longo (dezenas de sessões): sem travar a página e sem exigir conhecimento técnico.

### R1.4 Compatibilidade de dados
- O Prontuário DEVE consumir os dados já existentes (anamnese à parte, notas de `GET /patients/{id}/notes`, sessões de `GET /sessions?patient_id=`) sem exigir migração destrutiva.
- Notas livres pré-existentes (criadas antes desta onda, sem `session_id`) DEVEM continuar aparecendo corretamente na timeline.

---

## R2. Edição da sessão e registros vinculados

**Contexto:** o terapeuta quer clicar numa sessão (na Agenda ou no Prontuário) e poder **editar todos os dados dela** e, principalmente, **registrar notas/relatórios daquela sessão**, enriquecendo o prontuário. Hoje não existe edição de dados da sessão — só criação e transições de estado (`finish`/`cancel`/`no-show`). As notas por sessão já são suportadas no backend (`POST /patients/{id}/notes` aceita `session_id`), mas a UI nunca usou esse vínculo.

### R2.1 Abrir uma sessão
- QUANDO o terapeuta clica numa sessão (slot da Agenda ou marco do Prontuário), o sistema DEVE abrir uma visão/painel daquela sessão com: dados da sessão, suas transições de estado disponíveis e seus registros clínicos vinculados.

### R2.2 Editar dados da sessão
- O sistema DEVE permitir editar os dados de uma sessão: **data/hora de início e fim, local e modalidade**.
- O backend DEVE expor uma operação de edição de sessão (`PUT /v1/admin/sessions/{id}`) que valida as invariantes de domínio (fim após início, modalidade válida, paciente existente) e persiste a alteração.
- A edição NÃO DEVE permitir mudar o paciente da sessão (uma sessão pertence a um paciente; trocar paciente é criar outra sessão).
- QUANDO a sessão está num estado terminal (realizada/cancelada/falta), a edição de horário/local DEVE ser **bloqueada ou restrita** com mensagem clara (não faz sentido reagendar algo já realizado); o registro clínico (R2.3) continua permitido.
- A edição de sessão DEVE registrar entrada no **audit log** (operação sobre dado sensível).
- QUANDO a sessão possui evento no Google Calendar, a edição de horário DEVE ao menos não corromper o vínculo; a reconciliação completa com o Calendar pode ser tratada como melhoria futura e, se não implementada agora, DEVE ser explicitada ao terapeuta (ex.: "ajuste o convite no Google se necessário"). Esta é uma decisão a consolidar no design.

### R2.3 Registrar notas/relatórios da sessão
- O sistema DEVE permitir adicionar um ou mais registros em Markdown **vinculados a uma sessão** (via `session_id`), representando notas, comentários e relatórios daquela sessão.
- QUANDO o terapeuta adiciona um registro a partir da visão de uma sessão, o sistema DEVE enviar o `session_id` correspondente, de modo que o registro apareça correlacionado àquela sessão no Prontuário.
- O sistema DEVE permitir **remover** um registro, com confirmação amigável (comportamento já existente para notas).
- Cada registro DEVE exibir sua data/hora de criação e ser renderizado em Markdown.

### R2.4 Correlação sessão ↔ registros
- O Prontuário DEVE deixar visualmente claro **quais registros pertencem a qual sessão** e **quais são notas livres**.
- QUANDO uma sessão tem registros vinculados, a visão da sessão DEVE listá-los; QUANDO não tem, DEVE convidar a criar o primeiro relatório da sessão.

---

## R3. Contato por WhatsApp

**Contexto:** o admin roda no navegador. Ao ver o telefone do paciente, o terapeuta quer **clicar e abrir a conversa no WhatsApp Web**. Hoje o telefone é texto puro em vários pontos (cabeçalho e aba Dados do detalhe, lista de pacientes). Além disso, os números são gravados sem padronização (ex.: `11999990001`, sem DDI), o que impede montar o link de forma confiável.

### R3.1 Normalização do telefone (na entrada)
- QUANDO um telefone é informado (cadastro de paciente no admin, cadastro pelo portal, edição de dados), o sistema DEVE **normalizar e persistir** o número num formato canônico: somente dígitos, com DDI do Brasil (`55`) quando o número vier sem DDI.
- A normalização DEVE preservar números que já venham com DDI (ex.: começando com `55` e comprimento compatível) sem duplicar o prefixo.
- A normalização DEVE descartar máscara/pontuação (parênteses, traços, espaços, `+`).
- QUANDO o telefone informado não tiver dígitos suficientes para ser um número brasileiro plausível (fixo/móvel com DDD), o sistema DEVE rejeitar com mensagem PT-BR clara, mantendo o telefone como campo obrigatório (`docs/requirements.md §3.1`).
- A normalização é regra de **domínio**, aplicada no serviço de paciente, valendo para admin e portal (não só para a massa de testes).

### R3.2 Link para o WhatsApp Web
- Onde o telefone do paciente é exibido no admin (ao menos: cabeçalho do detalhe, aba Dados, lista de pacientes), o número DEVE ser um **hyperlink** que abre a conversa do WhatsApp.
- O link DEVE usar o formato `https://wa.me/<numero>` (o `wa.me` encaminha para o WhatsApp Web no desktop), abrindo em nova aba (`target="_blank"`, `rel="noopener"`).
- O número no link DEVE ser o canônico (só dígitos com DDI), coerente com R3.1.
- QUANDO o paciente não tiver telefone válido, o sistema NÃO DEVE renderizar um link quebrado; DEVE exibir o estado "—" ou texto simples.

### R3.3 Massa de testes
- A massa de testes (`scripts/seed.sh`) DEVE cadastrar pacientes com telefones que, após a normalização, resultem em números válidos de WhatsApp (com DDI), para exercitar o link de ponta a ponta.

---

## R4. Nova paleta visual

**Contexto:** o tema atual é branco com acento verde/teal (`--primary: teal`, `--accent: indigo`), que o terapeuta considera genérico. Quer tons de **lilás** e **azul royal** sobre fundos claros derivados e branco. As cores são centralizadas em tokens CSS (`:root` em `internal/web/static/app.css`), sem Tailwind nem build — trocar os tokens propaga para toda a UI.

### R4.1 Tokens de cor
- O sistema DEVE adotar **azul royal** como cor primária (`--primary` e variações strong/soft) e **lilás** como cor de acento (`--accent` e soft).
- Os fundos DEVEM ser claros e derivados da paleta (tons muito suaves de lilás/azul), com as superfícies principais em **branco**.
- As cores de estado (ok/atenção/perigo) DEVEM ser mantidas ou ajustadas apenas o necessário para harmonizar com a nova base, preservando semântica (verde=ok, âmbar=atenção, vermelho=perigo).

### R4.2 Acessibilidade do contraste
- Texto sobre superfície DEVE manter contraste ≥ 4.5:1; texto grande e elementos de interface ≥ 3:1 (WCAG AA), conforme já praticado no CSS atual.
- Branco sobre a cor primária (ex.: botões primários) DEVE manter contraste ≥ 4.5:1.
- O anel de foco (`--ring`) DEVE permanecer visível e perceptível sobre a nova paleta.

### R4.3 Consistência
- A troca DEVE ser feita nos tokens (`:root`) e em quaisquer gradientes/cores fixas que referenciem a paleta antiga (ex.: logo, hero), sem deixar resquícios de verde/teal fora da semântica de estado.

---

## R5. Calendário semanal fluido

**Contexto:** a visão semanal (seg–sáb) hoje usa células de altura fixa (`.week-day`, `min-height: 120px`) que apenas empilham os slots. Com ~10 pacientes num dia, a usabilidade sofre. O terapeuta quer a experiência do Google Calendar, onde o dia **cresce conforme a lotação**, de forma fluida, mantendo seg–sáb e sem domingo.

### R5.1 Dia que cresce com a lotação
- A visão semanal DEVE acomodar muitos atendimentos no mesmo dia (referência: ~10/dia) de forma legível, com o espaço do dia **crescendo conforme o número de sessões**, sem sobreposição ilegível nem corte de conteúdo.
- Os cartões de sessão DEVEM permanecer legíveis (horário, nome do paciente, modalidade, status) e clicáveis (abrindo a sessão — R2.1).
- A rolagem/consumo da semana cheia DEVE ser fluida; a experiência NÃO DEVE exigir conhecimento técnico.

### R5.2 Dias exibidos
- A visão semanal DEVE exibir **segunda a sábado** (6 dias) e **omitir o domingo**, conforme a operação do consultório.
- A visão mensal (planejamento) permanece como está (grade dom–sáb), fora do escopo de redesenho desta onda, salvo ajuste de cores (R4).

### R5.3 Clareza por status
- Cada cartão de sessão DEVE indicar o status por cor/afetação visual coerente com a nova paleta (R4), preservando a leitura rápida (ex.: agendada, realizada, falta, cancelada, solicitada).
- O dia atual DEVE continuar destacado.

### R5.4 Responsividade
- Em telas estreitas (celular/tablet), a semana DEVE permanecer utilizável (ex.: coluna única empilhada), sem perder a legibilidade dos cartões nem a capacidade de abrir a sessão.

---

## Fora de escopo (desta onda)

- Grade de calendário por eixo de horas (blocos posicionados pelo horário real, estilo Google "dia/semana" com linha do tempo vertical). A abordagem desta onda é o dia que cresce com a lotação (R5.1), decidida com o terapeuta.
- Reconciliação completa de edição de sessão com o Google Calendar (ver R2.2 — a consolidar no design).
- Redesenho da visão mensal além da paleta.
- Envio de mensagem automática por WhatsApp (apenas o link de abertura de conversa; nada de API de mensageria).
