---
inclusion: fileMatch
fileMatchPattern: "**/api/**,**/service/**,**/app/**,**/platform/crypto/**,**/platform/pb/**,**/config/**"
---

# Psicoman — Segurança e dados sensíveis de saúde

O prontuário psicológico é dado sensível de saúde. Este projeto lida com ele fora de um ambiente enterprise (homelab do próprio terapeuta), então os controles abaixo são o mínimo não negociável, não um "nice to have". Decisões completas em `docs/requirements.md` e `docs/architecture.md`.

## Autenticação e superfícies

- **Binário único atrás do `Pangolin`.** O sistema é um único processo (`cmd/psicoman`) que roda sempre atrás do `Pangolin` e nunca é exposto direto à internet. A segregação admin/portal é **lógica** (grupos de rota `/v1/admin/*` e `/v1/portal/*` com regras de auth por grupo), não física. Auth **simplificada**: o `Pangolin` é a barreira forte de borda e o app mantém só o mínimo de defense-in-depth.
- **Admin** (grupo `/v1/admin/*`): a identidade do admin chega via **header do `Pangolin`**. Não há mais um binário `psicoman-admin` separado; o enforcement de acesso ao grupo admin vive no middleware do próprio grupo de rota.
- **Portal** (grupo `/v1/portal/*`): expõe de forma controlada apenas cadastro/login do paciente e recursos mínimos (perfil próprio, agenda, sessões e débitos do paciente autenticado). O portal precisa:
  - **Gate de aprovação do paciente** (`pendente`/`aprovado`/`rejeitado`) com enforcement no servidor via **hook**: paciente não aprovado não acessa recurso algum (resposta **403**). Esse gate é no servidor, nunca só na UI.
  - Rate limiting próprio (por IP e por email) nas rotas públicas sem sessão prévia (cadastro, pedido de agendamento).
  - Nunca expor endpoint algum de dado clínico — só os recursos mínimos do paciente autenticado.
  - Não assumir que o `Pangolin` autenticou o paciente — toda rota autenticada valida a sessão da própria aplicação.
- Isolamento de dados do portal é por **email verificado** do OAuth, nunca por um ID passado em query string/body sem cruzar com a sessão autenticada.

## Segredos e criptografia

- Nenhum segredo (secret/header do admin, client secret OAuth, refresh tokens, chave de cifragem) em log, em resposta de erro, ou em código-fonte. Sempre via `config.yaml` (fora do repo) ou variável de ambiente.
- Refresh tokens do Google e o snapshot de backup do SQLite são sempre cifrados (AES-256-GCM) antes de persistir/enviar ao Drive.
- **Cripto de prontuário por campo em repouso (`SealedText`).** Os campos clínicos sensíveis (corpo da sessão, anamnese, notas, resumo de IA) são cifrados **no domínio**, por campo, via o tipo `SealedText` (AES-256-GCM, pacote `internal/platform/crypto`). O texto selado é persistido como o envelope `versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext` (base64); o `MarshalJSON` emite sempre o cifrado e o claro só sai via `Reveal(ctx)`. O `id_da_chave` no envelope habilita **rotação** de chave; `versao_do_esquema`/`algoritmo` habilitam evolução sem quebrar dados antigos. O PocketBase **não** é a fronteira de cifra — a cifra é do nosso `SealedText`.
- A cifra é **por campo**: campos não sensíveis (nome, email, telefone, datas, status, valores) ficam em **claro e indexáveis**; a busca clínica se dá pelas **tags** da IA (em claro), nunca pelo corpo (cifrado).
- A chave de cifragem, no MVP, vem do config — mas sempre atrás da interface `KeyProvider` (ver `docs/architecture.md`), para permitir troca por vault sem reescrever quem consome a chave.

## Auditoria

- Toda operação sobre dado sensível grava `audit_log`: leitura/escrita de prontuário (anamnese, notas), geração/quitação de débito, alteração de config, backup/restore, autenticação (sucesso e falha), aprovação/rejeição de paciente.
- Quando a IA roda, registrar `ai.process` como operação auditável — **sem** conteúdo clínico (só metadados: ator, paciente, quando, janela), aplicando **minimização**: audita-se o fato do processamento, nunca o texto enviado ou retornado.
- `audit_log` registra o ator (email), a ação, a entidade afetada e o timestamp — nunca o conteúdo clínico em si (não duplicar dados sensíveis no log).

## Drive como fallback/DR

- O Google Drive **não** é fonte da verdade nem está no caminho de request. O backup do SQLite para o Drive é **fallback/DR**, disparado por um **hook no ciclo de backup do PocketBase** (`internal/drivesync`, podendo ser 2º processo); o snapshot sobe sempre cifrado (AES-256-GCM) e compactado.

## Multi-tenant (uma instância por terapeuta)

- O sistema **não** implementa multi-tenancy dentro de uma instância — isolamento entre terapeutas é feito por VM/instância separada, não por `tenant_id` no schema. Não introduzir lógica de tenant compartilhado sem revisitar essa decisão em `docs/requirements.md`.

## Ao adicionar uma rota ou campo novo

Antes de expor qualquer dado novo via API, perguntar:
1. Esse dado é clínico/sensível? Se sim, só pode estar em rota `/v1/admin/*`, nunca `/v1/portal/*`.
2. Essa operação precisa de audit log?
3. Essa rota pública (portal, sem sessão) precisa de rate limiting?
