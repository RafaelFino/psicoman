---
inclusion: fileMatch
fileMatchPattern: "frontend/src/**"
---

# Frontend — Contrato com a API

## Nomes de campos

O backend Go retorna JSON com campos em **snake_case**. Exemplos:

- Paciente: `id`, `name`, `email`, `phone`, `birth_date`, `anamnesis`, `created_at`
- Appointment: `id`, `patient_id`, `patient_name`, `type`, `status`, `scheduled_at`, `duration_minutes`, `meet_link`, `notes`, `report_html`
- Payment: `id`, `patient_id`, `patient_name`, `amount_cents`, `status`, `due_date`, `received_at`
- Cost: `id`, `description`, `amount_cents`, `month`, `year`, `category`
- FinanceSummary: `month`, `year`, `total_received`, `total_pending`, `total_costs`, `balance`, `payments`, `costs`
- MonthlyReport: `month`, `year`, `patient_id`, `patient_name`, `appointments`, `total_amount`

## Valores monetários

- Sempre em **centavos** (int): `amount_cents`, `total_received`, etc.
- Para exibir: `(cents / 100).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })`

## Listas nulas

- Go retorna `null` para slices vazios (não `[]`)
- Sempre tratar: `data?.map(...)` ou `data || []`

## Autenticação no frontend

- Psicólogo: em dev mode, `X-Dev-Auth: dev-local` é adicionado automaticamente pelo `api.js`
- Paciente: JWT no header `Authorization: Bearer <token>`, armazenado em localStorage

## Ao adicionar novos endpoints

1. Adicionar a chamada em `frontend/src/api.js` (psychApi ou patientApi)
2. Usar os nomes de campo snake_case conforme retornados pela API
3. Testar com `curl` antes de integrar no componente React
