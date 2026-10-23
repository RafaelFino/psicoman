# pocketbase-core — Requisitos

Quarta onda de evolução do Psicoman e a primeira que é uma **reescrita da base técnica**, não um incremento. A onda adota o **PocketBase como framework** (biblioteca Go embarcada no nosso binário) como o coração do sistema: SQLite relacional com constraints, servidor HTTP, file storage, logging, auth/OAuth2, migrations e admin UI vêm de dentro dele. A plataforma roda **sempre atrás do [`Pangolin`](https://docs.fossorial.io/)** e nunca é exposta direto à internet. A decisão de arquitetura está lastreada em [`docs/decisions.md` (2026-10-21)](../../../docs/decisions.md).

> Fonte de verdade durável: [`docs/requirements.md`](../../../docs/requirements.md) e [`docs/architecture.md`](../../../docs/architecture.md). Esses dois serão **reescritos** para esta arquitetura na fase de fechamento das tasks desta onda (não antes). Enquanto isso, esta spec é a referência de requisitos do rumo `pocketbase-core`; onde houver conflito com as versões antigas desses docs, o descrito aqui prevalece para o escopo da onda.

## Resumo da arquitetura e premissas

- **Single-tenant, uma VM por terapeuta.** Um único usuário primário (a terapeuta) operando de forma essencialmente sequencial; eventualmente algum paciente se cadastrando. Num dia agitado, dois ou três cadastros novos. Nada de alta concorrência.
- **`Pangolin` na borda.** Todo o tráfego entra pelo gateway `Pangolin`. A aplicação nunca fica aberta diretamente para a internet; a barreira forte de autenticação/exposição é o gateway, e o app mantém só o mínimo de defense-in-depth no servidor.
- **V1 sem lastro.** A plataforma **ainda não foi publicada** e não há dados reais em produção. A base pode ser reescrita à vontade; esta onda vira a **arquitetura padrão da V1**, substituindo as explorações anteriores (SQLite puro, Drive-como-banco, corte híbrido Drive+SQLite — todas registradas como alternativas descartadas em [`docs/decisions.md`](../../../docs/decisions.md)).
- **Binário único.** Um executável (`cmd/psicoman`) substitui os antigos `cmd/admin` + `cmd/portal`. A segregação admin/portal deixa de ser física (dois processos) e passa a ser lógica (grupos de rota + regras de auth). Um binário simplifica o roll out.
- **Cifra por campo.** O conteúdo clínico sensível é cifrado em repouso campo a campo via o tipo de domínio `SealedText` (AES-256-GCM); os demais campos ficam em claro e indexáveis. A busca clínica se dá pelas **tags** da IA (em claro), não pelo corpo (cifrado).

## Convenções

- Requisitos em **EARS** ("O sistema DEVE …", "QUANDO … o sistema DEVE …").
- Toda mensagem ao usuário em **PT-BR**, tom acolhedor, zero jargão (público leigo).
- Valores monetários em centavos (`int64`, BRL); datas/horas no fuso `America/Sao_Paulo` em ISO-8601.
- Toda regra sensível é **imposta no servidor** (defense-in-depth), mesmo com o `Pangolin` na frente; a UI apenas reflete.
- **Nomenclatura canônica** (idêntica em toda a documentação da onda): PocketBase **como framework**, binário único; gateway `Pangolin`; tipo de cifra `SealedText`; adapter de persistência `platform/pb` (pacote `internal/platform/pb`); pacote de cripto `internal/platform/crypto`; bounded contexts `patient`, `clinical`, `billing`, `scheduling`, `costs`, `therapist`; envelope de resposta `{ message, elapsed_ms, data, error }` (`message` em PT-BR); prefixo de rotas próprias `/v1/*` (namespaces lógicos `admin`/`portal`); envelope da cifra `versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext` (base64); algoritmo AES-256-GCM.

## Resumo das evoluções

1. **R1** — PocketBase como framework e binário único.
2. **R2** — Segregação lógica admin/portal atrás do `Pangolin`.
3. **R3** — Auth simplificada e gate de aprovação do paciente.
4. **R4** — Domínio rico por bounded context.
5. **R5** — Ports & Adapters: `platform/pb` isola o PocketBase.
6. **R6** — Cripto por campo (`SealedText`).
7. **R7** — Contrato de API `/v1` com envelope padrão e Swagger.
8. **R8** — Integração Google Calendar síncrona.
9. **R9** — Google Drive como fallback/DR via hook de backup (`drivesync`).
10. **R10** — IA Gemini: resumo + tags com índice invertido por paciente/ano.
11. **R11** — Persistência com constraints/relations e padrões de domínio.

---

## R1. PocketBase como framework e binário único

**Contexto:** as explorações anteriores (SQLite puro, Drive como fonte da verdade, corte híbrido) não entregavam, de forma barata, **invariantes transacionais + HTTP + file-server + auth + admin** num processo só. O PocketBase, importado como biblioteca Go, entrega tudo isso de dentro de um único binário.

### R1.1 PocketBase embarcado
- O sistema DEVE embarcar o PocketBase **como framework** (biblioteca Go importada em `cmd/psicoman/main.go`), instanciando-o com `pocketbase.New()` e registrando as rotas próprias no hook `app.OnServe()`.
- O sistema DEVE aproveitar do PocketBase: o SQLite relacional com constraints/relations, o servidor HTTP, o file storage/server, o logging, o auth/OAuth2, as migrations e a admin UI.

### R1.2 Binário único
- O sistema DEVE ser entregue como um **único executável** (`cmd/psicoman`), substituindo os antigos `cmd/admin` e `cmd/portal`.
- O binário único DEVE simplificar o roll out: um artefato, um processo, um `data dir`.
- O sistema NÃO DEVE exigir um servidor HTTP intermediário próprio além do que o PocketBase já fornece.

### R1.3 Invariantes transacionais
- O sistema DEVE recuperar, via SQLite do PocketBase, os invariantes transacionais que as alternativas descartadas não davam: unicidade (`UNIQUE`), chaves estrangeiras (FK), `ON CONFLICT` e transação.

---

## R2. Segregação lógica admin/portal atrás do `Pangolin`

**Contexto:** como tudo roda atrás do `Pangolin` e nunca é exposto direto à internet, não há ganho em separar admin e portal em processos distintos.

### R2.1 Segregação lógica
- O sistema DEVE segregar **admin** e **portal** de forma **lógica** (namespaces de rota `/v1/admin/*` e `/v1/portal/*` + regras de auth por grupo), não por processos físicos separados.
- QUANDO uma requisição chega a um namespace, o sistema DEVE aplicar a cadeia de middleware e as regras de autorização daquele namespace, independentemente de compartilhar o mesmo binário.

### R2.2 Exposição controlada
- O sistema DEVE expor de forma controlada, para a internet (via `Pangolin`), apenas o que o paciente precisa (cadastro/login e os recursos mínimos do portal).
- As superfícies administrativas DEVEM assumir o `Pangolin` como barreira de borda e não replicar uma camada pesada de autenticação própria.

---

## R3. Auth simplificada e gate de aprovação do paciente

**Contexto:** o `Pangolin` é a barreira forte; o app confia no gateway e mantém só o mínimo de defense-in-depth. O gate de aprovação do paciente (herdado das ondas anteriores) permanece como regra de negócio.

### R3.1 Identidade do admin
- O sistema DEVE reconhecer a identidade do admin a partir de um **header injetado pelo `Pangolin`**, sem exigir um fluxo de login próprio para o administrador.
- QUANDO o header de identidade do `Pangolin` estiver ausente numa rota `/v1/admin/*`, o sistema DEVE recusar o acesso (defense-in-depth), mesmo confiando no gateway.

### R3.2 Cadastro/login do paciente
- O sistema DEVE expor cadastro e login do paciente via auth do PocketBase (coleção de auth), restrito ao namespace `/v1/portal/*`.

### R3.3 Gate de aprovação
- O paciente recém-cadastrado DEVE nascer com estado de aprovação `pendente`.
- QUANDO o terapeuta aprova ou rejeita um paciente, o sistema DEVE registrar o estado (`aprovado` | `rejeitado`) e DEVE impor essa decisão **no servidor** via hook do PocketBase.
- QUANDO um paciente `pendente` ou `rejeitado` tenta acessar recursos do portal além do próprio cadastro, o sistema DEVE negar (403), independentemente do que a UI permita.

---

## R4. Domínio rico por bounded context

**Contexto:** a onda troca o "domínio magro + services gordos" por um **domínio rico**: os agregados carregam comportamento, serialização e ações; os serviços ficam enxutos, apenas orquestrando.

### R4.1 Domínio rico
- Os agregados de domínio DEVEM carregar o próprio comportamento de negócio, a serialização/desserialização, a formatação de log e as ações que lhes dizem respeito (OO bem feita).
- Os serviços de negócio (`internal/app`) DEVEM ser enxutos, orquestrando os agregados e os adapters, sem concentrar regra que pertence ao domínio.

### R4.2 Bounded contexts
- O domínio DEVE ser segregado nos seguintes bounded contexts, cada um num pacote sob `internal/domain`: `patient`, `clinical`, `billing`, `scheduling`, `costs`, `therapist`.
- Cada bounded context DEVE ser a autoridade sobre suas entidades e invariantes; dependências entre contexts DEVEM ser explícitas e mínimas.

### R4.3 Comportamento no domínio
- QUANDO uma entidade precisa serializar-se para persistência ou para a API, o sistema DEVE delegar essa responsabilidade ao próprio agregado (ou a tipos de valor dele), não a um mapeador externo onisciente.
- A formatação de log de uma entidade DEVE omitir conteúdo clínico sensível por padrão.

---

## R5. Ports & Adapters: `platform/pb` isola o PocketBase

**Contexto:** para não ficarmos amarrados ao PocketBase, o mapeamento domínio↔PocketBase fica isolado atrás de um adapter. Trocar o PocketBase por outra base significa reescrever só o adapter.

### R5.1 Domínio não depende do PocketBase
- O pacote de domínio (`internal/domain/*`) NÃO DEVE importar o PocketBase; os agregados são a autoridade e desconhecem o framework de persistência.
- As operações de persistência DEVEM ser expressas por **interfaces de porta** (repositórios) definidas no domínio/app.

### R5.2 Adapter `platform/pb`
- O mapeamento domínio↔`Record` do PocketBase DEVE viver exclusivamente no pacote `internal/platform/pb`.
- QUANDO o framework de persistência mudar, a alteração DEVE ficar contida no `internal/platform/pb` e nas migrations, sem tocar o domínio.

---

## R6. Cripto por campo (`SealedText`)

**Contexto:** o conteúdo clínico sensível (corpo da sessão, anamnese, notas, resumo de IA) precisa ficar cifrado em repouso, mas sem perder a capacidade de buscar/indexar os metadados não sensíveis. A escolha é **cifra por campo** via um tipo de domínio.

### R6.1 Tipo `SealedText`
- Os campos clínicos sensíveis DEVEM ser representados pelo tipo wrapper `SealedText`, que guarda o texto cifrado em repouso.
- O `SealedText` DEVE expor o texto em claro **somente** sob demanda, via `Reveal(ctx)`.

### R6.2 Envelope de cifra
- A serialização do `SealedText` para persistência DEVE produzir o envelope `versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext` (campos em base64, separados por ponto).
- O algoritmo DEVE ser **AES-256-GCM**, implementado no pacote `internal/platform/crypto`.
- O campo `id_da_chave` DEVE habilitar **rotação de chave**; os campos `versao_do_esquema` e `algoritmo` DEVEM habilitar a evolução do esquema/algoritmo sem quebrar dados antigos.

### R6.3 Serialização nunca vaza o claro
- QUANDO o `SealedText` é serializado para JSON (`MarshalJSON`), o sistema DEVE emitir **sempre o envelope cifrado**, nunca o texto em claro.
- QUANDO o `SealedText` é desserializado (`UnmarshalJSON`), o sistema DEVE manter o conteúdo **cifrado** (parse do envelope), sem decifrar automaticamente.

### R6.4 Chave via `KeyProvider`
- A chave de cifra DEVE ser obtida via configuração, atrás de uma interface `KeyProvider` (resolve a chave por `id_da_chave` e informa o `id` corrente).
- O `KeyProvider` DEVE permitir, no futuro, trocar a origem da chave (config, vault externo) sem mudar o `SealedText`.

### R6.5 Cifra por campo, busca por tags
- A cifra DEVE ser **por campo**: apenas os campos clínicos sensíveis ficam cifrados. Nome, email, telefone, datas, status e valores DEVEM permanecer em claro e indexáveis.
- A busca clínica DEVE se dar pelas **tags** da IA (em claro), não pelo corpo clínico (cifrado), preservando a capacidade de busca sem expor o conteúdo.

---

## R7. Contrato de API `/v1` com envelope padrão e Swagger

**Contexto:** o contrato de API é revisado do zero. As rotas próprias vivem sob `/v1/*`, registradas sobre o router do PocketBase; a REST automática do PocketBase não é o contrato público.

### R7.1 Rotas próprias versionadas
- As rotas próprias DEVEM viver sob o prefixo `/v1/*`, com namespaces lógicos `/v1/admin/*` e `/v1/portal/*`.
- A REST automática gerada pelo PocketBase NÃO DEVE ser o contrato público do sistema; o contrato são as rotas `/v1/*` próprias.

### R7.2 Envelope de resposta
- Toda resposta das rotas `/v1/*` DEVE usar o envelope `{ message, elapsed_ms, data, error }`, nesta ordem, com `message` em PT-BR.
- QUANDO a operação é bem-sucedida, o sistema DEVE preencher `data` e deixar `error` nulo; QUANDO falha, DEVE preencher `error` com um código/descrição e deixar `data` nulo.
- O campo `elapsed_ms` DEVE refletir o tempo de processamento da requisição em milissegundos.

### R7.3 Swagger
- O contrato `/v1/*` DEVE ser documentado em Swagger/OpenAPI, mantido no mesmo ciclo em que o contrato muda.

---

## R8. Integração Google Calendar síncrona

**Contexto:** a integração com o Google Calendar continua valendo e permanece **síncrona** — é o único Google no caminho de request.

### R8.1 Calendar síncrono
- QUANDO o terapeuta agenda/confirma uma sessão, o sistema DEVE, de forma síncrona, criar o evento no Google Calendar e o link de atendimento (Meet) quando a modalidade for online.
- Antes de confirmar a sessão, o sistema DEVE checar conflito de horário via `freebusy`.
- QUANDO o Google não estiver configurado (sem credenciais) ou indisponível, o sistema DEVE degradar sem erro fatal (sem panic), registrando a condição, coerente com o comportamento já estabelecido em `dev_mode`.

---

## R9. Google Drive como fallback/DR via hook de backup (`drivesync`)

**Contexto:** usar o Google Drive diretamente como base de dados foi descartado. O Drive deixa de ser síncrono e vira apenas **fallback/DR**: um espelho frio dos backups.

### R9.1 Drive fora do caminho de request
- O Google Drive NÃO DEVE ser fonte da verdade nem estar no caminho de request de nenhuma operação de negócio.

### R9.2 Sync via hook de backup
- QUANDO o PocketBase conclui um **backup** (snapshot), um hook DEVE disparar o sync do snapshot para o Google Drive (pasta fria de DR).
- O sync para o Drive DEVE viver em `internal/drivesync` e PODE rodar como segundo processo/hook, isolado do caminho de request.
- O escopo OAuth do Drive DEVE permanecer mínimo (`drive.file`) e o snapshot DEVE ser cifrado antes do upload.

---

## R10. IA Gemini: resumo + tags com índice invertido por paciente/ano

**Contexto:** a IA (Gemini) continua valendo sem mudanças de propósito: resumo clínico + tagueamento, com índice invertido de tags por paciente/ano. O corpo clínico é `SealedText` (cifrado); as **tags ficam em claro** para habilitar a busca.

### R10.1 Resumo e tagueamento
- QUANDO o terapeuta **dispara explicitamente** o processamento de IA sobre uma sessão, o sistema DEVE gerar, via `internal/integration/gemini`, um resumo clínico e um conjunto de tags temáticas.
- O resultado da IA DEVE ser **revisável** pelo terapeuta antes de ser consolidado; nada é persistido como definitivo sem revisão.

### R10.2 Índice invertido de tags por paciente/ano
- O sistema DEVE manter um **índice invertido de tags por paciente e ano** (`tag -> lista de IDs de sessão`), estritamente isolado por paciente (sem busca cross-paciente).
- As **tags** DEVEM ser armazenadas em **claro** (indexáveis), enquanto o corpo clínico da sessão permanece `SealedText` (cifrado).

### R10.3 Opt-in e auditoria
- O processamento por IA DEVE ser **opt-in por instância**, com default **desligado**.
- QUANDO a IA roda, o sistema DEVE registrar uma auditoria `ai.process` **sem conteúdo clínico** nos metadados (minimização de dados).
- O sistema DEVE minimizar os dados enviados à IA ao estritamente necessário para o resumo/tagueamento.

### R10.4 Base legal
- O uso de IA DEVE ser documentado com a base legal: os dados são da própria terapeuta (controladora), que já faz esse trabalho de resumo/tagueamento manualmente; a IA apenas assiste.

---

## R11. Persistência com constraints/relations e padrões de domínio

**Contexto:** as coleções do PocketBase recuperam as garantias relacionais; os padrões de domínio do projeto permanecem não-negociáveis.

### R11.1 Constraints e relations
- As coleções do PocketBase DEVEM usar constraints/relations para recuperar: unicidade de email/CPF (`UNIQUE`), FK entre entidades e idempotência de efeito financeiro por chave única.
- QUANDO uma operação financeira é reprocessada com a mesma chave de idempotência, o sistema NÃO DEVE criar efeito duplicado (débito/pagamento).

### R11.2 Padrões de domínio
- Toda entidade DEVE ter identificador **ULID**.
- Todo valor monetário DEVE ser `int64` em **centavos BRL**.
- Todo instante DEVE ser gravado/transportado no fuso `America/Sao_Paulo` em ISO-8601.
- Toda operação de I/O DEVE receber `context.Context` com timeout.
- Toda mensagem de erro ao usuário DEVE ser em PT-BR.

---

## Fora do escopo

- **Multi-tenancy.** A plataforma permanece single-tenant (uma VM por terapeuta); nada de isolamento multi-inquilino nesta onda.
- **Vault externo de chave.** A chave de cifra fica atrás do `KeyProvider`; integrar um vault externo (KMS/Secrets Manager) é evolução futura — o `KeyProvider` só deixa a porta aberta.
- **Migração de dados reais.** A V1 não foi publicada; não há dados reais a migrar.
- **UI de produto nova** além do necessário à reorganização (binário único, portal mínimo). Reengenharia visual de telas não é objetivo desta onda.

## Glossário

- **PocketBase como framework** — PocketBase usado como biblioteca Go embarcada no nosso binário (não como servidor standalone separado), com `pocketbase.New()` + `app.OnServe()`.
- **`SealedText`** — tipo de domínio que guarda texto clínico cifrado em repouso; expõe o claro só via `Reveal(ctx)` e serializa sempre o envelope cifrado.
- **Envelope de cifra** — `versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext` (base64, separado por ponto); `id_da_chave` habilita rotação, `versao_do_esquema`/`algoritmo` habilitam evolução.
- **`platform/pb`** — pacote `internal/platform/pb`, o único que conhece o PocketBase; mapeia domínio↔`Record`.
- **Bounded context** — recorte de domínio autônomo (`patient`, `clinical`, `billing`, `scheduling`, `costs`, `therapist`).
- **`Pangolin`** — gateway de borda; barreira forte de exposição/auth na frente da aplicação.
- **`KeyProvider`** — interface que resolve a chave de cifra por `id_da_chave` e informa o `id` corrente (habilita rotação).
- **Índice invertido de tags** — mapa `tag -> lista de IDs de sessão`, por paciente e ano, com as tags em claro para busca.
- **Fallback/DR** — o Google Drive como espelho frio dos backups (disaster recovery), fora do caminho de request.
