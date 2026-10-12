# Stack local em Docker (teste e navegabilidade)

Ambiente descartável para subir o Psicoman inteiro (admin + portal) num container
local, com dados de demonstração, e **limpar tudo** depois sem deixar lixo. É um
ambiente de **teste em modo dev**: a autenticação do admin fica desligada e o
login do portal aceita qualquer e-mail. **Nunca use em produção** — o deploy real
está em [`deploy.md`](./deploy.md).

## O que compõe

| Arquivo | Papel |
|---------|-------|
| `Dockerfile` | Imagem única com os dois binários (`psicoman-admin`, `psicoman-portal`), build estático (`CGO_ENABLED=0`, SQLite via `modernc`). |
| `docker-compose.yml` | Serviços `admin` (porta 8080) e `portal` (8081) a partir da mesma imagem, compartilhando o volume `psicoman-local-data` (banco, GED, logs). |
| `deploy/config.docker.yaml` | Config do container em `dev_mode`, dados em `/data`. |
| `scripts/stack-up.sh` | Sobe o compose, espera ficar saudável e aplica o seed. |
| `scripts/seed.sh` | Carga inicial via API (origens, locais, pacientes, planos, custos, sessões). |
| `scripts/stack-down.sh` | Derruba e limpa tudo (containers, rede, volume e imagem local). |

## Subir

```bash
make stack-up          # ou ./scripts/stack-up.sh
```

Ao final, a stack fica disponível em:

- Admin:   http://localhost:8080/app/
- Portal:  http://localhost:8081/app/
- Swagger: http://localhost:8080/v1/swagger e http://localhost:8081/v1/swagger

Para subir **sem** carga inicial:

```bash
./scripts/stack-up.sh --no-seed
```

## Carga inicial (seed)

O `seed.sh` popula um cenário coerente para navegar o sistema:

- **Origens**: Indicação, Doctoralia, Instagram.
- **Locais**: "Consultório Centro" (presencial) e "Atendimento Online", com
  janelas de disponibilidade de segunda a sexta.
- **Pacientes aprovados** (cadastrados como se fossem pelo terapeuta): Ana, Bruno
  e Carla, cada um ligado a uma origem.
- **Paciente pendente**: "Diego Pendente" entra pelo **auto-cadastro do portal** e
  nasce `pendente` — use a tela **Aprovações** do admin para liberá-lo e ver o
  gate de aprovação em ação.
- **Planos**: um por consulta, um fechado mensal e um atendimento social.
- **Custos**: categorias (Plataformas, Infraestrutura) e itens, incluindo um item
  ligado à origem Doctoralia (base do relatório de ROI por canal).
- **Sessões**: duas sessões de exemplo em estado `solicitada`.

> As sessões são criadas como `solicitada` de propósito: criar já `agendada`
> acionaria a integração Google (Calendar/Meet), que não está autorizada no
> ambiente local.

Reaplicar o seed numa stack já no ar (ex.: depois de limpar só o banco):

```bash
make stack-seed        # ou ./scripts/seed.sh
```

O seed é tolerante a duplicatas: rodar de novo apenas reavisa o que já existe
(e-mail/CPF únicos continuam valendo).

## Logs

```bash
make stack-logs        # docker compose logs -f
```

## Limpar tudo

```bash
make stack-down        # ou ./scripts/stack-down.sh
```

Remove containers, rede, o **volume de dados** (banco, GED e logs) e a **imagem
local** `psicoman-local:dev`. Depois disso não resta nenhum artefato da stack.
Para preservar a imagem (próximo `stack-up` mais rápido):

```bash
./scripts/stack-down.sh --keep-image
```

## Notas

- Os dois serviços compartilham o mesmo volume e, portanto, o mesmo banco SQLite:
  o que você cria no admin aparece no portal e vice-versa.
- O `/readyz` fica em estado *degraded* (HTTP 503) neste ambiente porque reporta
  a integração Google como pendente de autorização — isso é esperado em dev. A
  prontidão para subir é verificada pelo `/healthz`.
- Portas 8080/8081 precisam estar livres no host.
