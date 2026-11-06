# Review do requirements.md da spec `auditoria-profunda`

Gate de qualidade/conformidade do entregável `.kiro/specs/auditoria-profunda/requirements.md`, julgado contra o relatório de auditoria `.agents/tasks/auditoria-profunda/findings.md` e o padrão de formato de `.kiro/specs/pocketbase-finish/requirements.md`. Não reexecutei a auditoria nem `make check`/`make lint` — o passo de auditoria já rodou e o resultado literal está no findings. Fiz um único spot-check de código (`internal/api/httpx/auth.go`) para confirmar o estado do modo dev de identidade do Pangolin. O documento revisado traça cada requisito a um achado real, mantém o formato da referência e cobre os três achados conhecidos desta sessão.

**Watch for:** nada bloqueante. Pontos menores não-bloqueantes: a severidade de R6 é rotulada "média/baixa" no cabeçalho do grupo enquanto DOC-3 é baixa (fiel ao findings, não é erro); R2.3 cita um caminho (`docs/decisions.md` + steering) que também aparece em R6.2 — redundância aceitável de rastreabilidade (possible).

**Verdict**: APPROVED

## High-level view

O formato segue exatamente a referência `pocketbase-finish`: título com sufixo `— Requisitos`, parágrafo de contexto datado 2026-11-05, blockquote o quê/por quê + como-no-design, grupos `## RN. Título`, sub-itens `**RN.x**` em estilo EARS (QUANDO…DEVE / NÃO DEVE), bloco `**Aceite.**` por grupo e `**Decisão necessária (Rafael):**` onde há escolha em aberto. A estrutura é fiel, inclusive no detalhe de herdar a "Definição de pronto" como grupo próprio (R9) e trazer um recorte em sub-ondas, espelhando a decisão-necessária de priorização da referência.

A rastreabilidade é sólida: todo requisito aponta um ID de achado real (SEG-1/2, DRV-1/2, QG-1, TST-1/2, DOC-1/2/3, SEG-3/4/5, QG-2) e nenhum requisito é inventado. Os achados marcados como resolvidos no findings (FEAT-1, OAUTH-1, OBS-1, ARQ-1) não são reabertos como requisitos — ficam explicitamente em R8 como registro de rastreabilidade do que já está fechado, exatamente o comportamento pedido.

A cobertura dos três achados conhecidos desta sessão está correta. O gap de criação automática da pasta de DR no Drive vira R3 (DRV-2). O backup não-incremental vira R2 (DRV-1). O modo dev de identidade do Pangolin (`pangolin.dev_identity`) é o ponto que o findings classifica como **verificado e OK** (fail-closed quando vazio, header real sempre vence, coerente com o ADR 2026-11-05) — confirmei isso lendo `auth.go` — então o requirements corretamente **não** o levanta como requisito aberto. Levantá-lo seria o erro; omiti-lo é o acerto.

O resultado literal de `make check` (verde, exit=0) e `make lint` (golangci-lint pulado por incompatibilidade de toolchain) está registrado no parágrafo de contexto e referenciado ao findings, e o lint desligado é promovido a requisito próprio (R4). As referências cruzadas em kebab-case e caminhos relativos resolvem corretamente a partir do diretório da spec.

<details>
<summary>Issues (0 bloqueantes)</summary>

Nenhum achado bloqueante. Observações não-acionáveis (não exigem mudança):

1. **Redundância R2.3 × R6.2** — ambos tocam `docs/decisions.md`/steering para o caso "snapshot inteiro"; é reforço de rastreabilidade, não conflito (possible, não-bloqueante).
2. **Rótulo de severidade de grupo R6** — "média/baixa" no cabeçalho porque agrupa DOC-1/DOC-2 (média) e DOC-3 (baixa); fiel ao findings (confirmed, não-bloqueante).

</details>

<details>
<summary>Details</summary>

## Conformidade de formato com a referência

Verificação item a item contra os elementos da referência `pocketbase-finish/requirements.md`: título `# auditoria-profunda — Requisitos` com o sufixo correto; parágrafo de contexto com a data **2026-11-05**; blockquote o quê/por quê + como-no-design apontando `design.md`/`tasks.md`; linha de princípios herdados; sub-itens `**Rn.x**` em EARS consistente ("QUANDO … O sistema DEVE …", "O sistema NÃO DEVE …") com `**Aceite.**` por grupo. `**Decisão necessária (Rafael):**` aparece exatamente onde há escolha real — R2 (incremental vs. snapshot), R3 (criação automática vs. documentar), R7 (rate limit próprio vs. nginx; state vs. Pangolin) e o recorte em sub-ondas — não como ruído. R9 herda a Definition of Done, espelhando o R7 da referência.

## Rastreabilidade ao findings

Cada grupo cita o(s) ID(s) do findings no próprio título e no `**Contexto.**`:

```
R1 → SEG-1, SEG-2 (auditoria LGPD no caminho clínico)
R2 → DRV-1          (backup incremental)
R3 → DRV-2          (pasta de DR)
R4 → QG-1, TST-1    (golangci-lint desligado)
R5 → TST-2          (cobertura de auditoria clínica)
R6 → DOC-1/2/3      (docs/steering)
R7 → SEG-3/4/5, QG-2 (segurança de severidade menor)
R8 → FEAT-1, OAUTH-1, OBS-1, ARQ-1 (resolvidos — NÃO reabertos)
```

