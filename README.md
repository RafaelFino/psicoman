# Psicoman

Sistema de gestão para consultório de psicologia — monolito Go auto-contido com SQLite, interface HTML moderna servida diretamente pelo binário.

Projetado para terapeuta autônomo: agenda, prontuário, evoluções de sessão, anamnese estruturada, contratos terapêuticos, supervisões, espaços/consultórios, documentos, financeiro e integração com Google Calendar. Roda em servidor local exposto via Pangolin (tunnel seguro para VM cloud).

---

## Arquitetura

```
Internet → Pangolin (VM OCI, TLS)
              ├── /patient/* (público, auth via Google OAuth + JWT)
              └── /psych/*   (privado, auth via Pangolin headers)
                    │
                    └── psicoman :8080 (servidor local, rede doméstica)
                         ├── API REST (Go/Gin) → JSON
                         ├── Interface HTML (Go templates + htmx + Alpine.js)
                         ├── SQLite em data/db/
                         └── GED (documentos) em data/ged/
```

| Camada | Tecnologia |
|--------|-----------|
| Backend | Go 1.23, Gin, SQLite (modernc.org/sqlite, pure-Go) |
| Frontend | Go html/template, htmx 2.x, Alpine.js 3.x, CSS custom |
| Auth Paciente | Google OAuth2 → JWT local (7 dias) |
| Auth Psicólogo | Pangolin reverse proxy headers |
| Deploy | Docker (Alpine, ~20MB), Pangolin no host |
| Logs | zerolog + lumberjack (JSON, rotação diária, 90 dias) |
| Banco | SQLite com WAL, foreign keys, migrations auto |

### Decisão: sem Node.js / npm / React

O frontend é servido diretamente pelo Go usando templates HTML com htmx para interações AJAX e Alpine.js para estado client-side local. Tudo é embutido no binário via go:embed. Resultado: build simples (go build), único executável, zero dependências externas em runtime.

---

## Features implementadas

| Feature | Descrição |
|---------|-----------|
| Agenda (dashboard) | Vista diária com consultas, estatísticas, próximos dias |
| Pacientes | CRUD completo, busca, página 360° com histórico completo |
| Agendamentos | Online e presencial, conflitos, regras de cancel/reschedule |
| Evoluções de sessão | Texto clínico + notas privadas + métricas de tempo |
| Anamnese estruturada | Templates personalizáveis (adulto/criança), campos dinâmicos |
| Contrato terapêutico | Templates com placeholders, envio ao paciente, assinatura digital |
| Supervisões | Registro de supervisores, sessões, notas, custos, métricas |
| Espaços/Consultórios | Gestão de salas (fixas/alugadas/temporárias), custos, reservas |
| Google Calendar + Meet | OAuth, criação de eventos, links Meet automáticos |
| GED (documentos) | Upload/download, organizado por paciente/tenant |
| Financeiro | Pagamentos, custos, relatórios mensais por paciente, saldo |
| Responsividade | Mobile-first CSS com sidebar colapsável, touch targets |
| Auth dupla | Pangolin (psicólogo) + Google OAuth/JWT (paciente) |
| Security headers | X-Frame-Options, X-Content-Type-Options, Referrer-Policy |
| DEV_MODE | Login sem Google, rotas debug, seed de dados |

---

## Quick start

### Pré-requisitos

- Go 1.23+
- Docker + Docker Compose (opcional)

### Rodar com Docker

```bash
make run
```

Acesse: http://localhost:8080/psych (psicólogo) ou http://localhost:8080/patient/login (paciente)

### Rodar sem Docker

```bash
make run-api
```

### Testes

```bash
make test
```

---

## Variáveis de ambiente

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| ADDR | :8080 | Endereço do servidor |
| DATA_DIR | ./data | Base para db, ged, logs |
| JWT_SECRET | change-me-in-production | Secret para JWT paciente |
| GOOGLE_CLIENT_ID | — | OAuth Client ID |
| GOOGLE_CLIENT_SECRET | — | OAuth Client Secret |
| DEV_MODE | — | true habilita rotas dev (NUNCA em prod) |
| DEV_SECRET | dev-local | Secret para X-Dev-Auth |

---

## Deploy em produção

1. JWT_SECRET: gerar com openssl rand -hex 32
2. Google Cloud Console: configurar OAuth + Calendar API
3. Pangolin: 2 endpoints (público + privado)
4. Não definir DEV_MODE
5. Volumes: data/db, data/ged, data/logs

```bash
docker compose up -d --build
```

---

## Makefile

| Target | Descrição |
|--------|-----------|
| make build | Compila binário Go em bin/psicoman |
| make run | Docker Compose dev |
| make run-api | Build + run local com .env.dev |
| make test | go test ./... -count=1 |
| make clean | Remove bin/ e data/ |

---

## Documentação adicional

- docs/AUDIT.md — Auditoria completa, bugs, plano de evolução
- .kiro/steering/ — Convenções de código, segurança, arquitetura, deploy
