# Psicoman

Sistema de gestão de atendimentos de psicologia — monolito em Go + React, auto-contido, com SQLite por instância.

---

## Status dos requisitos

| # | Requisito | Status |
|---|-----------|--------|
| 1 | Cadastro de pacientes | ✅ Implementado |
| 2 | Área web do psicólogo (agenda, notas, relatórios) | ✅ Implementado |
| 3 | Área web separada para pacientes | ✅ Implementado |
| 4 | Formulário de anamnese para o paciente | ✅ Implementado |
| 5 | GED — upload e download de documentos | ✅ Implementado (psicólogo e paciente) |
| 6 | Interface responsiva | ✅ Implementado (CSS responsivo com grid adaptável) |
| 7 | Integração Google Calendar + Google Meet | ✅ Implementado (OAuth, criação de evento e link Meet) |
| 8 | Agendamento presencial e online | ✅ Implementado (tipo `in_person` ou `online`, Meet link em todos) |
| 9 | Regras configuráveis de cancelamento e reagendamento | ✅ Implementado (min horas, max reagendamentos/mês, permissões do paciente) |
| 10 | Autenticação: psicólogo via Pangolin (headers), paciente via Gmail + JWT | ✅ Implementado |
| 11 | Multi-tenant: SQLite por tenant via header do Pangolin | ✅ Implementado |
| 12 | Logs em JSON com rotação diária | ✅ Implementado (zerolog + lumberjack) |
| 13 | Relatórios mensais de consultas por paciente (suporte a NF) | ✅ Implementado |
| 14 | Gestão financeira: pagamentos a receber, recebidos, custos, saldo | ✅ Implementado |
| 15 | Editor de texto para anotações e relatórios de atendimento | ⚠️ Parcial — textarea funcional; sem rich-text (negrito, itálico etc.) |
| 16 | Testes unitários e de integração | ⚠️ Parcial — cobertura em domain, service e storage; handlers com cobertura básica |
| 17 | Docker Compose com volumes externos para db, ged e logs | ✅ Implementado |

---

## Arquitetura

```
cmd/server/          → ponto de entrada (main.go)
internal/
  domain/            → entidades, tipos e regras de negócio puras
  storage/           → SQLite, repositórios, migrations SQL
  service/           → lógica de aplicação (appointment, auth, finance, ged, google, patient)
  web/               → Gin HTTP: router, handlers, middleware, logger, embed frontend
frontend/            → React (Vite) — SPA servida pelo próprio binário Go
data/                → volume no host (db/, ged/, logs/) — fora do container
```

### Autenticação

| Área | Mecanismo |
|------|-----------|
| Psicólogo / Admin | Headers HTTP injetados pelo proxy Pangolin (`X-User-Id`, `X-User-Email`, `X-User-Role`) |
| Paciente | Gmail OAuth 2.0 → JWT HS256 emitido pelo servidor (`Authorization: Bearer <token>`) |

O Pangolin **não faz parte deste repositório**. A aplicação lê os headers configuráveis e cria/abre o SQLite correspondente ao tenant.

### Multi-tenant

Cada instância serve **um psicólogo**. O header `X-User-Id` define qual `data/db/{tenant_id}.sqlite` usar. Para múltiplos psicólogos, rode instâncias Docker separadas.

---

## Pré-requisitos

- Docker + Docker Compose
- Go 1.23+ e Node.js 20+ (apenas para desenvolvimento local sem Docker)
- Conta Google Cloud com projeto configurado (opcional — integração Calendar/Meet)
- Proxy Pangolin configurado na cloud (apenas para produção)

---

## Configuração passo a passo

### 1. Google Cloud — criar projeto e credenciais OAuth

O sistema usa a API do Google para dois fins independentes:
- **OAuth do paciente**: login com Gmail
- **Google Calendar + Meet**: agenda e links de videochamada do psicólogo

#### 1.1 Criar projeto no Google Cloud Console

