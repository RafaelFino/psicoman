# Psicoman — Notas de versão

O que muda a cada onda de trabalho, em linguagem para quem usa o sistema. Detalhe técnico em [`decisions.md`](./decisions.md). Mais recente no topo.

---

## 2026-10-24 — Base técnica nova: um só aplicativo, prontuário cifrado e assistência por IA (onda `pocketbase-core`)

Reconstrói a fundação do Psicoman sobre uma base mais sólida e segura, sem mudar o jeito de trabalhar no dia a dia: agora é **um único aplicativo**, o prontuário fica **cifrado**, a agenda checa conflito na hora, a cobrança vira PDF pronto para enviar, o backup ganha uma cópia de segurança na nuvem e surge uma **ajuda opcional de IA** para resumir sessões. Detalhe técnico em [`decisions.md`](./decisions.md) e [`architecture.md`](./architecture.md).

**Novidades**
- **Prontuário protegido por criptografia.** As anotações clínicas, a anamnese e os resumos passam a ser **guardados cifrados**. Só você, no seu acesso de terapeuta, enxerga o conteúdo em texto; o portal do paciente e as ferramentas técnicas jamais mostram o conteúdo clínico.
- **Agenda avisa conflito na hora de marcar.** Antes de confirmar uma sessão, o sistema consulta a sua agenda do Google e, se houver choque de horário com qualquer compromisso, **bloqueia e avisa** — nada é sobrescrito. Sessões online ganham o link do Meet automaticamente.
- **Cobrança em PDF pronta para compartilhar.** Gere o **PDF da cobrança** de um débito com um clique; ele fica guardado como comprovante e você compartilha com o paciente por onde preferir (ex.: WhatsApp). O sistema não envia email — o canal continua sendo você.
- **Cópia de segurança na nuvem (Google Drive).** Além do backup local, o sistema envia uma **cópia cifrada** para uma pasta reservada no seu Google Drive, como plano de recuperação de desastre. Se o Google não estiver configurado, o backup local continua funcionando normalmente.
- **Assistência por IA para resumir sessões (opcional).** Se você ativar a IA (fornecendo sua chave do Gemini), pode pedir um **resumo** de uma sessão com **tags temáticas** — sempre a seu pedido e **para você revisar** antes de salvar. As tags ajudam a **buscar** sessões de um paciente por tema. Sem a chave, a IA fica desligada e nada é enviado para fora.
- **Portal do paciente com login próprio.** O paciente se cadastra com email e senha e entra no portal; continua valendo a sua **aprovação** antes de ele acessar qualquer coisa.

**Melhorias**
- **Um aplicativo só.** Onde antes havia dois programas (admin e portal), agora há **um único aplicativo** que atende os dois, mais simples de operar e atualizar. A tela técnica de administração continua disponível em `/_/`.
- **Configuração em um arquivo.** Tudo (chaves, Google, IA) fica num `config.yaml`, e dá para sobrescrever por variáveis de ambiente — o sistema registra de onde veio cada ajuste, sem nunca expor senhas ou chaves.
- **Busca clínica por tags.** As tags geradas pela IA ficam pesquisáveis por paciente e por ano, mantendo o conteúdo das anotações cifrado.

**Notas de migração/operação**
- O sistema agora é **um único binário** que roda **atrás do Pangolin** (que cuida do acesso e do HTTPS). Configure a chave de criptografia no `config.yaml` **antes** de subir — sem ela, o sistema não inicia (proteção proposital).
- **Envio de cobrança por email saiu** desta base: a cobrança é o PDF guardado no sistema, compartilhado por você. 
- A **cópia no Google Drive** e a **IA** são opcionais: funcionam só quando você fornece as credenciais correspondentes; na ausência delas, o resto do sistema segue normal.

---

## 2026-10-12 — A história do paciente numa linha do tempo e o seu dia num feed (onda `patient-timeline`)

Transforma a tela do paciente na história completa dele e dá ao seu dia um resumo didático; o faturamento por sessão passa a ser automático e os planos fechados cobram no dia certo. Detalhe técnico em [`decisions.md`](./decisions.md).

