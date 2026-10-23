// Package scheduling é o bounded context de agenda: pedido de agendamento,
// ciclo de vida da sessão (solicitada → agendada → confirmada → {realizada,
// cancelada, falta}), disponibilidade e local. Autoridade sobre as transições de
// estado da sessão, os metadados de cancelamento (autor/motivo) e a remarcação
// rastreável. Ver MER canônico (D4b).
package scheduling
