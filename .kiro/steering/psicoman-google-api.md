---
inclusion: fileMatch
fileMatchPattern: "**/integration/google/**"
---

# Psicoman — Integração Google (Calendar, Meet, Gmail, Drive)

Decisões de integração em `docs/architecture.md` e `docs/requirements.md`. As integrações vivem em `internal/integration/google` e são consumidas pelo **binário único** `cmd/psicoman` (que roda atrás do `Pangolin`, com segregação admin/portal apenas lógica — não há mais dois binários/processos para admin e portal).

## OAuth

- Conta pessoal do terapeuta (Google One, sem Workspace) → **sem domain-wide delegation**. Fluxo é **OAuth 3-legged**: autorização única do terapeuta, refresh token persistido **cifrado**, renovação automática do access token em runtime.
- Escopos mínimos necessários, nunca mais amplos que o preciso:
  - `https://www.googleapis.com/auth/calendar` (Calendar/Meet)
  - `https://www.googleapis.com/auth/gmail.send` (envio, não leitura de inbox)
  - `https://www.googleapis.com/auth/drive.file` (só arquivos criados pelo app — nunca `drive` ou `drive.readonly` full)
- Falha ao renovar o refresh token não pode derrubar o processo: sinalizar estado "reautorização necessária" (expor em `/readyz` como degraded e na UI do admin), sem panic.

## Calendar

- O Calendar permanece **síncrono** e é o **único Google no caminho de request**. Antes de confirmar qualquer sessão (agendamento direto ou confirmação de pedido do paciente), consultar **freebusy** do terapeuta para o horário pretendido — cobre também eventos não criados pelo Psicoman. Conflito → erro 409, nunca sobrescrever silenciosamente.
- Evento criado sempre inclui: paciente como convidado (pelo email cadastrado) e link de Google Meet.
- Reminders do evento (não notificação própria do sistema) carregam os intervalos configurados (default 1 dia + 30 min, acumulativos).

## Gmail

- Usado só para envio (cobranças em PDF, templates renderizados). Nunca implementar leitura/parsing de inbox — fora de escopo.
- Envio de email é **best-effort**: falha ao enviar não pode bloquear a operação de negócio que o originou (ex: gerar o PDF de cobrança tem que funcionar mesmo se o envio por email falhar). Logar e permitir reenvio manual.

## Drive

- O Drive **deixa de ser backup síncrono**: vira **fallback/DR**, disparado por um **hook no ciclo de backup do PocketBase** (`internal/drivesync`, podendo rodar como 2º processo). O Drive **não** é fonte da verdade nem está no caminho de request.
- Escopo `drive.file` restringe a arquivos criados pela própria aplicação — não há acesso ao Drive pessoal do terapeuta além disso.
- Snapshot de backup do SQLite: sempre cifrado (AES-256-GCM) e compactado antes do upload — nunca subir o `.db` em texto claro.
- Backup incremental por hash (manifesto no Drive), nunca reenviar o acervo inteiro a cada execução.

## IA (Gemini) — nota de passagem

- O foco deste steering é o Google-API (Calendar/Gmail/Drive). A IA (Gemini) não é Google-API aqui: vive em `internal/integration/gemini`, seguindo a **mesma disciplina** de ficar atrás de uma interface com fake nos testes. Regras de dados/opt-in/auditoria da IA estão em `psicoman-seguranca-lgpd.md`.

## Testes

- Toda integração com Google fica atrás de uma interface (`CalendarClient`, `GmailClient`, `DriveClient`) definida no `internal/app` ou no `internal/domain`, nunca chamada direto da lib do Google a partir dos serviços.
- Testes E2E usam fakes/mocks dessas interfaces — nenhum teste automatizado chama a API real do Google.
