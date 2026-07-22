---
inclusion: fileMatch
fileMatchPattern: "internal/web/**"
---

# Psicoman — Práticas de Segurança

## Autenticação

### Pacientes (público)
- Login exclusivamente via Google OAuth2
- Token JWT local emitido após callback OAuth (validade: 7 dias)
- JWT assinado com `JWT_SECRET` (variável de ambiente, nunca hardcoded em produção)
- Token armazenado no frontend em `localStorage`
- Cada request do paciente valida o JWT no middleware `PatientAuth()`

### Psicólogo (privado)
- Em produção: Pangolin reverse proxy injeta headers `X-User-Id`, `X-User-Email`, `X-User-Role`
- O Go **confia** nesses headers — a segurança vem do Pangolin não expor a porta 8080 diretamente
- Em dev: header `X-Dev-Auth` com secret configurável substitui Pangolin
- DEV_MODE **nunca** deve estar ativo em produção

## Autorização

- Paciente só acessa seus próprios dados (filtro por `patient_id` no middleware)
- Download de documentos verifica ownership: `doc.PatientID != pid`
- Psicólogo acessa tudo (é o admin do sistema)

## Validações obrigatórias

- Input binding com `ShouldBindJSON` do Gin (validação estrutural)
- Validações de negócio no service layer (não no handler)
- Erros nunca expõem stack traces — apenas mensagens user-friendly em português

## Headers de segurança ✅

Implementado no middleware `SecurityHeaders()` em `router.go`:
```go
c.Header("X-Content-Type-Options", "nosniff")
c.Header("X-Frame-Options", "DENY")
c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
c.Header("X-XSS-Protection", "1; mode=block")
```

## Secrets e variáveis de ambiente

- `JWT_SECRET`: DEVE ser forte em produção (min 32 chars, aleatório)
- `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`: credenciais OAuth do Google Cloud Console
- `DEV_SECRET`: apenas para dev local, nunca em produção
- Nunca commitar `.env` com secrets reais (`.gitignore` já cobre)

## Rate limiting (a implementar)

Endpoints públicos devem ter rate limiting:
- `/api/auth/patient/*`: 10 req/min por IP
- `/api/patient/*`: 60 req/min por token

## CORS

- Em produção: Pangolin serve tudo no mesmo domínio → sem CORS issues
- Em dev: tudo servido pelo Go na mesma porta :8080 → sem CORS issues
- Não é necessário configurar CORS headers no Go

## Upload de arquivos

- Validar mime type no servidor (não confiar apenas no Content-Type do request)
- Limitar tamanho de upload (configurar `gin.MaxMultipartMemory`)
- Armazenar fora do diretório web-acessível
- Nomes de arquivo: UUID + original filename (evita path traversal)
