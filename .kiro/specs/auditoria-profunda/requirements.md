# auditoria-profunda — Requisitos

Onda de **correção de achados da auditoria profunda** do Psicoman (2026-11-05). A auditoria foi uma varredura read-only do repositório contra as fontes de verdade (`docs/`, `.kiro/specs/`, `.kiro/steering/`) e o código (`internal/`, `cmd/`, `test/`), consolidada em [Anexo A](#anexo-a--achados-e-evidências). Nenhum código foi alterado, o servidor não foi executado e a API real do Google não foi tocada durante a auditoria. Esta onda resolve o que a auditoria apontou como aberto — gaps de feature do backup ao Drive, lacunas de auditoria LGPD no caminho clínico, dívidas de qualidade (lint desligado), inconsistências doc/steering e as métricas de observabilidade antes adiadas (promovidas a requisito a pedido do Rafael) — deixando de fora o que já está **resolvido** (gaps R1 da [`../pocketbase-finish/`](../pocketbase-finish/), OAuth 3-legged, health/livez/readyz).

> Responde **o quê**/**por quê**. O **como** fica em [`design.md`](./design.md) (a criar na sessão de execução); o plano em [`tasks.md`](./tasks.md). Fonte de cada requisito: um achado real do [Anexo A](#anexo-a--achados-e-evidências) (ID rastreado em cada grupo). Nenhum requisito aqui é inventado — gap já fechado não é reaberto.

Princípios herdados (não reabrir): binário único; regra de dependência (`domain`/`app`/`config` não importam o PocketBase); cifra por campo `SealedText` na borda + DTO/`Reveal()`; contrato `/v1` por ator com envelope PT-BR; segredos nunca em log/repo; ULID, centavos `int64`, `America/Sao_Paulo`. Auditoria de dado sensível conforme [`../../steering/psicoman-seguranca-lgpd.md`](../../steering/psicoman-seguranca-lgpd.md) e [`../../../docs/requirements.md`](../../../docs/requirements.md).

Resultado literal da auditoria para o gate: `make check` **verde** (exit=0 — gofmt + go vet + build + unit + e2e), porém `make lint` **pula o golangci-lint** (incompatível com o toolchain Go 1.27 × golangci-lint v1.64.8) — análise estática efetivamente desligada. Detalhe em [Anexo A](#anexo-a--achados-e-evidências) (seção "Resultado literal de `make check` e `make lint`") e nos requisitos R4 abaixo.

---

## R1. Auditoria LGPD no caminho clínico primário (SEG-1, SEG-2 — **alta**)

**Contexto.** O steering [`psicoman-seguranca-lgpd.md`](../../steering/psicoman-seguranca-lgpd.md) e [`../../../docs/requirements.md`](../../../docs/requirements.md) (§4.1) exigem que **toda** operação sobre dado sensível grave `audit_log` — leitura/escrita de prontuário (anamnese, notas). A auditoria encontrou que o caminho clínico primário **não** cumpre isso: `HistoryService`/`DayFeedService` auditam `AuditActionRead`, mas a leitura do chart e as escritas de nota/anamnese não. O `ProntuarioService` sequer recebe uma dependência de auditoria no wiring (`app.NewProntuarioService(prontuarioRepo, nil)`).

- **R1.1** QUANDO um terapeuta lê o prontuário via `GET /v1/app/patients/{id}` (chart com notas/anamnese reveladas em claro), O sistema DEVE gravar `AuditActionRead` em `patient`/`prontuario` com metadata **sem** conteúdo clínico (ator, `patient_id`, contagem). (SEG-1)
- **R1.2** QUANDO um terapeuta cria uma nota (`AddNote`) ou grava/atualiza a anamnese (`UpsertAnamnesis`), O sistema DEVE gravar `AuditActionCreate`/`AuditActionUpdate` com metadata **sem** conteúdo clínico. (SEG-2)
- **R1.3** O sistema DEVE injetar a dependência de auditoria no `ProntuarioService` (ou auditar no handler), substituindo o `nil` atual no wiring, sem violar a regra de dependência.
- **R1.4** O sistema NÃO DEVE registrar conteúdo clínico (texto de nota/anamnese) na metadata do `audit_log` em nenhum desses caminhos.

**Aceite.** Leitura e escrita de prontuário geram `audit_log`; a metadata nunca contém clínico; cobertura por teste unit no `service` **e** E2E de contrato (ver R5 — fecha também TST-2). A trilha de auditoria do chart passa a existir, fechando a lacuna de SEG-1/SEG-2.

---

## R2. Backup incremental ao Drive por manifesto/hash (DRV-1 — **alta**)

**Contexto.** O steering [`psicoman-google-api.md`](../../steering/psicoman-google-api.md) (seção Drive) pede backup incremental por hash, com manifesto no Drive, nunca reenviando o acervo inteiro a cada execução. A auditoria encontrou em `internal/drivesync/drivesync.go` (`mirror`/`uploadSealed`) que o backup lê o snapshot inteiro, cifra e sobe o blob completo (`<backup>.enc`) a cada execução — sem hash, manifesto, diff ou dedup. Steering e código divergem.

- **R2.1** QUANDO o backup ao Drive roda, O sistema DEVE enviar apenas o delta (itens novos/alterados) com base em um manifesto de hashes mantido no Drive, nunca o acervo inteiro — conforme o steering.
- **R2.2** O sistema DEVE manter um manifesto cifrado no Drive com hash por item, consultado para decidir o que subir.
- **R2.3** (Alternativa, se a decisão for manter snapshot inteiro) O sistema DEVE ter a decisão consciente registrada em [`../../../docs/decisions.md`](../../../docs/decisions.md) e o steering [`psicoman-google-api.md`](../../steering/psicoman-google-api.md) atualizado para refletir "snapshot inteiro", eliminando a divergência.

**Aceite.** Ou o backup passa a ser incremental por manifesto (R2.1/R2.2) com E2E cobrindo o reenvio só do delta, **ou** a decisão de snapshot inteiro está registrada e o steering alinhado (R2.3). Em qualquer caso, steering e código deixam de divergir.

**Decisão necessária (Rafael):** implementar o incremental (custo maior, cumpre o steering como escrito) **ou** oficializar o snapshot inteiro (mais simples, exige revisar o steering + ADR). Recomendação em `design.md`.

---

## R3. Criação automática da pasta de DR no Drive (DRV-2 — média)

**Contexto.** O DR real só ativa com `ClientID`/`ClientSecret`/`DriveFolder` preenchidos (`internal/api/wiring.go`); `drive_folder` vazio → `drivesync.New(nil, …)` = no-op silencioso. Não existe `files.create` de pasta em `internal/integration/google/drive.go` (só `Upload` de arquivo), então o operador precisa criar a pasta manualmente e colar o id em `google.drive_folder`. Isso confirma o comportamento observado na sessão do Rafael (a pasta de DR não foi criada sozinha; foi preciso criá-la à mão).

- **R3.1** QUANDO o primeiro backup roda e a pasta de DR ainda não existe, O sistema DEVE criá-la via `files.create` (mime `application/vnd.google-apps.folder`; o escopo `drive.file` permite) e persistir o folder id. (DRV-2 opção a)
- **R3.2** (Alternativa) O sistema DEVE documentar em [`../../../docs/deploy.md`](../../../docs/deploy.md) que o operador cria a pasta e informa o id, caso a criação automática fique fora de escopo. (DRV-2 opção b — ver R6)
- **R3.3** QUANDO o DR está configurado mas só falta a pasta, O sistema DEVE degradar com aviso observável em vez de desativar o backup silenciosamente.

**Aceite.** Ou o primeiro backup cria e persiste a pasta de DR (R3.1), ou o pré-requisito está documentado no deploy (R3.2); em ambos os casos a situação "só falta a pasta" vira degradação observável (R3.3), nunca silêncio.

**Decisão necessária (Rafael):** criação automática da pasta (R3.1) **ou** pré-requisito documentado (R3.2).

---

## R4. Qualidade: reativar análise estática (golangci-lint) (QG-1, TST-1 — média)

**Contexto.** `make lint`/`make check` logam o skip do golangci-lint (v1.64.8 incompatível com o toolchain Go 1.27 — "não decodifica o export data"). O `Makefile` pula a varredura deliberadamente para não travar o gate, mas o efeito é que **nenhum linter além do `go vet` roda**: ineffassign, staticcheck, errcheck e os demais configurados em `.golangci.yml` estão desligados na prática.

- **R4.1** O sistema DEVE atualizar o golangci-lint para uma versão compatível com o toolchain Go 1.27 e revalidar `.golangci.yml`.
- **R4.2** QUANDO o gate roda (`make lint`/`make check`), O golangci-lint DEVE executar de fato (não apenas em modo advisory/skip), fazendo a análise estática voltar a ser parte do gate.

**Aceite.** `make lint` roda o golangci-lint sem skip; `make check` continua verde com a análise estática ativa; `.golangci.yml` revalidado. Enquanto R4.1 não fecha, o projeto opera sem análise estática de bugs (ver Anexo A.1).

---

## R5. Testes de cobertura da auditoria clínica (TST-2 — baixa)

**Contexto.** Há testes de auditoria para history/dayfeed (`internal/app/feed_service_test.go`), mas nenhum para `GetPatientChart`/`AddNote`/`UpsertAnamnesis` — porque a auditoria nesses caminhos ainda não existe (consequência de R1/SEG-1/SEG-2). Os E2E de não-vazamento cobrem o clínico não vazar, não a trilha de auditoria da leitura.

- **R5.1** QUANDO R1 for implementado, O sistema DEVE ganhar teste unit no `service` cobrindo que leitura/escrita de prontuário gravam `audit_log` (fakes de repositório), conforme [`../../steering/psicoman-testes-e2e.md`](../../steering/psicoman-testes-e2e.md).
- **R5.2** O sistema DEVE ter E2E de contrato exercitando a leitura/escrita de prontuário e verificando a trilha de auditoria resultante (sem clínico na metadata).

**Aceite.** Os testes de R5 acompanham a correção de R1 no mesmo ciclo (DoD de [`../../steering/psicoman-processo-entrega.md`](../../steering/psicoman-processo-entrega.md)); nenhum `t.Skip` sem justificativa.

---

## R6. Docs e steering coerentes com o código (DOC-1, DOC-2, DOC-3 — média/baixa)

**Contexto.** A auditoria encontrou três divergências doc↔código, todas sem impacto em runtime mas que induzem a erro:

- **R6.1 (DOC-1, média)** O steering [`psicoman-google-api.md`](../../steering/psicoman-google-api.md) cita `https://www.googleapis.com/auth/gmail.send` e tem uma seção Gmail inteira, mas Gmail está **fora de escopo** ([`../../../docs/requirements.md`](../../../docs/requirements.md) §3.7/§5), não há código Gmail e `AuthorizeScopes` pede só `calendar`+`drive.file`. O sistema DEVE atualizar o steering removendo `gmail.send`/seção Gmail (ou marcá-la explicitamente fora de escopo), para não induzir pedido de escopo a mais. A implementação está correta; só o steering está desatualizado.
- **R6.2 (DOC-2, média)** QUANDO DRV-1/DRV-2 (R2/R3) forem resolvidos, O sistema DEVE refletir em [`../../../docs/architecture.md`](../../../docs/architecture.md) (§0.10) e [`../../../docs/deploy.md`](../../../docs/deploy.md) o comportamento real do Drive DR: pré-requisito da pasta (ou criação automática) e se o backup é incremental ou snapshot inteiro.
- **R6.3 (DOC-3, baixa)** O sistema DEVE consolidar a tabela "Status ao vivo" da [`../pocketbase-finish/tasks.md`](../pocketbase-finish/tasks.md), que hoje tem a **Fase 10 duplicada** (uma `⬜ pendente`, outra `✅ concluída`), em uma linha coerente com o estado real (onda concluída e mergeada — PR #4), marcando os checkboxes da Fase 10.

**Aceite.** Steering do Google sem Gmail; docs de arquitetura/deploy descrevendo o Drive DR real; tabela da Fase 10 consolidada. Cada doc atualizado no mesmo ciclo da mudança que o motiva (R6.2 acompanha R2/R3).

---

## R7. Endurecimento de segurança de severidade menor (SEG-3, SEG-4, SEG-5, QG-2 — média/baixa)

**Contexto.** Achados de segurança de prioridade menor, agrupados por serem decisões pontuais. Nenhum é bloqueante no modelo single-tenant atrás do Pangolin, mas cada um tem uma lacuna documentada.

- **R7.1 (SEG-3, média)** O steering [`psicoman-seguranca-lgpd.md`](../../steering/psicoman-seguranca-lgpd.md) pede rate limiting **próprio** por IP **e por email** nas rotas públicas (cadastro, pedido de agendamento). Hoje só o nginx de borda limita (por IP, 30 r/min) — `register` por email repetido não é limitado em lugar nenhum. O sistema DEVE **(a)** adicionar rate limit por email no app para `POST /v1/portal/register` e `appointment-requests`, **ou (b)** registrar em [`../../../docs/decisions.md`](../../../docs/decisions.md) a decisão de que o nginx cobre o risco e atualizar o steering. Hoje o steering está violado sem registro.
- **R7.2 (SEG-4, baixa)** O `state` do OAuth é gerado (`ulid.New()`) mas **não validado** no `GoogleCallback` (sem armazenamento entre authorize e callback). O sistema DEVE persistir o `state` (sessão/coleção efêmera) e validá-lo no callback, **ou** documentar que o Pangolin na borda cobre o CSRF e o state é cosmético.
- **R7.3 (SEG-5, baixa — operacional)** `config.dev.yaml` contém segredos reais em claro no disco (gitignored, não rastreado — OK). O sistema NÃO exige mudança de código; a recomendação é **rotacionar** o client_secret/refresh token do Google e a api_key do Gemini ao encerrar a fase de testes, pois foram expostos no ambiente de dev.
- **R7.4 (QG-2, baixa)** `internal/api/app/sessions.go:127` tem TODO de fase antiga: o `actor` do audit de sessão é hard-coded ("terapeuta") em vez de derivar de `httpx.ActorIdentity` (identidade do Pangolin, hoje gravada sem consumidor). O sistema DEVE amarrar o `actor` do `audit_log` à identidade real e remover o TODO. Impacto baixo em single-tenant, mas a trilha fica genérica.

**Aceite.** Rate limit por email implementado **ou** decisão registrada (R7.1); `state` validado **ou** decisão documentada (R7.2); recomendação de rotação de segredos registrada (R7.3); `actor` do audit derivado da identidade real e TODO removido (R7.4).

**Decisão necessária (Rafael):** R7.1 (rate limit próprio vs. aceitar o nginx) e R7.2 (validar state vs. confiar no Pangolin) são escolhas de produto/segurança. Recomendação em `design.md`.

---

## R8. Métricas ricas de observabilidade (OBS-1, parte adiada — média)

**Contexto.** Os probes `GET /v1/health`, `/v1/livez` e `/v1/readyz` já existem e estão cobertos (ver R9). O que a [`../pocketbase-finish/`](../pocketbase-finish/) (R5.2) deixou **adiado por decisão** ([`../../../docs/decisions.md`](../../../docs/decisions.md), 2026-10-24) são as **métricas**: hoje só existe o timing por rota **logado** (`httpx.Timing`/`httpx.Logging`), sem endpoint consultável nem série temporal. Esta onda promove as métricas de "adiado" a requisito aberto, com escopo **rico**: a operação deve conseguir enxergar tráfego, latência, erros, negócio, integrações Google/Gemini, backup, cifra e saúde do processo.

A coleta (scrape) acontece em **ambiente fechado**, sem exposição à web. Por isso o endpoint de métricas **não precisa** passar pelo gate de identidade do Pangolin, no mesmo espírito dos probes de saúde (fora da auth). A borda pública já encaminha só o portal e a saúde e responde 404 no resto (ver [`../../../docs/stack-local.md`](../../../docs/stack-local.md) e `deploy/nginx/portal.conf`), então o endpoint nasce fora do alcance da internet. Esse isolamento vira requisito verificável (R8.9).

- **R8.1 Endpoint.** O sistema DEVE expor um endpoint de métricas em formato texto Prometheus (ex.: `GET /v1/metrics`), fora do gate do Pangolin, respondendo sem o envelope JSON padrão (o formato de exposição é o do Prometheus).
- **R8.2 HTTP.** O sistema DEVE medir, por **ator** (`app`/`portal`/`admin`/`infra`), **rota** (padrão da rota, nunca o path bruto), **método** e **status**: total de requisições, histograma de latência, requisições em andamento (gauge) e tamanho da resposta.
- **R8.3 Segurança e acesso.** O sistema DEVE contar respostas `401`/`403` por ator e motivo (sem header do Pangolin, sem sessão, paciente não aprovado, gate de aprovação do portal), e expor um gauge que indique se o **modo dev de identidade** (`pangolin.dev_identity`) está ativo (`1`/`0`), para alerta de configuração insegura em produção.
- **R8.4 Negócio.** O sistema DEVE contar eventos de domínio relevantes: pacientes cadastrados/aprovados/recusados, sessões criadas/confirmadas/canceladas/finalizadas/faltas, pedidos de agendamento criados/confirmados/recusados, débitos gerados, pagamentos registrados e PDFs de cobrança gerados, além de gauges de fila pendente (pacientes e pedidos aguardando o terapeuta).
- **R8.5 Google Calendar/Meet.** O sistema DEVE medir chamadas ao Calendar (freebusy e criação de evento): total por resultado (`ok`/`erro`/`conflito`), histograma de latência e contagem de agendamentos que degradaram sem evento por falta de credencial.
- **R8.6 Drive DR e backup.** O sistema DEVE medir o ciclo de backup: total por resultado, duração, tamanho do snapshot, timestamp do último backup **bem-sucedido** (gauge, base para alerta de "backup velho"), resultado do espelhamento no Drive (`ok`/`erro`/`desativado`) e, quando R2 for implementado, bytes e itens enviados versus ignorados (delta).
- **R8.7 OAuth Google, Gemini e auditoria.** O sistema DEVE expor o estado da autorização Google como gauge (`ok`/`degradado`, o mesmo critério do `/readyz`), medir chamadas ao Gemini (total por resultado, latência, IA ligada/desligada) e contar gravações em `audit_log` por ação (leitura/criação/atualização), o que também sustenta a verificação de R1.
- **R8.8 Cifra e runtime.** O sistema DEVE contar operações de cifra (selar/abrir, por resultado, com erros destacados para detectar chave ausente ou rotação mal feita) e expor métricas de runtime Go e de processo (goroutines, memória, GC, descritores abertos, uptime, versão/build como `info`).
- **R8.9 Restrições de LGPD, cardinalidade e isolamento.**
  - **R8.9.1** Nenhum rótulo, nome de métrica ou valor DEVE conter dado pessoal ou clínico: sem `patient_id`, e-mail, telefone, CPF, nome, conteúdo de nota ou anamnese. Rótulos só usam enums fechados (ator, rota, método, status, resultado, ação).
  - **R8.9.2** A rota DEVE ser o **padrão registrado** (ex.: `/v1/app/patients/{id}`), nunca o path com ULID, para manter a cardinalidade limitada. Requisições a rotas inexistentes DEVEM cair em um rótulo único (ex.: `nao_encontrada`).
  - **R8.9.3** O endpoint de métricas NÃO DEVE ser encaminhado pela borda pública: o nginx do portal continua respondendo 404 para ele, e isso DEVE ter teste ou verificação documentada.
  - **R8.9.4** Segredos (chaves de cifra, client secret, refresh token, api key do Gemini) NUNCA DEVEM aparecer em métricas, rótulos ou mensagens de erro expostas.
- **R8.10 Custo e falha.** A instrumentação NÃO DEVE bloquear nem alterar o resultado de uma requisição: falha ao registrar métrica é engolida e logada em nível baixo, nunca vira `500`. O overhead por requisição DEVE ser desprezível frente ao I/O do SQLite.
- **R8.11 Dependência.** A escolha entre a biblioteca cliente oficial do Prometheus e uma exposição mínima própria é decisão de design. Em qualquer caso a versão DEVE ser fixada, e a regra de dependência se mantém: `domain`, `app` e `config` não importam o PocketBase nem a biblioteca de métricas. A instrumentação de negócio entra por uma porta (interface) injetada, com implementação no-op para os testes.

**Aceite.** `GET /v1/metrics` responde em formato Prometheus com as famílias de R8.2 a R8.8. Teste E2E exercita requisições reais (sucesso, `401`, `403`, `404`) e confere que os contadores e os rótulos esperados aparecem. Teste de "vazamento" confere que nenhum identificador pessoal, clínico ou segredo aparece na saída. Teste confere que a rota usa o padrão e não o path bruto. A instrumentação de negócio é coberta por unit no `service` com a porta no-op/fake. Docs atualizadas no mesmo ciclo: [`../../../docs/architecture.md`](../../../docs/architecture.md), [`../../../docs/deploy.md`](../../../docs/deploy.md) (como coletar, nomes das métricas e alertas sugeridos), [`../../../docs/decisions.md`](../../../docs/decisions.md) (reverte o adiamento de 2026-10-24) e Swagger, se a rota for documentada.

**Decisão necessária (Rafael):** (a) biblioteca cliente oficial do Prometheus (`client_golang`) **ou** exposição mínima própria; (b) o endpoint ficar na mesma porta (`/v1/metrics`) **ou** num listener separado só para scrape, em porta interna. O listener separado isola ainda mais, mas custa mais complexidade. Recomendação em `design.md`.

---

## R9. O que NÃO entra nesta onda (resolvido/adiado — rastreabilidade)

Registrado para evitar reabrir trabalho já fechado:

- **FEAT-1 — resolvido.** Os gaps R1 da [`../pocketbase-finish/`](../pocketbase-finish/) (DayFeed, History, Metrics, Cost, locais, origens, planos, `appointment_request`) estão **expostos no `/v1`** (`internal/api/app/mount.go`) e cobertos; o contrato Swagger↔rotas é validado por teste (`internal/api/swagger/contract_test.go`). Não há rota órfã.
- **OAUTH-1 — resolvido.** O OAuth 3-legged real + `google_token` cifrado (R2 da pocketbase-finish) está implementado.
- **OBS-1 — probes resolvidos; métricas promovidas a [R8](#r8-métricas-ricas-de-observabilidade-obs-1-parte-adiada--média).** `/health`+`/livez`+`/readyz` presentes (`internal/api/api.go`); `readyz` reflete cifra/DB/estado do Google, e isso NÃO é reaberto. As métricas Prometheus estavam **adiadas por decisão** ([`../../../docs/decisions.md`](../../../docs/decisions.md) 2026-10-24) e agora entram nesta onda como R8, a pedido do Rafael. Isso fecha o R5.2 da pocketbase-finish.
- **ARQ-1 — OK.** Regra de dependência testada e verde (`internal/archtest/arch_test.go`); padrões de domínio (ULID, centavos `int64`, timezone, envelope PT-BR) respeitados.

---

## R10. Definição de pronto (herdada de `psicoman-processo-entrega`)

Toda mudança de comportamento fecha com: código verde (`make check`), docs atualizadas no mesmo ciclo (requirements/architecture/README/Swagger/decisions conforme o caso), e testes cobrindo (regra nova → unit; fluxo/contrato → E2E; segurança/acesso → E2E permitido **e** negado; cripto → round-trip + não-vazamento). Trabalho em branch da onda; commit por fase; push/PR só no fim e com OK do Rafael. Ver [`../../steering/psicoman-processo-entrega.md`](../../steering/psicoman-processo-entrega.md), [`../../steering/psicoman-golang.md`](../../steering/psicoman-golang.md) e [`../../steering/psicoman-testes-e2e.md`](../../steering/psicoman-testes-e2e.md).

---

## Recorte sugerido em sub-ondas

A onda é média-grande; candidato a quebrar por severidade (prioridade do Anexo A):

- **Sub-onda A (alta):** R1 (auditoria clínica) + R5 (testes de R1) + R2 (backup incremental). O coração de LGPD e o gap de backup.
- **Sub-onda B (média):** R3 (pasta de DR) + R4 (golangci-lint) + R6 (docs/steering) + R7.1 (rate limit) + R8 (métricas ricas). R8 depende parcialmente de R1 (contador de `audit_log`, R8.7) e de R2 (delta do backup, R8.6); as métricas desses pontos entram quando os requisitos de origem forem implementados, sem bloquear o restante de R8.
- **Sub-onda C (baixa):** R7.2 (state OAuth) + R7.3 (rotação de segredos) + R7.4 (actor do audit) + R6.3 (tabela Fase 10).

**Decisão necessária (Rafael):** confirmar o recorte em sub-ondas e, dentro de R2/R3/R7.1/R7.2/R8, as escolhas em aberto marcadas acima. Detalhamento e recomendações em [`design.md`](./design.md).

---

## Anexo A — Achados e evidências

Consolida o relatório da auditoria de 2026-11-05 dentro da spec, para que a implementação não dependa de artefatos locais fora do git. Cada achado aberto aponta o requisito que o resolve e a evidência (arquivo ou saída de comando) encontrada na varredura. Os números de linha refletem o código na data da auditoria e podem ter se deslocado.

### A.1 Resultado literal de `make check` e `make lint`

- `make check`: **exit=0** (gofmt + go vet + build + unit + e2e, todos verdes).
- `make lint`: **exit=0**, mas o golangci-lint foi **pulado**. Saída: `golangci-lint v1.64.8 é incompatível com este toolchain Go (não decodifica o export data); pulando. Gate real = gofmt + go vet.` Promovido a [R4](#r4-qualidade-reativar-análise-estática-golangci-lint-qg-1-tst-1--média).

### A.2 Achados abertos

| ID | Sev. | Requisito | Evidência |
|----|------|-----------|-----------|
| SEG-1 | alta | R1.1 | `internal/api/app/patients.go` (`GetPatientChart`) revela notas e anamnese em claro via `Reveal()` em `internal/api/app/dto.go`, sem auditoria. `ProntuarioService` (`internal/app/prontuario_service.go`) não tem dependência de auditoria; o wiring o constrói com `app.NewProntuarioService(prontuarioRepo, nil)` em `internal/api/wiring.go`. `HistoryService` e `DayFeedService` auditam `AuditActionRead`; o chart não. |
| SEG-2 | alta | R1.2 | `AddNote` e `UpsertAnamnesis` em `internal/app/prontuario_service.go` e `internal/api/app/patients.go` persistem sem gerar `audit_log`. |
| DRV-1 | alta | R2 | `internal/drivesync/drivesync.go` (`mirror`/`uploadSealed`): lê o snapshot inteiro, cifra e sobe o blob completo (`<backup>.enc`). Sem hash, manifesto, diff ou dedup. O steering `psicoman-google-api.md` pede incremental por manifesto. |
| DRV-2 | média | R3 | `internal/api/wiring.go` (`wireDriveSync`) exige `client_id`, `client_secret` e `drive_folder`; vazio vira `drivesync.New(nil, …)` (no-op). `internal/integration/google/drive.go` só tem `Upload` de arquivo, sem `files.create` de pasta. Reproduzido numa sessão de teste real: o upload com o nome da pasta deu `404`, e com o folder id criado à mão funcionou. |
| QG-1 / TST-1 | média | R4 | Saída do `make lint` em A.1. `.golangci.yml` existe mas não executa. |
| TST-2 | baixa | R5 | Há teste de auditoria para history/dayfeed em `internal/app/feed_service_test.go`; nenhum para `GetPatientChart`, `AddNote` ou `UpsertAnamnesis`. |
| DOC-1 | média | R6.1 | `psicoman-google-api.md` cita o escopo `gmail.send` e tem seção Gmail, mas Gmail está fora de escopo (`docs/requirements.md` §3.7/§5). `AuthorizeScopes` em `internal/integration/google/oauth.go` pede só `calendar` e `drive.file`. |
| DOC-2 | média | R6.2 | `docs/architecture.md` §0.10 descreve o DR como funcional, sem citar o pré-requisito da pasta nem o snapshot inteiro. `docs/deploy.md` não traz o passo de criar a pasta. |
| DOC-3 | baixa | R6.3 | `.kiro/specs/pocketbase-finish/tasks.md`: a tabela "Status ao vivo" tem a Fase 10 duas vezes (`⬜ pendente` e `✅ concluída`), e a seção da Fase 10 segue com checkboxes `[ ]`. A onda foi mergeada no PR #4. |
| SEG-3 | média | R7.1 | Nenhum middleware de rate limit em `internal/**/*.go`. O limite existe só no nginx de borda (`deploy/nginx/portal.conf`, por IP). `POST /v1/portal/register` com e-mail repetido não é limitado em lugar nenhum. |
| SEG-4 | baixa | R7.2 | `internal/api/admin/google_oauth.go`: `GoogleAuthorize` gera `state := ulid.New()`; `GoogleCallback` não lê nem compara `state`. |
| SEG-5 | baixa | R7.3 | `config.dev.yaml` tem segredos reais em claro (chaves de cifra, client secret do Google, api key do Gemini). O arquivo é git-ignored e não rastreado. Os valores foram expostos no ambiente de dev. |
| QG-2 | baixa | R7.4 | `internal/api/app/sessions.go`: `// TODO Fase 5: extrair o autor do AuthAppStub; por ora é a terapeuta.` O `actor` do audit de sessão é fixo. `httpx.ActorIdentity` grava a identidade do Pangolin, mas ninguém a consome. |
| OBS-1 | média | R8 | Probes `/v1/health`, `/v1/livez` e `/v1/readyz` existem (`internal/api/api.go`). Só há timing por rota **logado** (`httpx.Timing`/`httpx.Logging`), sem endpoint de métricas. Métricas adiadas em `docs/decisions.md` (2026-10-24) e promovidas nesta onda a pedido do Rafael. |

### A.3 Verificado e correto (não vira requisito)

- **Features no `/v1`:** os gaps R1 da `pocketbase-finish` estão fechados. `internal/api/app/mount.go` registra day-feed, history, session-metrics, calendário, custos, locais, disponibilidades, origens, planos e pedidos de agendamento. `internal/api/swagger/contract_test.go` falha o build se rota e Swagger divergirem.
- **OAuth 3-legged:** autorização real, refresh token cifrado em `google_token`, estado "reautorização necessária" no `/readyz`.
- **Gate do Pangolin e modo dev:** `internal/api/httpx/auth.go` é fail-closed. Sem header e com `pangolin.dev_identity` vazio, responde 401; o header real sempre vence a identidade dev; o boot loga WARN quando o modo dev está ativo. Coerente com o ADR de 2026-11-05.
- **Gate de aprovação do portal:** `internal/api/portal/gate.go` (não aprovado → 403, sem sessão → 401).
- **Não vazamento clínico:** `Reveal()` só aparece nos mappers do ator `app`; o portal tem `leak_test.go`.
- **Arquitetura:** `internal/archtest/arch_test.go` prova que `domain`, `app` e `config` não importam o PocketBase. ULID, centavos `int64`, `America/Sao_Paulo` e envelope PT-BR respeitados.
- **Testes:** suíte E2E ampla em `test/e2e/`, sem `t.Skip` e sem chamada à API real do Google ou do Gemini (sempre fakes atrás de interface).
- **Gmail:** sem código Gmail, coerente com a decisão de produto (cobrança em PDF e envio por WhatsApp).

### A.4 Registro da revisão

O `requirements.md` passou por revisão independente (veredito **APPROVED**, zero achados bloqueantes), que conferiu formato contra a `pocketbase-finish`, rastreabilidade de cada requisito a um achado real e cobertura dos pontos levantados na sessão (pasta do Drive, backup incremental, modo dev de identidade). A revisão cobriu R1 a R7. O **R8 (métricas)** entrou depois, a pedido do Rafael, e foi revisado apenas pelo autor da edição. O modo dev de identidade não foi levantado como requisito porque está implementado e correto (A.3).

Observações não bloqueantes da revisão: R2.3 e R6.2 tocam `docs/decisions.md` e o steering para o caso "snapshot inteiro" (redundância aceita, reforça a rastreabilidade); o rótulo de severidade "média/baixa" em R6 reflete DOC-1 e DOC-2 (média) com DOC-3 (baixa).
