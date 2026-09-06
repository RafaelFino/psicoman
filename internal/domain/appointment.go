package domain

import (
	"errors"
	"time"
)

// Status de pedido de agendamento.
const (
	ApptPendente   = "pendente"
	ApptConfirmado = "confirmado"
	ApptRecusado   = "recusado"
)

// AppointmentRequest é o pedido de agendamento do paciente (registro interno;
// não toca o Google até a confirmação do terapeuta) — requirements §3.3.
type AppointmentRequest struct {
	ID         string
	PatientID  string
	LocationID string
	SlotStart  time.Time
	SlotEnd    time.Time
	Status     string
	Note       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Validate verifica as invariantes do pedido.
func (a *AppointmentRequest) Validate() error {
	if a.PatientID == "" {
		return errors.New("O pedido precisa de um paciente.")
	}
	if a.SlotStart.IsZero() || a.SlotEnd.IsZero() {
		return errors.New("Informe o horário desejado.")
	}
	if !a.SlotEnd.After(a.SlotStart) {
		return errors.New("O horário final deve ser posterior ao inicial.")
	}
	return nil
}