**Novidades**
- **Timeline do paciente.** Ao abrir um paciente, a primeira coisa que você vê é a **história completa dele** num feed do mais recente ao mais antigo: cadastro, sessões, remarcações, relatórios clínicos, anamnese, débitos, pagamentos e arquivos. Dá para **filtrar** por tema (Clínico, Financeiro, Agenda, Arquivos) e clicar em cada evento para abrir a sessão, o débito ou o arquivo.
- **Calendário mensal do paciente.** Uma visão de calendário com todas as sessões daquele paciente — passadas e futuras — coloridas por status, com navegação por mês. Clique num dia para abrir o registro da sessão.
- **Métricas de sessão no paciente.** No topo da timeline, um resumo com realizadas, faltas, canceladas, **taxa de comparecimento**, **taxa de cancelamento** e o valor faturado das sessões realizadas.
- **Débito automático a cada sessão realizada.** Marcar a sessão como realizada já gera a cobrança, sem depender de lembrar de marcar um campo. Se quiser que uma sessão específica **não** gere cobrança, basta desmarcar na finalização.
- **Planos fechados cobram no dia de vencimento.** Cada plano fechado tem um **dia de vencimento** (1 a 28); a cobrança do período é gerada a partir dessa data, com o vencimento certo — não mais "sete dias depois".
- **Controle completo do status da sessão.** Agora há um passo opcional de **confirmação** e, ao cancelar, você registra **quem cancelou** (paciente ou terapeuta) e o **motivo**. Tudo isso aparece na história do paciente, inclusive as **remarcações** de horário.
- **Relatório de sessões por local.** Para repassar a clínicas parceiras: por local e por mês, quantos atendimentos houve por categoria e o faturamento das sessões realizadas.
- **Feed do dia do terapeuta.** O painel passa a abrir num **feed do seu dia**, transversal a todos os pacientes: o que aconteceu (sessões, pagamentos, arquivos) e o que vem (próximas sessões, vencimentos, pedidos pendentes), agrupado por "agora", "mais tarde", "amanhã". Você escolhe a janela (hoje, próximos 7 dias, recentes). A agenda em grade continua a um clique no menu.
- **Prontuário como feed clínico.** O prontuário mostra os registros do mais recente ao mais antigo, com a **anamnese em destaque** como ficha inicial e um atalho para registrar rapidamente o que aconteceu.

**Melhorias**
- **Resumo do dia enxuto.** No feed do dia, o bloco de números (resumo agregado) começa recolhido para não poluir — você expande quando quiser; os eventos do dia ficam sempre à vista.
- **Marca no título da aba.** O título da aba do navegador agora traz sempre "Psicoman · …", no admin e no portal.

**Notas de migração/operação**
- Esta versão aplica automaticamente duas atualizações no banco no primeiro boot (aditivas e idempotentes): o **dia de vencimento** nos planos (planos fechados pré-existentes recebem o dia 5 como padrão) e os **novos campos de status da sessão** (confirmação, autor/motivo/data de cancelamento). Nenhum dado é perdido.
- O **job diário** que fecha os ciclos dos planos fechados passa a respeitar o dia de vencimento: ele pode rodar todo dia sem gerar cobrança duplicada.
- A **exportação do relatório por local** (CSV/impressão) ainda não entra nesta versão; o relatório é legível na tela para repasse.

---

## 2026-10-12 — Prontuário num lugar só, WhatsApp e visual novo (onda `prontuario-ux`)

Deixa o acompanhamento clínico mais fácil de ler e registrar, aproxima o contato com o paciente e renova a cara do sistema. Detalhe técnico em [`decisions.md`](./decisions.md).

**Novidades**
- **Prontuário unificado.** No paciente, uma aba única **Prontuário** reúne, numa linha do tempo por data (do mais recente para o mais antigo), as sessões e tudo o que você anotou — relatórios de cada sessão e anotações livres, lado a lado. Substitui as antigas abas "Sessões" e "Notas". A Anamnese continua separada, como um documento à parte.
- **Abrir e editar uma sessão.** Clique numa sessão (na Agenda ou no Prontuário) para abrir um painel: ajuste data/hora, local e modalidade, finalize/cancele/marque falta, e escreva os relatórios daquela sessão. Os relatórios aparecem agrupados sob a sessão certa no Prontuário.
- **Telefone com WhatsApp.** O telefone do paciente vira um link que abre a conversa no WhatsApp, direto do painel. Os números passam a ser guardados num formato padronizado (com DDD e país), para o link funcionar sempre.
- **Resumo rápido do acompanhamento.** No topo do Prontuário: total de sessões, realizadas, faltas e a data do último registro.

