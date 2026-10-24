# Stack local em Docker (teste e navegabilidade)

Ambiente descartável para subir o Psicoman — **o binário único** com o
PocketBase embarcado — num container local e limpar tudo depois. Serve para
navegar o sistema e exercitar o contrato `/v1` sem instalar nada além do Docker.
Para o deploy real (produção atrás do Pangolin), ver [`deploy.md`](./deploy.md).

> O Psicoman é **um único processo** (`cmd/psicoman`, PocketBase como framework).
> Não há mais dois binários (admin/portal): a segregação por ator é lógica
> (`/v1/app`, `/v1/portal`, `/v1/admin`). Ver [`architecture.md`](./architecture.md).

## O que compõe

| Arquivo | Papel |
|---------|-------|
| `Dockerfile` | Imagem única do binário `psicoman` (build estático `CGO_ENABLED=0`, SQLite via `modernc`). `ENTRYPOINT psicoman`, default `serve --http 0.0.0.0:8090 --dir /data`. |
| `docker-compose.yml` | Um serviço `psicoman` (porta `8090`), volume `psicoman-local-data` (dados do PocketBase: SQLite, storage, logs). A **chave de cifra de teste** vem por env (`PSICOMAN_CIPHER_*`). |
| `deploy/config.docker.yaml` | Config de exemplo OPCIONAL (monte em `/app/config.yaml` e aponte com `--config` para ligar Google/Gemini no teste). |

## Subir

```bash
docker compose up --build
```

Ao ficar saudável (`/v1/livez`), a stack fica disponível em:

- Admin UI nativa do PocketBase: <http://localhost:8090/_/>
- Contrato `/v1`: <http://localhost:8090/v1/health>, `/v1/livez`, `/v1/readyz`
- Swagger: <http://localhost:8090/v1/swagger> (e `/v1/swagger.json`)

As rotas de ator exigem o header do Pangolin (não há Pangolin no teste local, então
envie-o manualmente):

```bash
# Terapeuta (ator app):
curl -s http://localhost:8090/v1/app/patients -H 'X-Pangolin-User: terapeuta@local'

# Cadastrar um paciente:
curl -s -X POST http://localhost:8090/v1/app/patients \
  -H 'X-Pangolin-User: terapeuta@local' -H 'Content-Type: application/json' \
  -d '{"name":"Ana Teste","phone":"11999990000","email":"ana@local"}'
```

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
