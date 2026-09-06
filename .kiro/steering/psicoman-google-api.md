---
inclusion: fileMatch
fileMatchPattern: "**/integration/google/**"
---

# Psicoman — Integração Google (Calendar, Meet, Gmail, Drive)

Decisões de integração em `docs/architecture.md §4.3` e `docs/requirements.md §3.7`.

## OAuth

- Conta pessoal do terapeuta (Google One, sem Workspace) → **sem domain-wide delegation**. Fluxo é **OAuth 3-legged**: autorização única do terapeuta, refresh token persistido **cifrado**, renovação automática do access token em runtime.
- Escopos mínimos necessários, nunca mais amplos que o preciso:
  - `https://www.googleapis.com/auth/calendar` (Calendar/Meet)
  - `https://www.googleapis.com/auth/gmail.send` (envio, não leitura de inbox)
  - `https://www.googleapis.com/auth/drive.file` (só arquivos criados pelo app — nunca `drive` ou `drive.readonly` full)
- Falha ao renovar o refresh token não pode derrubar o processo: sinalizar estado "reautorização necessária" (expor em `/readyz` como degraded e na UI do admin), sem panic.

## Calendar

- Antes de confirmar qualquer sessão (agendamento direto ou confirmação de pedido do paciente), consultar **freebusy** do terapeuta para o horário pretendido — cobre também eventos não criados pelo Psicoman. Conflito → erro 409, nunca sobrescrever silenciosamente.
- Evento criado sempre inclui: paciente como convidado (pelo email cadastrado) e link de Google Meet.
- Reminders do evento (não notificação própria do sistema) carregam os intervalos configurados (default 1 dia + 30 min, acumulativos).

## Gmail

- Usado só para envio (cobranças em PDF, templates renderizados). Nunca implementar leitura/parsing de inbox — fora de escopo.
- Envio de email é **best-effort**: falha ao enviar não pode bloquear a operação de negócio que o originou (ex: gerar o PDF de cobrança tem que funcionar mesmo se o envio por email falhar). Logar e permitir reenvio manual.

## Drive

- Escopo `drive.file` restringe a arquivos criados pela própria aplicação — não há acesso ao Drive pessoal do terapeuta além disso.
- Backup do SQLite: sempre cifrado (AES-GCM) e compactado antes do upload — nunca subir o `.db` em texto claro.
- Backup do GED: incremental por hash (manifesto no Drive), nunca reenviar o acervo inteiro a cada execução.

## Testes

- Toda integração com Google fica atrás de uma interface (`CalendarClient`, `GmailClient`, `DriveClient`) definida em `internal/service` ou `internal/domain`, nunca chamada direto da lib do Google em `service`.
- Testes E2E usam fakes/mocks dessas interfaces — nenhum teste automatizado chama a API real do Google.
