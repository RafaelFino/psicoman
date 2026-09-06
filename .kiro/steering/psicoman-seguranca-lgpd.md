---
inclusion: fileMatch
fileMatchPattern: "**/api/**,**/service/**,**/platform/crypto/**,**/config/**"
---

# Psicoman — Segurança e dados sensíveis de saúde

O prontuário psicológico é dado sensível de saúde. Este projeto lida com ele fora de um ambiente enterprise (homelab do próprio terapeuta), então os controles abaixo são o mínimo não negociável, não um "nice to have". Decisões completas em `docs/requirements.md §4.1` e `docs/architecture.md §4.5, §8`.

## Autenticação e superfícies

- **Admin** (`psicoman-admin`): confia no Pangolin para roteamento, mas **valida por conta própria** o header de email + secret. Nunca remover essa validação assumindo que "o Pangolin já filtrou" — é defense in depth deliberado.
- **Portal** (`psicoman-portal`): fica atrás do Pangolin, mas **sem o controle de acesso** que o admin tem — o Pangolin ali só termina TLS/HTTPS. A autenticação é da própria aplicação (login social Google). Isso significa que o portal precisa:
  - Ter rate limiting próprio (por IP e por email) nas rotas públicas sem sessão prévia (cadastro, pedido de agendamento), já que o Pangolin não filtra acesso aqui.
  - Nunca expor endpoint algum de dado clínico — só perfil próprio, agenda, sessões e débitos do paciente autenticado.
  - Não assumir que o Pangolin autenticou o paciente — toda rota autenticada valida a sessão da própria aplicação.
- Isolamento de dados do portal é por **email verificado** do OAuth, nunca por um ID passado em query string/body sem cruzar com a sessão autenticada.

## Segredos e criptografia

- Nenhum segredo (secret do admin, client secret OAuth, refresh tokens, chave de cifragem de backup) em log, em resposta de erro, ou em código-fonte. Sempre via `config.yaml` (fora do repo) ou variável de ambiente.
- Refresh tokens do Google e o snapshot de backup do SQLite são sempre cifrados (AES-GCM) antes de persistir/enviar ao Drive.
- A chave de cifragem, no MVP, vem do config — mas sempre atrás da interface `KeyProvider` (`docs/architecture.md §4.4`), para permitir troca por vault sem reescrever quem consome a chave.

## Auditoria

- Toda operação sobre dado sensível grava `audit_log`: leitura/escrita de prontuário (anamnese, notas), geração/quitação de débito, alteração de config, backup/restore, autenticação (sucesso e falha).
- `audit_log` registra o ator (email), a ação, a entidade afetada e o timestamp — nunca o conteúdo clínico em si (não duplicar dados sensíveis no log).

## Multi-tenant (uma instância por terapeuta)

- O sistema **não** implementa multi-tenancy dentro de uma instância — isolamento entre terapeutas é feito por VM/instância separada, não por `tenant_id` no schema. Não introduzir lógica de tenant compartilhado sem revisitar essa decisão em `docs/requirements.md §1`.

## Ao adicionar uma rota ou campo novo

Antes de expor qualquer dado novo via API, perguntar:
1. Esse dado é clínico/sensível? Se sim, só pode estar em rota `/v1/admin/*`, nunca `/v1/portal/*`.
2. Essa operação precisa de audit log?
3. Essa rota pública (portal, sem sessão) precisa de rate limiting?
