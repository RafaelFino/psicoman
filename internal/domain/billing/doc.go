// Package billing é o bounded context financeiro: contrato, plano, débito e
// pagamento. Autoridade sobre a geração idempotente de débito (idempotency_key),
// o fechamento de ciclo de planos fechados por dia de vencimento e o cálculo de
// status de pagamento. Dinheiro sempre em int64 centavos. Ver MER canônico (D4b).
package billing
