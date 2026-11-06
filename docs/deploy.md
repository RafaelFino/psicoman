# Psicoman — Guia de Instalação e Deploy

Guia para colocar uma instância do Psicoman no ar. Cada terapeuta roda **uma
instância isolada** (uma VM por terapeuta): **um único binário** (`bin/psicoman`,
com o PocketBase embarcado) atrás do gateway **Pangolin**.

> Arquitetura vigente (onda `pocketbase-core` + `pocketbase-finish`): processo
> único, segregação por ator **lógica** (`/v1/app`, `/v1/portal`, `/v1/admin`),
> cifra por campo (AES-256-GCM), Google via OAuth 3-legged (`calendar` +
> `drive.file`, **sem Gmail**). Ver [`architecture.md`](./architecture.md) e
> [`requirements.md`](./requirements.md).

---

## Visão geral do que você vai montar

```mermaid
flowchart TB
    Paciente -->|internet, TLS| Borda[nginx de borda<br/>só /v1/portal/*]
    Terapeuta -->|header de identidade| Pangolin
    Operador -->|header de identidade| Pangolin
    Borda -->|/v1/portal/*| Psicoman[bin/psicoman :8090]
    Pangolin -->|/v1/app/* e /v1/admin/*| Psicoman
    Psicoman --> DB[(SQLite — PocketBase)]
    Psicoman --> GED[/GED no storage do PocketBase/]
    Psicoman --> Google[Google Calendar/Meet + Drive DR]
```

- O binário escuta **apenas em loopback** (`127.0.0.1:8090`) e **nunca** é
  exposto direto à internet. Duas superfícies distintas chegam a ele:
  - **Portal do paciente** (`/v1/portal/*`): exposto à internet por um **nginx
    de borda endurecido** (rate limit, limite de corpo, timeouts), que encaminha
    SOMENTE o portal e as sondas de saúde e **nega** `/v1/app`, `/v1/admin`,
    `/api/` e `/_/`. É a única superfície pública.
  - **Terapeuta/operação** (`/v1/app/*`, `/v1/admin/*`): atrás do **Pangolin**,
    que injeta o header de identidade; nunca passa pelo nginx público.
- A autenticação do paciente é da própria aplicação (sessão real do PocketBase)
  + o gate de aprovação; o nginx apenas endurece a borda e segrega a exposição.

---

## 1. Pré-requisitos

| Ferramenta | Uso | Como checar |
|-----------|-----|-------------|
| Go 1.27+  | compilar o binário | `go version` |
| git       | clonar/atualizar o código | `git --version` |
| openssl   | gerar a chave de cifra | `openssl version` |
| curl      | validação de saúde | `curl --version` |
| systemd   | rodar como serviço | `systemctl --version` |

Uma conta **Google pessoal** (Google One serve) do terapeuta — sem necessidade
de Google Workspace nem domain-wide delegation.

---

## 2. Provisão da VM

Qualquer Linux recente com systemd serve (2 vCPU / 2 GB RAM / 20 GB disco é
confortável para um consultório).

```bash
sudo apt update && sudo apt -y upgrade
sudo apt -y install git curl openssl
# Go 1.27+: baixe de https://go.dev/dl/ e ajuste o PATH, se o repo não tiver.

git clone https://github.com/rafaelfino/psicoman.git
cd psicoman
```

Mantenha o relógio da VM sincronizado (NTP). O sistema opera internamente em
`America/Sao_Paulo`.

---

## 3. Compilar e configurar

### 3.1 Compilar o binário único

```bash
make tools        # uma vez: instala o golangci-lint v2 (compilado com o Go local)
make build        # gera bin/psicoman (go build -o bin/psicoman ./cmd/psicoman)
make check        # gate completo: fmt + vet + build + testes + lint (obrigatório)
```

> O `make check` roda o `golangci-lint` v2 como **gate obrigatório** (não é mais
> advisory). O `make tools` o instala compilando a versão fixada com o toolchain
> Go local — binários pré-compilados não decodificam o export data do Go 1.27.
> Para pular o lint pontualmente: `make check SKIP_LINT=1` (desaconselhado).

### 3.2 Gerar o `config.yaml`

Copie o exemplo e preencha. **Nunca** versione o `config.yaml` com segredos.

```bash
cp config.example.yaml config.yaml
openssl rand -base64 32    # gere a chave de cifra (32 bytes) e cole em crypto.keys.k1
```

Campos (ver [`../config.example.yaml`](../config.example.yaml)):

- `crypto.key_id` / `crypto.keys` — **obrigatório** (fail-closed: sem cifra o
  boot aborta). `key_id` aponta a chave corrente; `keys` é um mapa `id -> base64`
  (chaves antigas ficam para decifrar o que foi gravado antes de uma rotação).
