# Stack local em Docker (teste e navegabilidade)

Ambiente descartável para subir o Psicoman — **o binário único** com o
PocketBase embarcado — num container local e limpar tudo depois. Serve para
navegar o sistema e exercitar o contrato `/v1` sem instalar nada além do Docker.
Para o deploy real (produção atrás do Pangolin), ver [`deploy.md`](./deploy.md).

> O Psicoman é **um único processo** (`cmd/psicoman`, PocketBase como framework).
> Não há mais dois binários (admin/portal): a segregação por ator é lógica
> (`/v1/app`, `/v1/portal`, `/v1/admin`). Ver [`architecture.md`](./architecture.md).

A stack **espelha a topologia de produção** (Opção A): dois serviços, e a única
porta exposta ao host é a do **nginx de borda**, que só deixa passar o portal do
paciente. As superfícies do terapeuta/operação (`/v1/app`, `/v1/admin`, `/_/`)
**não** são alcançáveis pela porta pública — exatamente como em produção, onde
elas ficam atrás do Pangolin.

## O que compõe

| Arquivo | Papel |
|---------|-------|
| `Dockerfile` | Imagem única do binário `psicoman` (build estático `CGO_ENABLED=0`, SQLite via `modernc`). `ENTRYPOINT psicoman`, default `serve --http 0.0.0.0:8090 --dir /data`. |
| `docker-compose.yml` | Dois serviços: `psicoman` (interno, **sem porta no host**) e `nginx-portal` (publica `8080`, única borda pública). Volume `psicoman-local-data`. A **chave de cifra de teste** vem por env (`PSICOMAN_CIPHER_*`). |
| `deploy/nginx/portal.conf` | Config da borda: encaminha só `/v1/portal/*` + saúde; nega o resto; rate limit, limites de corpo/conexão, timeouts. |
| `deploy/config.docker.yaml` | Config de exemplo OPCIONAL (monte em `/app/config.yaml` e aponte com `--config` para ligar Google/Gemini no teste). |

## Subir

```bash
docker compose up --build
```

Ao ficar saudável, a borda pública fica em **http://localhost:8080** e serve
SOMENTE o portal + saúde:

- Portal do paciente: `POST http://localhost:8080/v1/portal/register`, `GET /v1/portal/me`, etc.
- Saúde: <http://localhost:8080/v1/livez>, `/v1/readyz`, `/v1/health`
- `/v1/app`, `/v1/admin`, `/api/`, `/_/` e `/v1/swagger` respondem **404 na borda** (bloqueados de propósito).

```bash
# Portal (passa pela borda):
curl -s -X POST http://localhost:8080/v1/portal/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ana","phone":"11999990000","email":"ana@local","password":"senha-forte-123"}'

# App bloqueado na borda (esperado 404):
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/v1/app/patients
```

### Acessar o terapeuta/admin localmente

Como a borda só expõe o portal, para exercitar `/v1/app`, `/v1/admin`, o Swagger
ou a admin UI `/_/` no teste local, publique a porta do binário **temporariamente**
(só em dev — nunca em produção):

```bash
# Sobe a stack e publica a porta interna do psicoman no host:
docker compose run --rm --service-ports -d psicoman   # ou adicione `ports: ["8090:8090"]` ao serviço psicoman em dev
# Então o app/admin/_/ ficam em http://localhost:8090 (com o header do Pangolin):
curl -s http://localhost:8090/v1/app/patients -H 'X-Pangolin-User: terapeuta@local'
# Admin UI nativa: http://localhost:8090/_/
```

Alternativa sem Docker: `make serve` (binário direto, todas as rotas em
`127.0.0.1:8090`, sem a separação de borda — ver abaixo).

> O `/v1/readyz` reporta `google: degraded` neste ambiente (sem OAuth autorizado) —
> **esperado em teste**. Isso NÃO impede a prontidão: cifra + DB OK ⇒ `ready:true`.
> A liveness é `/v1/livez`.

### Semear dados para navegar

Não há mais um `seed.sh` acoplado ao modelo antigo. Para popular um cenário,
use o próprio contrato `/v1` (origens, locais, planos, pacientes, sessões) — a
jornada ponta a ponta está documentada em [`deploy.md`](./deploy.md#primeiro-uso)
e no `README.md`. Para operações em coleções trancadas (seed direto), use a
admin UI em `/_/` como superusuário.

## Logs

```bash
docker compose logs -f
```

## Limpar tudo

```bash
docker compose down -v
```

Remove o container, a rede e o **volume de dados** (`psicoman-local-data`).
Para preservar os dados entre execuções, omita o `-v`.

## Alternativa sem Docker (desenvolvimento)

Para iterar no código sem container, rode o binário direto:

```bash
make serve            # build + serve --http 127.0.0.1:8090 --dir ./pb_data
# exige a chave de cifra no ambiente (fail-closed):
PSICOMAN_CIPHER_KEY_ID=k1 \
PSICOMAN_CIPHER_KEYS='k1:wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY=' \
  make serve
```

A admin UI fica em <http://127.0.0.1:8090/_/> e o contrato em `/v1/*`.

## Notas

- Um único processo e um único volume: tudo (app/portal/admin) opera sobre o mesmo
  SQLite do PocketBase.
- As migrations do MER são aplicadas **no boot** (`RunAllMigrations`), não há passo
  manual.
- A porta `8090` precisa estar livre no host.
