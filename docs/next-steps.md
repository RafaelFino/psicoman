# Next Steps — Psicoman

Funcionalidades e melhorias planejadas para as próximas versões.

## Prioridade Alta — Segurança

| Item | Descrição | Esforço |
|------|-----------|---------|
| StaffAuth em produção | Rejeitar requests sem X-User-Email quando DEV_MODE=false | 1h |
| Race condition DBCalendar | Passar DB como parâmetro ao invés de mutar campo compartilhado | 2h |
| Rate limiting | Token bucket em endpoints públicos (10 req/min /auth/*, 60/min /patient/*) | 4h |
| Validação de uploads | Verificar MIME real via http.DetectContentType + limitar tamanho (10MB) | 2h |

## Prioridade Média — Funcionalidades

| Item | Descrição | Esforço |
|------|-----------|---------|
| Relatórios PDF | Exportar relatório mensal consolidado como PDF (usando wkhtmltopdf ou similar) | 8h |
| Google Calendar — teste em produção | Configurar credenciais reais, testar fluxo OAuth, tratar refresh tokens | 4h |
| Vinculação espaço → consulta | Ao criar consulta presencial, selecionar espaço disponível | 4h |
| Notificações por email | Lembrete de consulta, contrato pendente, anamnese não preenchida | 8h |
| Backup automatizado | Script de rsync/cron para data/ | 2h |
| Google Calendar no celular | Configurar push notifications para que o terapeuta receba alertas de consultas no Android/iOS via Google Calendar | 4h |
| Backup SQLite no Google Drive | Upload automático do arquivo .sqlite para Google Drive do terapeuta (usando mesma autenticação OAuth já existente) | 6h |

## Prioridade Baixa — Polish

| Item | Descrição | Esforço |
|------|-----------|---------|
| Drag-and-drop na agenda | Reagendar consulta arrastando na visão semanal | 8h |
| Rich text editor | Evolução de sessão com formatação (negrito, listas, etc.) | 6h |
| CSP header | Content-Security-Policy restritivo | 2h |
| Busca global | Buscar pacientes/consultas por texto livre | 4h |

## Bugs conhecidos (severidade baixa)

| # | Local | Descrição |
|---|-------|-----------|
| 1 | storage/document.go | Dead code no branch sem patientID (variável não usada) |
| 2 | web/handlers_psych.go | downloadDocument abre arquivo duas vezes (redundante) |
| 3 | service/google.go | fallback() gera URL fake de Meet que não funciona |
| 4 | domain/scheduling.go | Cálculo de horas com valor negativo (funciona mas confuso) |
| 5 | service/patient.go | FullReport usa janela de 10 anos desnecessariamente |