- `pangolin.identity_header` — nome do header de identidade (default
  `X-Pangolin-User`). Não é segredo.
- `pangolin.dev_identity` — **só para DEV** (vazio em produção, fail-closed). Quando
  preenchido, relaxa o gate do Pangolin e usa esse valor como ator nas requisições
  sem o header real. **Atenção:** esse ator **não é uma identidade verificada pela
  borda** — é um valor de config. Por isso o `audit_log` em modo dev mostra a
  identidade configurada, não uma autenticação real. Em produção, mantenha vazio.
- `google.*` — `client_id`/`client_secret`/`calendar_id`/`drive_folder`. Vazios ⇒
  Calendar/Drive degradam (no-op, sem evento/espelho), sem derrubar o processo.
- `llm.gemini.api_key` — opt-in da IA pela **presença** da chave. Vazio ⇒ IA off.

> Alternativa a arquivo: toda chave tem env equivalente e **env sobrescreve o
> arquivo** (ex.: `PSICOMAN_CIPHER_KEY_ID`, `PSICOMAN_CIPHER_KEYS`,
> `PSICOMAN_PANGOLIN_IDENTITY_HEADER`, `PSICOMAN_GEMINI_API_KEY`). A origem de
> cada chave (arquivo|env|default) é **logada no boot sem vazar o valor**.

---

## 4. Credenciais do Google (OAuth 3-legged)

O Psicoman usa **OAuth 3-legged** com refresh token (conta pessoal). Você cria um
projeto no Google Cloud, habilita as APIs e gera um **Client ID + Client Secret**.

### 4.1 Projeto + APIs

1. <https://console.cloud.google.com/> → novo projeto (ex.: `psicoman-<seu-nome>`).
2. **APIs e Serviços → Biblioteca**, habilite:
   - **Google Calendar API**
   - **Google Drive API**
   (Gmail **não** é usado — a cobrança é PDF no GED, enviada pela terapeuta fora da plataforma.)

### 4.2 Tela de consentimento

1. **APIs e Serviços → Tela de permissão OAuth** → tipo **Externo**.
2. Em **Escopos**, adicione exatamente os mínimos:
   - `https://www.googleapis.com/auth/calendar`
   - `https://www.googleapis.com/auth/drive.file`
3. Em **Usuários de teste**, adicione o e-mail do terapeuta. Em modo de teste, só
   esses e-mails autorizam — suficiente para uma instância pessoal.

