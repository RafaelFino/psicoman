package pb

import (
	"context"
	"fmt"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
)

// portalAuthRepo implementa app.PortalAuth sobre a coleção de auth patient_user
// (Fase 9). Cria a credencial do paciente (email + senha, auth nativa do
// PocketBase) e resolve o patient_id a partir do id do auth record da sessão.
// É o ÚNICO ponto que toca a coleção de auth; o domínio/app não conhecem o PB.
type portalAuthRepo struct{ app core.App }

var _ app.PortalAuth = (*portalAuthRepo)(nil)

// NewPortalAuthRepository cria o repositório de auth do portal.
func NewPortalAuthRepository(a core.App) app.PortalAuth { return &portalAuthRepo{app: a} }

// CreateLogin cria a credencial vinculada ao paciente. O PocketBase valida o
// email (único na coleção auth) e faz o hash da senha (SetPassword). A relation
// patient_id é UNIQUE: um login por paciente.
func (r *portalAuthRepo) CreateLogin(ctx context.Context, patientID, email, password string) error {
	col, err := r.app.FindCollectionByNameOrId("patient_user")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	rec.SetEmail(email)
	rec.SetPassword(password)
	rec.Set("patient_id", patientID)
	// email verificado: o fluxo de verificação por email está fora do escopo
	// desta onda (o paciente é aprovado pelo terapeuta — gate de aprovação).
	rec.SetVerified(true)
	if err := r.app.SaveWithContext(ctx, rec); err != nil {
		return mapError(err)
	}
	return nil
}

// PatientIDByAuthID resolve o patient_id do login a partir do id do auth record
// (e.Auth.Id da sessão do PocketBase). ErrNotFound quando não há login com esse
// id ou ele não está vinculado a um paciente.
func (r *portalAuthRepo) PatientIDByAuthID(ctx context.Context, authRecordID string) (string, error) {
	rec, err := getRecordByID(r.app, "patient_user", authRecordID)
	if err != nil {
		return "", err
	}
	pid := rec.GetString("patient_id")
	if pid == "" {
		return "", fmt.Errorf("login sem paciente vinculado: %w", app.ErrNotFound)
	}
	return pid, nil
}
