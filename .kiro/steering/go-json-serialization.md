---
inclusion: fileMatch
fileMatchPattern: "**/*.go"
---

# Go JSON Serialization Rules

## Obrigatório: json tags em todas as structs expostas via API

Toda struct que será serializada como resposta HTTP (via `c.JSON()` do Gin) **deve** ter `json` tags explícitas em todos os campos.

### Convenção de nomes

- Usar **snake_case** nas json tags: `json:"field_name"`
- Campos opcionais que podem ser vazios: `json:"field_name,omitempty"`
- Os templates Go e htmx esperam todos os campos em snake_case

### Exemplo correto

```go
type Patient struct {
    ID        string     `json:"id"`
    Email     string     `json:"email"`
    Name      string     `json:"name"`
    Phone     string     `json:"phone"`
    BirthDate *time.Time `json:"birth_date"`
    CreatedAt time.Time  `json:"created_at"`
}
```

### Exemplo INCORRETO (nunca fazer)

```go
type Patient struct {
    ID        string
    Email     string
    Name      string
}
// Isso serializa como {"ID":"...","Email":"...","Name":"..."} — incompatível com o frontend!
```

### Regra

- Ao criar uma nova struct que será retornada em endpoints HTTP, adicionar json tags imediatamente
- Ao adicionar campos a structs existentes, incluir a json tag no mesmo commit
- Os tipos de domínio ficam em `internal/domain/types.go` e são a referência canônica