> Referência oficial: [Configurar a tela de consentimento OAuth](https://developers.google.com/workspace/guides/configure-oauth-consent). _Conteúdo adaptado das docs do Google._

### 4.3 Credenciais OAuth

1. **Credenciais → Criar credenciais → ID do cliente OAuth** → **Aplicativo da Web**.
2. Em **URIs de redirecionamento autorizados**, adicione a URL de callback (a
   mesma do domínio público), por exemplo:

   ```
   https://psicoman.seudominio.com.br/v1/admin/google/callback
   ```

3. Copie o **Client ID** e o **Client Secret** para `google.client_id` /
   `google.client_secret` do `config.yaml`. **Nunca** versione.

### 4.4 Autorizar (depois de subir — passo 6)

A autorização é uma vez só, pelo navegador do operador (o fluxo redireciona ao
consentimento do Google e volta no callback):

1. Acesse, autenticado pelo Pangolin, a rota de autorização:
   `https://psicoman.seudominio.com.br/v1/admin/google/authorize`
   — ela redireciona ao consentimento do Google (`access_type=offline`,
   `prompt=consent`, escopos `calendar`+`drive.file`).
2. Conceda o acesso. O Google chama de volta
   `/v1/admin/google/callback?code=...`, o Psicoman troca o code pelo **refresh
   token** e o persiste **cifrado** na coleção `google_token`.
3. Confira o estado:
   `curl https://psicoman.seudominio.com.br/v1/admin/google/status`
   → `{"state":"ok"}` quando autorizado; `degraded` se pendente.

A partir daí o access token é renovado automaticamente. Se a renovação falhar no
futuro, `/v1/readyz` passa a reportar `google: degraded` (reautorização
necessária) — basta repetir este passo. O refresh token nunca aparece em resposta
nem em log.

---

## 5. Borda: duas superfícies separadas

O binário escuta só em `127.0.0.1:8090`. Duas camadas de borda distintas chegam
a ele, com exposições diferentes:

### 5.1 Portal do paciente → nginx público endurecido

O **único** ponto exposto à internet. Um `nginx` à frente encaminha SOMENTE o
portal (`/v1/portal/*`) e as sondas de saúde, e **nega explicitamente** tudo o
mais (`/v1/app`, `/v1/admin`, `/api/`, `/_/`, `/v1/swagger`). A config de
referência está em [`../deploy/nginx/portal.conf`](../deploy/nginx/portal.conf)
e é a mesma usada na stack local (`docker-compose.yml`). Ela aplica:

- **rate limit** por IP nas rotas do portal (ex.: 30 req/min, burst 20 → 429);
- **limite de corpo** (2 MB) e **timeouts curtos** (mitiga slowloris);
- **limite de conexões** por IP;
- limpeza do header de identidade do Pangolin vindo de fora (anti-spoofing);
- cabeçalhos de segurança (`X-Content-Type-Options`, `X-Frame-Options`, etc.).

TLS: o nginx local fala HTTP; em produção quem termina TLS é a camada acima
(o próprio Pangolin/LB também pode cobrir o domínio do portal, ou monte um cert
no nginx — ver o `.conf`). O app **também** impõe suas regras (gate de
aprovação, validação), então o nginx é defesa em profundidade, não a única.

### 5.2 Terapeuta e operação → Pangolin

`/v1/app/*` e `/v1/admin/*` ficam atrás do **Pangolin**, que termina o TLS e
injeta o **header de identidade** (o mesmo `pangolin.identity_header` do config;
default `X-Pangolin-User`). Ausente/vazio ⇒ a aplicação responde **401**. Essas
superfícies **nunca** passam pelo nginx público.

Configuração do Pangolin (nomes variam conforme a versão):

1. **Upstream**: `http://127.0.0.1:8090` (onde o binário escuta).
2. **Domínio**: ex. `psicoman-admin.seudominio.com.br` (separado do domínio do portal).
3. **Identidade**: injetar o header de e-mail do terapeuta nos prefixos `/v1/app` e `/v1/admin`.
4. **DNS**: aponte os domínios (portal → nginx; admin → Pangolin).

> Como o binário escuta só em loopback, ele **não** fica acessível direto da
> internet: o portal passa pelo nginx; o app/admin, pelo Pangolin.

---

## 6. Rodar como serviço (systemd)

Um **único** serviço. Crie um usuário de serviço e a unit:

```bash
sudo useradd --system --home /opt/psicoman --shell /usr/sbin/nologin psicoman || true
sudo mkdir -p /opt/psicoman /var/lib/psicoman
sudo cp bin/psicoman /opt/psicoman/psicoman
sudo cp config.yaml /opt/psicoman/config.yaml
sudo chown -R psicoman:psicoman /opt/psicoman /var/lib/psicoman
sudo chmod 600 /opt/psicoman/config.yaml
```

`/etc/systemd/system/psicoman.service`:

```ini
[Unit]
Description=Psicoman (binário único, PocketBase embarcado)
After=network-online.target
Wants=network-online.target

[Service]
User=psicoman
Group=psicoman
WorkingDirectory=/opt/psicoman
# A config de cifra pode vir do config.yaml (--config) ou de env (fail-closed).
ExecStart=/opt/psicoman/psicoman serve --http 127.0.0.1:8090 --dir /var/lib/psicoman --config /opt/psicoman/config.yaml
Restart=on-failure
RestartSec=3
# Endurecimento básico:
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/psicoman
ProtectHome=true

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now psicoman
```

As **migrations do MER são aplicadas automaticamente no boot** (o PocketBase roda
`RunAllMigrations`). A admin UI nativa fica em `/_/` (crie o superusuário com
`psicoman superuser upsert <email> <senha> --dir /var/lib/psicoman`).

### Verificação rápida

```bash
# Na VM (loopback):
curl -s http://127.0.0.1:8090/v1/livez    # {"data":{"status":"alive"}}
curl -s http://127.0.0.1:8090/v1/readyz   # ready:true (google degraded até autorizar)

# Pelo domínio (após o Pangolin):
curl -s https://psicoman.seudominio.com.br/v1/livez
```

Swagger: `https://psicoman.seudominio.com.br/v1/swagger`.

### Primeiro uso (jornada ponta a ponta)

1. Autorize o Google (passo 4.4).
2. Configure o perfil do terapeuta (aparece no cabeçalho da cobrança):

   ```bash
   curl -X PUT https://psicoman.seudominio.com.br/v1/app/therapist \
     -H 'X-Pangolin-User: terapeuta@seudominio.com.br' -H 'Content-Type: application/json' \
     -d '{"name":"Dra. Fulana","crp":"06/12345","email":"terapeuta@seudominio.com.br"}'
   ```

3. Cadastre uma **origem**, um **local**, um **paciente** e um **plano** (por
   consulta), agende uma **sessão**, finalize-a (o débito é gerado
   automaticamente pelo plano vigente) e gere a **cobrança** em PDF:

   ```bash
   H='-H X-Pangolin-User:terapeuta@seudominio.com.br -H Content-Type:application/json'
   base=https://psicoman.seudominio.com.br/v1/app
   # paciente
   pid=$(curl -s $H -X POST $base/patients -d '{"name":"Ana","phone":"11999990000","email":"ana@x.com"}' | jq -r .data.id)
   # plano por consulta
   curl -s $H -X POST $base/patients/$pid/plans -d '{"type":"pagamento_por_consulta","amount_cents":18000,"starts_at":"2026-01-01T00:00:00-03:00"}'
   # sessão + finalizar
   sid=$(curl -s $H -X POST $base/sessions -d '{"patient_id":"'$pid'","modality":"online","starts_at":"2026-06-10T14:00:00-03:00","ends_at":"2026-06-10T15:00:00-03:00"}' | jq -r .data.id)
   curl -s $H -X POST $base/sessions/$sid/finish -d '{"bill":true}'
   # débito gerado → cobrança
   did=$(curl -s $H $base/patients/$pid/debts | jq -r '.data[0].id')
   curl -s $H -X POST $base/debts/$did/invoice
   ```

---

## 7. Atualizar com segurança

Atualização manual, com backup antes e validação depois:

```bash
# 1. Backup do data dir (SQLite + storage) a quente via a rota de backup do PocketBase:
curl -s -X POST https://psicoman.seudominio.com.br/v1/admin/backup \
  -H 'X-Pangolin-User: terapeuta@seudominio.com.br'
#    (ou copie /var/lib/psicoman com o serviço parado).

# 2. Nova versão + recompila. Se a compilação falhar, NÃO toque em produção.
cd /caminho/para/psicoman && git fetch && git checkout <tag>
make build

# 3. Troca o binário e reinicia (migrations aplicam no boot).
sudo systemctl stop psicoman
sudo cp bin/psicoman /opt/psicoman/psicoman
sudo systemctl start psicoman

# 4. Valida. Se /readyz não ficar pronto, restaure o binário anterior e o backup.
curl -s http://127.0.0.1:8090/v1/readyz
```

A **chave de cifra** vive apenas no `config.yaml` (ou no env). Guarde uma cópia
segura **fora da VM** — sem ela, nem os dados nem os backups cifrados são
recuperáveis.

> Os scripts shell de instalação/atualização do modelo antigo (dois binários)
> foram **removidos** nesta onda; a instalação e a atualização seguem os passos
> manuais acima (compilar, trocar o binário, reiniciar o systemd, validar
> `/v1/readyz`).

---

## 8. Operação do dia a dia

```bash
sudo systemctl status psicoman
sudo journalctl -u psicoman -f
sudo systemctl restart psicoman
```

Observabilidade (fora da auth, para probes):

- `/v1/livez` — o processo está vivo (200 fixo).
- `/v1/readyz` — cifra montada + DB acessível + estado do OAuth Google
  (`google: ok|degraded`). 503 se cifra ou DB falham.
- `/v1/health` — smoke simples no envelope.

O timing por rota é logado (não há endpoint de métricas dedicado nesta versão —
decisão registrada em [`decisions.md`](./decisions.md)).

---

## 9. Solução de problemas

| Sintoma | Provável causa | O que fazer |
|--------|----------------|-------------|
| Boot aborta na partida | cifra ausente/inválida (fail-closed) | Defina `crypto.key_id`+`crypto.keys` no config ou `PSICOMAN_CIPHER_*` |
| `/v1/readyz` com `google: degraded` | refresh token do Google ausente/expirado | Autorize/reautorize (passo 4.4) |
| `/v1/app/*` responde `401` sempre | header de identidade do Pangolin não chega | Confira o `identity_header` e a injeção no Pangolin |
| Portal responde `401` em `/v1/portal/*` | sessão ausente/expirada | Faça login de novo (auth-with-password) |
| Sessão não vira evento no Calendar | Google não autorizado ou `calendar_id` vazio | Autorize o Google e preencha `google.calendar_id` |

---

## 10. Checklist final

- [ ] VM provisionada, Go 1.27+, relógio sincronizado.
- [ ] `config.yaml` com a chave de cifra (nunca versionada) — cópia da chave num cofre fora da VM.
- [ ] Projeto Google: Calendar + Drive habilitados; consentimento com escopos mínimos; Client ID/Secret no config; redirect `/v1/admin/google/callback` cadastrado.
- [ ] Pangolin: upstream `127.0.0.1:8090`, identidade injetada em `/v1/app` e `/v1/admin`, DNS apontado.
- [ ] `systemctl enable --now psicoman`; `/v1/readyz` pronto.
- [ ] Google autorizado (`/v1/admin/google/status` = ok); perfil do terapeuta configurado.
- [ ] Jornada de ponta a ponta: origem → local → paciente → plano → sessão → finalizar → débito → cobrança.
