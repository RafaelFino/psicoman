---
inclusion: manual
---

# Deployment com Pangolin

## Arquitetura de rede

```
┌─────────────────────────────────────────────────────────────────┐
│  Server X99 (rede local / doméstica)                             │
│                                                                   │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │  Pangolin                                                  │   │
│  │  - TLS termination (Let's Encrypt / Cloudflare tunnel)     │   │
│  │  - Reverse proxy para localhost:8080                        │   │
│  │  - Auth para área do psicólogo                             │   │
│  │  - Injeta headers: X-User-Id, X-User-Email, X-User-Role   │   │
│  └─────────────────────┬─────────────────────────────────────┘   │
│                         │ localhost                                │
│  ┌─────────────────────▼─────────────────────────────────────┐   │
│  │  Docker: psicoman :8080                                    │   │
│  │  - API Go (Gin)                                            │   │
│  │  - Interface HTML (Go templates + htmx + Alpine.js)        │   │
│  │  - SQLite em ./data/db/                                    │   │
│  │  - Documentos em ./data/ged/                               │   │
│  └───────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

**Nota**: Pangolin e o container Docker rodam no mesmo servidor físico (X99). Não há tunnel entre máquinas — o proxy é local (localhost:8080).

## Dois endpoints

### 1. Endpoint público (pacientes)
- URL: `https://pacientes.seudominio.com` (ou path /patient)
- Sem auth do Pangolin — qualquer pessoa acessa
- Auth feita pelo próprio app via Google OAuth → JWT
- Rotas: `/patient/*`, `/api/auth/patient/*`, `/api/patient/*`

### 2. Endpoint privado (psicólogo)
- URL: `https://admin.seudominio.com` (ou path /psych)
- Pangolin exige login antes de permitir acesso
- Pangolin injeta headers de identidade no request
- Rotas: `/psych/*`, `/api/psych/*`

## Headers do Pangolin

O Go lê estes headers (configuráveis via env):
- `PANGOLIN_USER_HEADER` (default: `X-User-Id`) → tenant_id
- `PANGOLIN_EMAIL_HEADER` (default: `X-User-Email`) → email do psicólogo
- `PANGOLIN_ROLE_HEADER` (default: `X-User-Role`) → role (admin/psychologist)

## Docker Compose para produção

```yaml
services:
  psicoman:
    build: .
    ports:
      - "127.0.0.1:8080:8080"   # só localhost — Pangolin faz o proxy externo
    volumes:
      - ./data/db:/app/data/db
      - ./data/ged:/app/data/ged
      - ./data/logs:/app/data/logs
    environment:
      - ADDR=:8080
      - DATA_DIR=/app/data
      - JWT_SECRET=${JWT_SECRET}  # gerar com: openssl rand -hex 32
      - GOOGLE_CLIENT_ID=${GOOGLE_CLIENT_ID}
      - GOOGLE_CLIENT_SECRET=${GOOGLE_CLIENT_SECRET}
      - GOOGLE_REDIRECT_URL=https://pacientes.seudominio.com/api/auth/patient/callback
      - GOOGLE_PSYCH_REDIRECT_URL=https://admin.seudominio.com/api/psych/google/callback
      - GOOGLE_CALENDAR_ID=primary
      # DEV_MODE NÃO definido = produção
    restart: unless-stopped
```

**Importante**: bind em `127.0.0.1:8080` — a porta não é exposta à rede. Pangolin (rodando no mesmo X99) faz o proxy local.

## Backup

- SQLite: copiar `data/db/*.sqlite` (com WAL checkpoint antes)
- Documentos: rsync de `data/ged/`
- Cron sugerido: backup diário para Object Storage (OCI)

## Checklist de deploy

1. [ ] Gerar JWT_SECRET forte: `openssl rand -hex 32`
2. [ ] Configurar Google Cloud Console (OAuth consent screen + credentials)
3. [ ] Configurar Pangolin no X99 com os dois endpoints (público + privado)
4. [ ] Bind do Docker em 127.0.0.1:8080 (Pangolin faz proxy local)
5. [ ] Não definir DEV_MODE na produção
6. [ ] Verificar volumes montados corretamente
7. [ ] Testar fluxo OAuth completo com redirect URLs corretos
8. [ ] Configurar backup automático do data/
