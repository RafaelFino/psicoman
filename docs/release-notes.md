# Psicoman — Notas de versão

O que muda a cada onda de trabalho, em linguagem para quem usa o sistema. Detalhe técnico em [`decisions.md`](./decisions.md). Mais recente no topo.

---

## Em andamento — Onda `mvp-audit1`

Planejada (ver spec em `.kiro/specs/mvp-audit1/`). Ainda não liberada.

**Novidades previstas**
- Aprovação de pacientes: quem se cadastra pelo site fica "em análise" até você aprovar, protegendo seu consultório de acessos indevidos.
- Telas que faltavam: locais de atendimento e horários, origens dos pacientes, planos, agendamento direto pela agenda, custos e retorno por canal (ROI), seu perfil profissional, backup e histórico de auditoria.

**Melhorias previstas**
- Portal do paciente mais claro (sem telas técnicas), sem caixas de diálogo do navegador, com pagamentos e confirmações dentro da própria tela.

---

## 2026-09-06 — Agenda, ficha do paciente e textos formatados

**Novidades**
- **Agenda da semana** no painel: veja de segunda a sábado o que está marcado, navegue entre semanas e clique numa sessão para abrir o paciente.
- **Ficha completa do paciente**: ao clicar num paciente, você vê tudo dele em abas — dados, anamnese, sessões, anotações, débitos/pagamentos/recibos, custos e arquivos.
- **Modelos (templates)**: crie modelos de texto (como a anamnese) para enviar aos pacientes, com pré-visualização de como ficará.

**Melhorias**
- **Escrita formatada (Markdown)** nos campos de texto (anamnese, anotações, modelos): escreva com títulos, listas, negrito, itálico, links e trechos de código, e veja o resultado formatado. O que você pré-visualiza é igual ao que o paciente recebe.
- Edição dos dados do paciente direto na ficha.
