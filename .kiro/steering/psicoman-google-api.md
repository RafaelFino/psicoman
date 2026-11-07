---
inclusion: fileMatch
fileMatchPattern: "**/integration/google/**"
---

# Psicoman — Integração Google (Calendar, Meet, Drive)

Decisões de integração em `docs/architecture.md` e `docs/requirements.md`. As integrações vivem em `internal/integration/google` e são consumidas pelo **binário único** `cmd/psicoman` (que roda atrás do `Pangolin`, com segregação admin/portal apenas lógica — não há mais dois binários/processos para admin e portal).

## OAuth

- Conta pessoal do terapeuta (Google One, sem Workspace) → **sem domain-wide delegation**. Fluxo é **OAuth 3-legged**: autorização única do terapeuta, refresh token persistido **cifrado**, renovação automática do access token em runtime.
- Escopos mínimos necessários, nunca mais amplos que o preciso:
  - `https://www.googleapis.com/auth/calendar` (Calendar/Meet)
  - `https://www.googleapis.com/auth/drive.file` (só arquivos criados pelo app — nunca `drive` ou `drive.readonly` full)
- Falha ao renovar o refresh token não pode derrubar o processo: sinalizar estado "reautorização necessária" (expor em `/readyz` como degraded e na UI do admin), sem panic.

## Calendar

- O Calendar permanece **síncrono** e é o **único Google no caminho de request**. Antes de confirmar qualquer sessão (agendamento direto ou confirmação de pedido do paciente), consultar **freebusy** do terapeuta para o horário pretendido — cobre também eventos não criados pelo Psicoman. Conflito → erro 409, nunca sobrescrever silenciosamente.
- Evento criado sempre inclui: paciente como convidado (pelo email cadastrado) e link de Google Meet.
- Reminders do evento (não notificação própria do sistema) carregam os intervalos configurados (default 1 dia + 30 min, acumulativos).

## Envio de e-mail

- **Gmail: fora de escopo** (ver `docs/requirements.md` §3.7 e §5). A cobrança é um PDF no GED e o envio fica com a terapeuta (WhatsApp). Não pedir o escopo `gmail.*` nem criar cliente de e-mail.

## Drive

- O Drive **deixa de ser backup síncrono**: vira **fallback/DR**, disparado por um **hook no ciclo de backup do PocketBase** (`internal/drivesync`, podendo rodar como 2º processo). O Drive **não** é fonte da verdade nem está no caminho de request.
- Escopo `drive.file` restringe a arquivos criados pela própria aplicação — não há acesso ao Drive pessoal do terapeuta além disso.
- Snapshot de backup do SQLite: **snapshot inteiro por execução**, sempre cifrado (AES-256-GCM) antes do upload — nunca subir o `.db` em texto claro. O zip gerado pelo PocketBase já é compactado; o código não recompacta. **Não há backup incremental nem manifesto** (decisão do Rafael, 2026-11-05; ver `docs/decisions.md`).
- **Retenção por contagem** feita pelo `internal/drivesync` após um upload bem-sucedido: mantém os N snapshots mais recentes (`google.drive_retention`, default 14; `0` = retenção manual) e manda os excedentes para a **lixeira** (`trashed`), **nunca** `delete`. Só toca arquivos criados pelo app (`appProperties psicoman_kind=snapshot`, sufixo `.enc`, pasta de DR). Falha de retenção não derruba o backup.
- **Pasta de DR criada automaticamente**: `google.drive_folder` é opcional. Ordem de resolução: config > id persistido (coleção `drive_dr`) > busca por nome/`appProperties` > criação ("Psicoman DR"). Detalhes em `docs/architecture.md §0.10`.

## IA (Gemini) — nota de passagem

- O foco deste steering é o Google-API (Calendar/Drive). A IA (Gemini) não é Google-API aqui: vive em `internal/integration/gemini`, seguindo a **mesma disciplina** de ficar atrás de uma interface com fake nos testes. Regras de dados/opt-in/auditoria da IA estão em `psicoman-seguranca-lgpd.md`.

## Testes

- Toda integração com Google fica atrás de uma interface: os clientes (`CalendarClient`, `DriveClient`) vivem em `internal/integration/google`, com fakes (`FakeDriveClient` etc.) ao lado; os serviços (`internal/app`) só enxergam as **portas** (`ConflictChecker`, `SessionCalendar`) e o `drivesync` só as interfaces locais (`SnapshotStore`, `FolderClient`, `FolderStore`). Nenhum serviço chama a lib do Google direto.
- Testes E2E usam fakes/mocks dessas interfaces — nenhum teste automatizado chama a API real do Google.
