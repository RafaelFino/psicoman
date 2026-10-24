package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// 0040_patient_auth cria a coleção de AUTH do paciente (patient_user) para o
// portal (Fase 9, D2/D3). Deliberadamente ADIADA da Fase 5: até aqui a
// identidade do portal era o SEAM X-Portal-Email; agora o cadastro/login usam a
// auth NATIVA do PocketBase (email + senha, tokens, auth-with-password).
//
// Opção A (decisão do Rafael): coleção de auth SEPARADA, vinculada 1:1 ao
// patient já existente (sem migração destrutiva do patient, que concentra
// relations clínicas, approval_status e índices). O patient_user guarda só a
// credencial; o patient segue como a entidade de negócio. A relation patient_id
// (UNIQUE) amarra o login ao paciente; o gate de aprovação continua lendo o
// approval_status do patient.
//
// A coleção auth usa o id nativo do PocketBase (não o ULID custom): o PB gere
// tokenKey/password/verified e a emissão/validação de tokens.
func init() {
	m.Register(upPatientAuth, downPatientAuth, "0040_patient_auth")
}

func upPatientAuth(app core.App) error {
	pu := core.NewAuthCollection("patient_user")

	// Relation 1:1 ao patient (UNIQUE patient_id garante um login por paciente).
	patientRel, err := relationTo(app, "patient_id", "patient", true)
	if err != nil {
		return err
	}
	pu.Fields.Add(patientRel)
	pu.AddIndex("ux_patient_user_patient", true, "patient_id", "")

	// Regras de acesso: nulas (só superuser via REST). O contrato do paciente é
	// o nosso /v1/portal/* (handlers próprios + gate), não a REST automática —
	// coerente com D13 (coleções trancadas; REST não é contrato público). O
	// login usa o endpoint nativo auth-with-password da própria coleção, que é
	// liberado independentemente das listRule/viewRule.
	if err := app.Save(pu); err != nil {
		return err
	}
	return nil
}

func downPatientAuth(app core.App) error {
	return dropCollections(app, "patient_user")
}
