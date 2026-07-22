# Guia de Testes — Psicoman

## Testes automatizados

```bash
make test
```

Executa `go test ./... -count=1`. Cobre:
- `internal/domain` — regras de cancelamento/reagendamento
- `internal/service` — criação de appointments, conflitos
- `internal/storage` — CRUD de pacientes, appointments, migrations
- `internal/web` — handlers HTTP (httptest com router completo)

## Dados de teste (seed)

### Pré-requisitos
- Servidor rodando com DEV_MODE=true em localhost:8080
- `make run-api` ou `make run`

### Carregar dados de teste

```bash
bash scripts/seed-test-data.sh
```

Cria:
- 3 pacientes: "TEST Ana Silva", "TEST Bruno Costa", "TEST Carla Lima"
- 12 consultas (4 por paciente: 2 no mês atual, 2 no mês anterior)
- Emails: test.ana@test.com, test.bruno@test.com, test.carla@test.com
- Telefones: (11) 91111-0001 a 0003

### Limpar dados de teste

```bash
curl -X DELETE http://localhost:8080/api/dev/test-data -H "X-Dev-Auth: dev-local"
```

Remove TODOS os registros onde:
- Nome do paciente começa com "TEST "
- OU email termina com @test.com

A limpeza é cascata: remove consultas, evoluções, documentos, contratos, pagamentos e respostas de anamnese dos pacientes de teste.

Resposta exemplo:
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

### Convenções para dados de teste

| Regra | Exemplo |
|-------|---------|
| Nome deve começar com "TEST " | "TEST Maria Silva" |
| Email deve usar domínio @test.com | "test.maria@test.com" |
| Ambas as condições são verificadas (OR) | Basta uma para ser removível |

### Reset completo do banco

```bash
rm -f data/db/dev.sqlite*
make run-api   # recria com migrations
bash scripts/seed-test-data.sh  # popula com dados de teste
```

## Validação manual

Após carregar os dados de teste:

1. Acesse http://localhost:8080/psych — deve ver agenda com consultas de hoje (se houver)
2. Clique em "Pacientes" — deve ver TEST Ana, TEST Bruno, TEST Carla
3. Clique em um paciente — deve ver calendário mensal com indicadores nos dias 5 e 15
4. Navegue para o mês anterior — deve ver indicadores nos dias 8 e 20
5. Clique em dia vazio — modal de criar consulta com data pré-preenchida
6. Clique em indicador de consulta — modal de edição
7. Alterne para tema escuro (botão 🌙 no header)
8. Teste responsividade (viewport < 768px)
