---
inclusion: fileMatch
fileMatchPattern: "internal/**/*.go"
---

# Go — Padrões do Service Layer

## Camadas e responsabilidades

```
cmd/server/main.go     → composição de dependências, inicialização
internal/web/          → HTTP handlers, middleware, routing (Gin)
internal/service/      → lógica de negócio, orquestração
internal/storage/      → acesso a dados (SQLite queries)
internal/domain/       → tipos, enums, regras de domínio puras
```

## Regras por camada

### domain/
- **Zero dependências externas** (nem storage, nem service)
- Contém: tipos (structs), enums (const), funções de validação puras
- Funções recebem valores, retornam valores ou error
- Sem I/O, sem banco, sem HTTP

### storage/
- Recebe `*DB` (wrapper do `*sql.DB`)
- Cada arquivo = um aggregate root (patient.go, appointment.go, etc.)
- Nomes de métodos: `List*`, `Get*`, `Create*`, `Update*`, `Delete*`
- Retorna domain types, nunca tipos internos do SQL
- Erros do SQL propagados como-são (o service decide como tratar)

### service/
- Recebe `*storage.DB` como parâmetro (injetado pelo handler)
- Orquestra múltiplas operações de storage + lógica de negócio
- Pode chamar domain para validações
- Pode chamar ports externos (Calendar, etc.)
- Não conhece HTTP, Gin, ou headers

### web/
- Handlers traduzem HTTP → service calls → HTTP responses
- Binding de input com `ShouldBindJSON` ou `FormFile`
- Erros retornados como `gin.H{"error": msg}` com status code adequado
- Não contém lógica de negócio (delega para service)

## Padrão para novos endpoints

```go
// 1. Definir input struct (se necessário)
type CreateXInput struct {
    FieldA string
    FieldB int
}

// 2. Service method
func (s *XService) Create(db *storage.DB, in CreateXInput) (*domain.X, error) {
    // validação
    // storage calls
    // side effects (calendar, etc.)
    return result, nil
}

// 3. Handler (em web/handlers_psych.go ou handlers_patient.go)
func (a *App) createX(c *gin.Context) {
    var in service.CreateXInput
    if err := c.ShouldBindJSON(&in); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    result, err := a.XService.Create(getDB(c), in)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusCreated, result)
}
```

## Tratamento de erros

- Erros de validação: `400 Bad Request`
- Não encontrado: `404 Not Found`
- Conflito (ex: horário ocupado): `400 Bad Request` com mensagem descritiva
- Erros internos: `500 Internal Server Error` (logar detalhes, retornar mensagem genérica)
- Nunca retornar stack traces ao cliente

## Migrations

- Arquivo único: `internal/storage/migrations/001_init.sql`
- Para adicionar tabelas/colunas: criar `002_*.sql` e atualizar `db.migrate()`
- Migrations são idempotentes (`CREATE TABLE IF NOT EXISTS`, `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`)
- Nunca deletar migrations antigas

## Testes

- Cada camada tem seus testes independentes
- Storage tests usam `t.TempDir()` para DB isolado
- Service tests usam DB real (SQLite in-memory via TempDir)
- Web tests usam `httptest` com o router completo
- Assertions com `testify/assert` e `testify/require`
