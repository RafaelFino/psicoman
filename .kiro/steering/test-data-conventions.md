---
inclusion: auto
---

# Dados de Teste — Convenções

## Identificação de dados de teste

Todo dado criado para fins de teste DEVE seguir estas convenções:

- **Pacientes de teste**: nome começa com "TEST " (com espaço), e-mail termina com @test.com
- Exemplos: "TEST Ana Silva" / "test.ana@test.com"

## Scripts disponíveis

### Carregar dados de teste

```bash
bash scripts/seed-test-data.sh
```

Requer servidor rodando com DEV_MODE=true em localhost:8080.

### Limpar dados de teste

```bash
curl -X DELETE http://localhost:8080/api/dev/test-data -H "X-Dev-Auth: dev-local"
```

Remove cascata: pacientes + appointments + session_notes + documents + contracts + payments + anamnesis_responses.

### Reset completo

```bash
rm -f data/db/dev.sqlite*
make run-api
bash scripts/seed-test-data.sh
```

## Regras ao criar novos dados de teste

- Sempre usar prefixo "TEST " no nome do paciente
- Sempre usar domínio @test.com no email
- O endpoint DELETE /api/dev/test-data identifica dados de teste por AMBAS as condições (OR)
- Nunca criar dados de teste com nomes/emails que possam se confundir com dados reais
- Scripts de seed devem ser idempotentes (rodar 2x não deve duplicar — usar emails únicos)

## Endpoint de limpeza

```
DELETE /api/dev/test-data
Headers: X-Dev-Auth: dev-local
```

Resposta:
```json
{
  "patients": 3,
  "appointments": 12,
  "session_notes": 0,
  "documents": 0,
  "contracts": 0,
  "payments": 0,
  "anamnesis_responses": 0
}
```

Disponível apenas quando DEV_MODE=true.
