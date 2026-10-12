# Psicoman — Notas de versão

O que muda a cada onda de trabalho, em linguagem para quem usa o sistema. Detalhe técnico em [`decisions.md`](./decisions.md). Mais recente no topo.

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