**Melhorias**
- **Agenda da semana que respira.** Um dia cheio (vários atendimentos) agora cresce para baixo sem espremer os outros dias, como num calendário de verdade. Continua de segunda a sábado, e cada cartão abre a sessão com um clique.
- **Visual novo.** Nova identidade em **azul royal** e **lilás** sobre fundos claros, mais sóbria e acolhedora, mantendo as cores de aviso (verde/âmbar/vermelho) onde elas ajudam a entender rápido.

**Notas de migração/operação**
- A padronização do telefone vale para novos cadastros e para quando você editar um paciente. Cadastros antigos continuam aparecendo normalmente; o número é ajustado automaticamente na próxima vez que você salvar aquele paciente.
- Ao mudar o horário de uma sessão que tem convite no Google, ajuste o convite no Google manualmente — o sistema avisa isso na tela.

---

## 2026-10-12 — Controle de acesso do portal e telas que faltavam (onda `mvp-audit1`)

Fecha a lacuna entre o que o sistema já fazia por dentro e o que você consegue fazer pela tela, e protege o portal com um aceite explícito do terapeuta. Detalhe técnico em [`decisions.md`](./decisions.md).

**Novidades**
- **Aprovação de pacientes.** Quem se cadastra pelo site fica "em análise" e não vê nenhuma informação até você aprovar. Você tem uma fila de aprovação no painel, com contagem em destaque, e aprova ou rejeita cada cadastro ali mesmo. Pacientes que você mesmo cadastra já entram liberados.
- **Locais de atendimento e horários.** Cadastre seus consultórios (presencial/online), o custo de cada um e as janelas de disponibilidade por dia da semana. Esses horários passam a aparecer na agenda aberta do portal.
- **Origens dos pacientes.** Registre por onde cada paciente chegou (indicação, Doctoralia, etc.) — é a base do relatório de retorno por canal.
- **Planos por paciente.** Defina como a cobrança é gerada: por consulta, por mês, plano fechado mensal/trimestral ou atendimento social, com valor e vigência quando faz sentido.
- **Agendar sessão direto pela agenda.** Crie uma sessão informando paciente, local, modalidade e horário, com aviso claro quando houver conflito de horário.
- **Custos e ROI.** Cadastre categorias e itens de custo e veja relatórios de custo e de retorno por canal (ROI) por período, em valores legíveis.
- **Seu perfil profissional.** Nome, CRP, contatos, bio em texto formatado, foto e links de plataforma.
- **Backup e restauração.** Gere backup sob demanda e veja o resultado; a restauração pede dupla confirmação por ser uma ação destrutiva.
- **Histórico de auditoria.** Tela que lista as operações sensíveis (quem fez, o quê e quando).

**Melhorias**
- **Portal do paciente mais claro.** Sessões e débitos aparecem em listas legíveis (data por extenso curta, status em português, valores em reais) — sem telas técnicas.
- **Fim das caixas de diálogo do navegador.** Pagamentos e confirmações de exclusão acontecem dentro da própria tela, com formulários e confirmações inline.
- **Login do portal honesto.** Os textos descrevem o login social como ele realmente funciona.
- **Navegação do painel reorganizada** em grupos (Atendimento, Gestão, Financeiro), com todas as áreas acessíveis pelo menu, datas curtas nas listas e navegação por teclado.

**Notas de migração/operação**
- Esta versão adiciona o estado de aprovação aos pacientes automaticamente no primeiro boot (migração aditiva e idempotente). **Todos os pacientes já existentes entram como aprovados** — ninguém perde acesso. Apenas novos cadastros feitos pelo site passam a exigir sua aprovação.

---

## 2026-09-06 — Agenda, ficha do paciente e textos formatados

**Novidades**
- **Agenda da semana** no painel: veja de segunda a sábado o que está marcado, navegue entre semanas e clique numa sessão para abrir o paciente.
- **Ficha completa do paciente**: ao clicar num paciente, você vê tudo dele em abas — dados, anamnese, sessões, anotações, débitos/pagamentos/recibos, custos e arquivos.
- **Modelos (templates)**: crie modelos de texto (como a anamnese) para enviar aos pacientes, com pré-visualização de como ficará.

**Melhorias**
- **Escrita formatada (Markdown)** nos campos de texto (anamnese, anotações, modelos): escreva com títulos, listas, negrito, itálico, links e trechos de código, e veja o resultado formatado. O que você pré-visualiza é igual ao que o paciente recebe.
- Edição dos dados do paciente direto na ficha.
