package pb

import (
	"context"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
)

// queueStats implementa app.QueueStats com CountRecords (um COUNT no SQLite,
// sem carregar registros). Alimenta os gauges de fila da observabilidade (R8.4).
type queueStats struct {
	app core.App
}

var _ app.QueueStats = (*queueStats)(nil)

// NewQueueStats cria o leitor das filas de trabalho.
func NewQueueStats(a core.App) app.QueueStats { return &queueStats{app: a} }

// PendingPatients conta os pacientes com cadastro pendente de aprovação.
func (q *queueStats) PendingPatients(_ context.Context) (int, error) {
	n, err := q.app.CountRecords("patient", dbx.HashExp{"approval_status": "pendente"})
	return int(n), mapError(err)
}

// PendingAppointments conta os pedidos de agendamento pendentes.
func (q *queueStats) PendingAppointments(_ context.Context) (int, error) {
	n, err := q.app.CountRecords("appointment_request", dbx.HashExp{"status": "pendente"})
	return int(n), mapError(err)
}
