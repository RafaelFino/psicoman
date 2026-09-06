---
inclusion: always
---

# Psicoman — Processo de entrega

Política transversal a **todo** trabalho neste repositório: como uma mudança é considerada "pronta". Complementa `psicoman-testes-e2e.md` (o quê testar) e `psicoman-golang.md` (como codar). Aqui fica **o processo** de fechamento de trabalho.

## Organização e nomenclatura da documentação

Padrão **único** para todos os arquivos de documentação do projeto. Não criar variações.

- **Raiz do repositório**: apenas `README.md`. É a única exceção de nome em maiúsculas, por convenção universal do ecossistema (GitHub, package managers). Nenhum outro `.md` fica na raiz.
- **Toda a documentação técnica vive em `docs/`.**
- **Nome dos arquivos em `docs/`**: `kebab-case`, tudo minúsculo, com hífen separando palavras e extensão `.md`. Sem maiúsculas, sem `snake_case`, sem espaços.
  - Ex.: `docs/architecture.md`, `docs/requirements.md`, `docs/decisions.md`, `docs/deploy.md`, `docs/release-notes.md`.
- **Documentos canônicos** (papéis fixos):
  - `README.md` — visão geral e uso (raiz).
  - `docs/requirements.md` — o quê/por quê (requisitos de produto).
  - `docs/architecture.md` — como (decisões técnicas, contratos, fluxos).
  - `docs/deploy.md` — instalação e operação.
  - `docs/decisions.md` — registro de decisões (ADR leve).
  - `docs/release-notes.md` — notas de versão visuais (ver abaixo).
- **Specs** ficam em `.kiro/specs/<nome-da-onda>/` com os três arquivos `requirements.md`, `design.md`, `tasks.md` (também minúsculos). O `<nome-da-onda>` em `kebab-case` (ex.: `mvp-audit1`).
- **Steering** fica em `.kiro/steering/` com prefixo `psicoman-` e `kebab-case` (ex.: `psicoman-processo-entrega.md`).
- Ao mover/renomear um documento, **atualizar todos os links** que apontam para ele (Makefile, scripts, README, specs, outros docs) no mesmo commit. Preferir `git mv` para preservar histórico.

## Definição de pronto (Definition of Done)

Nenhuma mudança de comportamento é "concluída" enquanto os três eixos abaixo não estiverem cobertos. Vale para features, correções, ajustes de UI e mudanças de contrato de API.

### 1. Código
- Compila (`make build`) e passa no gate (`make check` quando aplicável: fmt + vet + build + testes).
- Segue as convenções de `psicoman-golang.md` e, para UI, `psicoman-web-responsivo.md`.

### 2. Documentação (sempre atualizada junto com o código)
- Toda funcionalidade nova ou alterada **deve** ser refletida na documentação do projeto no mesmo ciclo — nunca "depois".
- Fontes de verdade a manter coerentes: `docs/requirements.md`, `docs/architecture.md`, `README.md` (uso), `docs/decisions.md` (decisões) e `docs/release-notes.md` (o que mudou por onda).
- Se uma mudança introduz endpoint, tela, entidade, migration ou regra de negócio, os documentos acima devem mencioná-la. Endpoints entram também no Swagger (`internal/api/swagger`).
- Uma task de spec só fecha se a documentação correspondente foi atualizada.

### 3. Testes (cobrem a mudança)
- Regra de negócio nova → teste unitário no `service` (fakes de repositório/integração), conforme `psicoman-testes-e2e.md`.
- Fluxo de negócio novo/alterado → cobertura E2E em `test/e2e/` (request HTTP real, Google mockado).
- Mudança de contrato de API (novo endpoint, novo campo, novo status de erro) → E2E que exercita o novo contrato.
- Mudança de UI → no mínimo smoke de parse dos templates e verificação manual documentada na task; regras de UI com lógica (ex: máscara de dinheiro, render markdown) ganham teste onde couber.
- Toda mudança de segurança/acesso (ex: gate de aprovação) → E2E cobrindo tanto o caminho permitido (2xx) quanto o negado (403/401).
- Nenhum `t.Skip` sem justificativa no próprio teste.

## Release notes

Documentar de forma **visual e legível** (para humanos, não só changelog técnico) o que cada onda entrega.

- Arquivo único: `docs/release-notes.md`, mais recente no topo.
- Uma entrada por onda de implementação (ou versão), com: título, data, resumo em uma frase, e listas por tema — **Novidades**, **Melhorias**, **Correções**, **Notas de migração/operação** quando houver.
- Linguagem orientada ao usuário/terapeuta (o que ele passa a conseguir fazer), não a implementação interna. Detalhe técnico profundo vai para `docs/decisions.md`.
- Atualizar no fechamento da onda (junto com a task de fechamento final).

## Fluxo de Git

Objetivo: histórico legível, com commits por marco lógico, e integração remota controlada.

- **Uma branch por onda de specs** (conjunto grande de trabalho), não uma branch por task. Nome em `kebab-case` referenciando a onda (ex.: `mvp-audit1`). Trabalhar processos longos nessa branch dedicada, nunca direto na `main`.
- **Commit ao fim de cada grupo de tasks** (por fase da spec, ou por task grande e autônoma). Cada commit deixa a base consistente: compila e testes verdes.
- **Push apenas no final da onda de implementação** (quando o conjunto planejado estiver concluído e verde). Não fazer push a cada commit.
- Antes de cada commit: `make check` (ou, no mínimo, build + testes relevantes) verde.
- Mensagens de commit em PT-BR, no imperativo, descrevendo o marco (ex.: "adiciona gate de aprovação de paciente no portal"). Referenciar a fase/onda quando fizer sentido (ex.: "mvp-audit1 Fase A").
- Não commitar segredos, `config.dev.yaml` com chaves reais, nem artefatos temporários de teste. Preferir `git add` de arquivos específicos a `git add -A`.
- Não reescrever histórico já publicado; `--force`/`reset --hard`/`clean -fd` só com pedido explícito.
- Preservar hooks (não usar `--no-verify` sem necessidade explícita).

## Registro de decisões (lastro)

Decisões relevantes de produto ou arquitetura tomadas durante a implementação **devem** ser registradas para termos rastreabilidade no futuro.

- Arquivo: `docs/decisions.md` (formato leve de ADR: data, contexto, decisão, consequências).
- O que registrar: mudança de escopo, novo requisito derivado de auditoria, decisão de contrato de API, escolha entre alternativas técnicas, política de segurança (ex.: gate de aprovação).
- O que **não** poluir com: detalhes triviais de implementação já óbvios no código.
- Cada spec (`.kiro/specs/*`) que nasce de uma auditoria ou decisão deve ter uma entrada correspondente em `docs/decisions.md` apontando para ela.

## Checklist de fechamento de uma fase de spec

Ao concluir um grupo de tasks:
1. Código compila e testes (unit + E2E afetados) verdes.
2. `docs/requirements.md` / `docs/architecture.md` / `README.md` atualizados conforme a mudança.
3. `docs/decisions.md` atualizado se houve decisão relevante.
4. Swagger atualizado se o contrato de API mudou.
5. Commit em PT-BR referenciando a fase (na branch da onda).
6. `docs/release-notes.md` e push somente quando a onda inteira estiver concluída e verde.