Confirmei a correspondência contra o findings: cada requisito deriva de um achado com evidência (arquivo:linha ou saída de comando) no relatório. Os pontos que o findings classifica como "verificados e OK" (gate de identidade do Pangolin, gate de aprovação do portal, não-vazamento clínico, cifra na borda, regra de dependência, Swagger↔rotas, migrations) não são transformados em requisitos — correto. O requirements não inventa nada que não esteja no findings.

O tratamento dos resolvidos é o ponto fino e está certo: FEAT-1 (gaps R1 da `pocketbase-finish`), OAUTH-1 (OAuth 3-legged + token cifrado), OBS-1 (health/livez/readyz) e ARQ-1 (regra de dependência) vão para R8 como "o que NÃO entra nesta onda", em vez de reaparecerem como lacunas abertas. Isso satisfaz o critério de "gaps da pocketbase-finish já resolvidos não aparecem como abertos".

## Cobertura dos achados conhecidos da sessão

Os três itens que o enunciado pede verificar SE confirmados no findings:

**Gap de criação de pasta do Drive** — confirmado no findings (DRV-2, evidência `wiring.go:270-282`, `drive.go` sem `CreateFolder`, `config.dev.yaml` com folder id preenchido à mão) e refletido na experiência da sessão do Rafael ("criei a pasta no drive, realmente não estava lá"). Coberto por **R3**, com R3.1 (criar via `files.create`), R3.2 (alternativa: documentar o pré-requisito) e R3.3 (degradar com aviso em vez de desativar silenciosamente). Cobertura completa.

**Backup incremental possivelmente ausente** — confirmado no findings (DRV-1, evidência `drivesync.go:68-110` sobe o blob inteiro, grep por manifesto/hash ausente). Coberto por **R2**, com R2.1/R2.2 (implementar incremental por manifesto) e R2.3 (alternativa: oficializar snapshot inteiro + ADR + steering). Cobertura completa, com a decisão em aberto corretamente marcada.

**Estado do modo dev de identidade `pangolin.dev_identity`** — este é o "modo debug" da conversa original do Rafael (subir a API sem a validação do Pangolin para testar). O findings o classifica como **verificado e OK**: `auth.go:49-66` fail-closed sem header e `devIdentity==""` → 401, default vazio em `config.dev.yaml`, WARN no `Register` quando ativo, coerente com o ADR 2026-11-05. Fiz o spot-check direto em `internal/api/httpx/auth.go` e confirmo: o bypass existe, o header real sempre vence quando presente, e com `devIdentity==""` o comportamento é idêntico ao de produção. Portanto o modo dev **não** é um achado aberto — está presente e correto. O requirements acertou ao **não** levantá-lo como requisito; incluí-lo contrariaria a rastreabilidade (seria requisito sem achado). Critério satisfeito pela ausência correta.

## Registro do resultado de make check / make lint

O parágrafo de contexto registra literalmente: `make check` **verde** (exit=0 — gofmt + go vet + build + unit + e2e) e `make lint` **pulando o golangci-lint** (v1.64.8 incompatível com o toolchain Go 1.27), apontando a seção literal do findings. Além do registro, a dívida vira requisito próprio (R4: atualizar o golangci-lint e reativá-lo no gate). O critério pede que o resultado real esteja "registrado (no requirements ou referenciado do findings)" — está nos dois.

## Nomenclatura e referências cruzadas

As âncoras de severidade (alta/média/baixa) nos títulos batem com a tabela-resumo do findings. Os caminhos relativos resolvem a partir de `.kiro/specs/auditoria-profunda/`:

```
../../../.agents/tasks/auditoria-profunda/findings.md   → raiz/.agents/...        OK
../../steering/psicoman-seguranca-lgpd.md               → .kiro/steering/...      OK
../../../docs/requirements.md | architecture.md | ...    → raiz/docs/...           OK
../pocketbase-finish/ | ../pocketbase-finish/tasks.md    → .kiro/specs/...         OK
./design.md | ./tasks.md                                 → mesmo diretório         OK
```

Todos em kebab-case, extensão `.md`, coerentes com a política de `psicoman-processo-entrega` (specs em `.kiro/specs/<onda>/` com `requirements.md`/`design.md`/`tasks.md` minúsculos; steering com prefixo `psicoman-`). Nenhum link quebrado detectado.

</details>

## File map

<details>
<summary>Arquivos consultados</summary>

- `.kiro/specs/auditoria-profunda/requirements.md` — entregável sob review.
- `.agents/tasks/auditoria-profunda/findings.md` — relatório de auditoria (fonte de verdade dos achados; resultado literal de make check/lint).
- `.kiro/specs/pocketbase-finish/requirements.md` — padrão de formato de referência.
- `internal/api/httpx/auth.go` — spot-check do modo dev `pangolin.dev_identity` (confirma estado OK; não é achado aberto).

</details>