1. Acesse [console.cloud.google.com](https://console.cloud.google.com)
2. Clique em **Selecionar projeto → Novo projeto**
3. Dê um nome (ex.: `psicoman`) e clique em **Criar**

#### 1.2 Ativar as APIs necessárias

1. No menu lateral vá em **APIs e serviços → Biblioteca**
2. Ative as seguintes APIs:
   - **Google Calendar API**
   - **People API** (para leitura do perfil no OAuth do paciente)

#### 1.3 Configurar a tela de consentimento OAuth

1. Vá em **APIs e serviços → Tela de consentimento OAuth**
2. Selecione **Externo** e clique em **Criar**
3. Preencha:
   - Nome do app: `Psicoman`
   - Email de suporte: seu email
   - Domínios autorizados: `seu-dominio.com` (ou `localhost` para dev)
4. Em **Escopos**, adicione:
   - `openid`
   - `email`
   - `profile`
   - `https://www.googleapis.com/auth/calendar`
5. Adicione seu email como **usuário de teste** (enquanto em modo de testes)
6. Salve

#### 1.4 Criar credenciais OAuth 2.0

1. Vá em **APIs e serviços → Credenciais → Criar credenciais → ID do cliente OAuth 2.0**
2. Selecione **Aplicativo da Web**
3. Adicione as **URIs de redirecionamento autorizadas**:
   - Para paciente (login com Google):
     ```
     https://seu-dominio.com/api/auth/patient/callback
     http://localhost:8080/api/auth/patient/callback
     ```
   - Para psicólogo (Calendar):
     ```
     https://seu-dominio.com/api/psych/google/callback
     http://localhost:8080/api/psych/google/callback
     ```
4. Clique em **Criar**
5. Copie o **Client ID** e o **Client Secret** — você vai precisar deles nas variáveis de ambiente

> **Importante:** Use o **mesmo par** de Client ID / Client Secret para os dois fluxos OAuth. Os escopos e redirect URLs distintos são suficientes para separar os dois fluxos.

---

### 2. Pangolin — configurar o proxy reverso

O Pangolin é o proxy na cloud que protege o acesso à área do psicólogo. A aplicação não valida credenciais — ela confia nos headers injetados pelo proxy.

#### 2.1 O que o Pangolin deve injetar

Configure seu site/tunnel no Pangolin para injetar os seguintes headers em **todas** as requisições para `/api/psych/*`:

| Header | Valor esperado | Descrição |
|--------|---------------|-----------|
| `X-User-Id` | ID único do tenant (ex.: `dra-ana`) | Define qual banco SQLite usar |
| `X-User-Email` | Email do psicólogo | Registrado como staff no banco |
| `X-User-Role` | `admin` ou vazio | `admin` dá permissões extras; vazio = psicólogo padrão |

> Os nomes dos headers são configuráveis pelas variáveis `PANGOLIN_USER_HEADER`, `PANGOLIN_EMAIL_HEADER` e `PANGOLIN_ROLE_HEADER`.

#### 2.2 Rotas que o Pangolin deve proteger

- **Proteger** (exige autenticação Pangolin): `/api/psych/*`, `/psych`, `/psych/*`
- **Liberar** (acesso público): `/api/auth/patient/*`, `/api/patient/*`, `/patient`, `/patient/*`, `/`, `/assets/*`

#### 2.3 Configuração típica (exemplo com Traefik + Pangolin)

```yaml
# Exemplo conceitual — adapte à sua instalação do Pangolin
labels:
  - "traefik.http.middlewares.psicoman-auth.headers.customrequestheaders.X-User-Id=dra-ana"
  - "traefik.http.middlewares.psicoman-auth.headers.customrequestheaders.X-User-Email=dra.ana@clinica.com"
  - "traefik.http.middlewares.psicoman-auth.headers.customrequestheaders.X-User-Role=admin"
```

---

### 3. Variáveis de ambiente

Crie um arquivo `.env` na raiz do projeto (onde está o `docker-compose.yml`):

```env
# Segredo para assinar os JWTs dos pacientes (use uma string longa e aleatória)
JWT_SECRET=troque-por-valor-aleatorio-longo-aqui

# Credenciais OAuth Google (Client ID e Secret do passo 1.4)
GOOGLE_CLIENT_ID=123456789-abc.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-seu-secret-aqui

# URL de callback do OAuth do PACIENTE (login com Gmail)
# Em produção, substitua localhost pelo seu domínio
GOOGLE_REDIRECT_URL=https://seu-dominio.com/api/auth/patient/callback

# URL de callback do OAuth do PSICÓLOGO (Calendar/Meet)
GOOGLE_PSYCH_REDIRECT_URL=https://seu-dominio.com/api/psych/google/callback

# ID do calendário Google do psicólogo (use "primary" para o calendário principal)
GOOGLE_CALENDAR_ID=primary

# ID do tenant padrão (nome do arquivo SQLite: data/db/{DEFAULT_TENANT_ID}.sqlite)
DEFAULT_TENANT_ID=dra-ana

# Headers que o Pangolin injeta (mude só se personalizar no Pangolin)
PANGOLIN_USER_HEADER=X-User-Id
PANGOLIN_EMAIL_HEADER=X-User-Email
PANGOLIN_ROLE_HEADER=X-User-Role

# Porta e diretório de dados
ADDR=:8080
DATA_DIR=/app/data
```

> **Nunca** commite o `.env` em repositórios públicos. Ele já está no `.gitignore`.

---

### 4. Rodar localmente com `make run` (modo dev — sem Google, sem Pangolin)

Este é o fluxo para testar tudo localmente com um único comando, sem configurar nada além do Docker.

```bash
make run
```

O que acontece:
1. Cria `data/db`, `data/ged` e `data/logs` no host (se não existirem)
2. Faz o build completo da imagem Docker (Go + React embutido)
3. Sobe o container com `DEV_MODE=true` — sem Google OAuth, sem Pangolin

Acesse:
```
Psicólogo: http://localhost:8080/psych
Paciente:  http://localhost:8080/patient/login
```

**Área do psicólogo** — acesso direto, sem autenticação adicional. O servidor usa o `DEFAULT_TENANT_ID=dev` automaticamente.

**Área do paciente** — na tela de login, clique no botão **🛠 Dev** que aparece automaticamente quando `DEV_MODE` está ativo. Preencha nome e email fictícios e clique em "Entrar sem Google". Um paciente é criado/reutilizado e você entra diretamente.

Para parar:
```bash
make stop
# ou Ctrl+C se estiver em foreground
```

Para subir em background:
```bash
make run-bg
docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f
```

> **O `DEV_MODE` está ativado apenas no `docker-compose.dev.yml`**, que só é carregado pelos targets `make run` e `make run-bg`. O `docker-compose.yml` padrão não tem `DEV_MODE`, então ele nunca vai para produção acidentalmente.

---

### 5. Rodar em produção com Docker Compose

### 5. Rodar em produção com Docker Compose

#### 5.1 Criar pastas de dados no host

```bash
mkdir -p data/db data/ged data/logs
```

Essas pastas ficam **fora** do container e persistem mesmo que o container seja destruído.

#### 5.2 Build e start

```bash
docker compose up -d --build
```

#### 5.3 Verificar logs

```bash
docker compose logs -f psicoman
```

#### 5.4 Parar

```bash
docker compose down
```

---

## Desenvolvimento local

### 5.1 Instalar dependências

```bash
# Backend Go
go mod download

# Frontend React
cd frontend && npm install && cd ..
```

### 5.2 Modo de desenvolvimento (DEV_MODE)

Em desenvolvimento você não tem o Pangolin rodando, então não há headers `X-User-*` nas requisições. O modo `DEV_MODE=true` resolve isso de três formas:

1. **Área do psicólogo sem Pangolin** — adicione o header `X-Dev-Auth: <DEV_SECRET>` nas suas requisições para `/api/psych/*`. O servidor aceita esse header como substituto dos headers do Pangolin e autentica como admin do tenant padrão.

2. **JWT de paciente sem Google OAuth** — use as rotas `/api/dev/*` para criar pacientes e obter tokens JWT diretamente, sem precisar passar pelo fluxo de login com Gmail.

3. **Fallback silencioso** — se você acessar `/api/psych/*` sem nenhum header (nem Pangolin nem `X-Dev-Auth`), o servidor ainda usa o `DEFAULT_TENANT_ID` e o email `admin@local.dev`. Útil para testar pelo navegador rapidamente.

> **Segurança:** As rotas `/api/dev/*` e o comportamento do `X-Dev-Auth` só existem quando `DEV_MODE=true`. Em produção, nunca defina essa variável.

### 5.3 Configurar e rodar em dev

O arquivo `.env.dev` já está pronto com valores padrão para desenvolvimento:

```bash
# Revise/ajuste o .env.dev se quiser mudar tenant, secret etc.
cat .env.dev

# Compilar e rodar carregando as variáveis do .env.dev
make run-dev
```

O servidor avisa no log quando está em modo dev:
```
WARN  DEV_MODE enabled — dev routes active at /api/dev/*. DO NOT use in production.
```

### 5.4 Rotas disponíveis em DEV_MODE

```
GET  /api/dev/status                → confirma dev mode ativo, mostra config
POST /api/dev/patient-token         → emite JWT para patient_id + email fornecidos
POST /api/dev/create-patient        → cria paciente e já retorna o JWT pronto
```

Todas as rotas `/api/dev/*` exigem o header `X-Dev-Auth: <DEV_SECRET>`.

### 5.5 Fluxo completo de teste local

**Passo 1 — confirmar que o servidor está em dev mode:**
```bash
curl http://localhost:8080/api/dev/status
```
```json
{
  "dev_mode": true,
  "default_tenant_id": "dev",
  "google_configured": false,
  "hint_psych": "Add header X-Dev-Auth: <DEV_SECRET> to any /api/psych/* request",
  "hint_patient": "POST /api/dev/create-patient to get a patient JWT"
}
```

**Passo 2 — acessar a API do psicólogo:**
```bash
# Listar pacientes (substitua minha-chave-dev-local pelo valor em DEV_SECRET)
curl http://localhost:8080/api/psych/patients \
  -H "X-Dev-Auth: minha-chave-dev-local"

# Criar paciente
curl -X POST http://localhost:8080/api/psych/patients \
  -H "X-Dev-Auth: minha-chave-dev-local" \
  -H "Content-Type: application/json" \
  -d '{"name": "João Silva", "email": "joao@teste.com", "phone": "11999999999"}'
```

**Passo 3 — criar paciente e obter JWT para testar a área do paciente:**
```bash
curl -X POST http://localhost:8080/api/dev/create-patient \
  -H "X-Dev-Auth: minha-chave-dev-local" \
  -H "Content-Type: application/json" \
  -d '{"name": "Maria Souza", "email": "maria@teste.com"}'
```
```json
{
  "patient": { "id": "abc123", "name": "Maria Souza", ... },
  "token": "eyJhbGci..."
}
```

**Passo 4 — usar o JWT para testar a área do paciente:**
```bash
TOKEN="eyJhbGci..."

# Ver perfil
curl http://localhost:8080/api/patient/me \
  -H "Authorization: Bearer $TOKEN"

# Ver horários disponíveis
curl http://localhost:8080/api/patient/slots \
  -H "Authorization: Bearer $TOKEN"

# Agendar consulta
curl -X POST http://localhost:8080/api/patient/appointments \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type": "online", "scheduled_at": "2026-06-10T10:00:00Z", "duration_minutes": 50}'
```

**Alternativa: se você tiver o JWT do paciente salvo num token já emitido:**
```bash
# POST /api/dev/patient-token → só gera token, não cria paciente
curl -X POST http://localhost:8080/api/dev/patient-token \
  -H "X-Dev-Auth: minha-chave-dev-local" \
  -H "Content-Type: application/json" \
  -d '{"patient_id": "id-que-ja-existe", "email": "joao@teste.com"}'
```

### 5.6 Rodar apenas a API (sem frontend embutido)

```bash
make run
```

A rota `/` retorna apenas uma mensagem de texto. Use esse modo quando quiser rodar o frontend separadamente com hot-reload:

```bash
# Terminal 1 — API
make run

# Terminal 2 — Frontend com hot-reload
cd frontend && npm run dev
```

O Vite proxy está configurado em `frontend/vite.config.js` para redirecionar `/api/*` para `http://localhost:8080`.

### 5.7 Rodar com frontend embutido

```bash
make build
./bin/psicoman
```

### 5.8 Acessar

```
Psicólogo: http://localhost:8080/psych
Paciente:  http://localhost:8080/patient/login
```

### 5.9 Rodar testes

```bash
make test
```

---

### 6. Conectar o Google Calendar (primeiro uso)

Esta etapa é feita **uma vez** pelo psicólogo após o sistema estar rodando:

1. Acesse `/psych/settings`
2. Clique em **Conectar Google**
3. Faça login com a conta Google do psicólogo e autorize os escopos de Calendar
4. Você será redirecionado de volta para `/psych/settings?google=connected`

A partir daí, todos os novos agendamentos criarão automaticamente eventos no Google Calendar com link do Google Meet.

> **Nota:** O token de acesso é salvo no banco SQLite do tenant (`data/db/{tenant}.sqlite`). Se o banco for apagado, será necessário reconectar.

---

### 7. Estrutura de dados no host

```
data/
├── db/
│   └── dra-ana.sqlite      # banco do tenant; um arquivo por psicólogo
├── ged/
│   └── dra-ana/
│       └── {patient_id}/   # documentos organizados por paciente
│           └── arquivo.pdf
└── logs/
    └── psicoman-2026-06-06.json   # rotação diária, compressão automática
```

---

### 8. Primeiro acesso — paciente

O paciente pode criar sua conta de duas formas:

**Via Gmail (recomendado):**
1. Acesse `/patient/login`
2. Clique em **Entrar com Google**
3. Autorize o acesso ao email/perfil
4. Na primeira entrada, uma conta é criada automaticamente

**Via cadastro manual (sem Google):**
```bash
curl -X POST https://seu-dominio.com/api/auth/patient/register \
  -H "Content-Type: application/json" \
  -d '{"name": "João Silva", "email": "joao@email.com"}'
```
> Cadastro manual ainda não gera token de acesso automaticamente — o paciente precisará vincular o Gmail posteriormente para fazer login.

---

### 9. Múltiplos psicólogos / instâncias

O sistema é projetado para rodar **uma instância por psicólogo**. Para adicionar um segundo psicólogo:

1. Crie uma nova pasta de dados no host:
   ```bash
   mkdir -p /opt/psicoman-dr-carlos/data/{db,ged,logs}
   ```

2. Crie um novo `docker-compose.yml` com `DEFAULT_TENANT_ID=dr-carlos` e porta diferente (ex.: `8081:8080`)

3. Configure uma nova rota no Pangolin apontando para a segunda instância

---

## API — referência rápida

### Público
```
GET  /api/auth/patient/url            → URL de login Google do paciente
GET  /api/auth/patient/callback       → Callback OAuth (redireciona com JWT)
POST /api/auth/patient/register       → Cadastro manual de paciente
```

### Psicólogo (`/api/psych/*` — requer headers Pangolin)
```
GET    /me
GET    /patients
POST   /patients
GET    /patients/:id
GET    /patients/:id/report
GET    /appointments?from=&to=&patient_id=
POST   /appointments
PATCH  /appointments/:id/cancel
PATCH  /appointments/:id/reschedule
PATCH  /appointments/:id/notes
PATCH  /appointments/:id/complete
GET    /scheduling-rules
PUT    /scheduling-rules
GET    /documents?patient_id=
POST   /documents          (multipart/form-data)
GET    /documents/:id/download
GET    /finance/summary?month=&year=
GET    /finance/reports/monthly?month=&year=
POST   /finance/payments
POST   /finance/payments/:id/receive
POST   /finance/costs
GET    /google/auth
GET    /google/callback
```

### Paciente (`/api/patient/*` — requer `Authorization: Bearer <jwt>`)
```
GET   /me
GET   /appointments
GET   /slots
POST  /appointments
PATCH /appointments/:id/cancel
PATCH /appointments/:id/reschedule
PUT   /anamnesis
GET   /documents
POST  /documents          (multipart/form-data)
GET   /documents/:id/download
```

---

## Variáveis de ambiente — referência completa

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `ADDR` | `:8080` | Endereço e porta do servidor |
| `DATA_DIR` | `./data` | Pasta base para db, ged e logs |
| `JWT_SECRET` | `change-me-in-production` | Segredo HMAC para tokens de paciente |
| `GOOGLE_CLIENT_ID` | — | Client ID do OAuth Google |
| `GOOGLE_CLIENT_SECRET` | — | Client Secret do OAuth Google |
| `GOOGLE_REDIRECT_URL` | `http://localhost:8080/api/auth/patient/callback` | Callback OAuth do paciente |
| `GOOGLE_PSYCH_REDIRECT_URL` | `http://localhost:8080/api/psych/google/callback` | Callback OAuth do psicólogo (Calendar) |
| `GOOGLE_CALENDAR_ID` | `primary` | ID do calendário Google para eventos |
| `PANGOLIN_USER_HEADER` | `X-User-Id` | Header com ID do tenant (psicólogo) |
| `PANGOLIN_EMAIL_HEADER` | `X-User-Email` | Header com email do psicólogo/admin |
| `PANGOLIN_ROLE_HEADER` | `X-User-Role` | Header com role (`admin` ou vazio) |
| `DEFAULT_TENANT_ID` | `default` | Tenant quando header está ausente (dev) |

---

## Logs

Todos os acessos HTTP são logados em JSON com rotação diária automática:

```
data/logs/psicoman-YYYY-MM-DD.json
```

Exemplo de entrada de log:
```json
{"level":"info","time":"2026-06-06T10:00:00Z","method":"POST","path":"/api/psych/appointments","status":201,"duration_ms":12,"ip":"10.0.0.1","user":"dra-ana","message":"request"}
```

Os logs são comprimidos após a rotação e mantidos por 90 dias (configurável no código em `internal/web/logger.go`).

---

## Testes

```bash
make test
```

Cobertura atual:
- `internal/domain` — regras de cancelamento e reagendamento (unitários)
- `internal/service` — criação e cancelamento de appointments (unitários com mock de calendar)
- `internal/storage` — repositórios SQLite (integração com banco em memória)
- `internal/web` — handlers HTTP (integração com servidor de teste)
