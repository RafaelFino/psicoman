package api

import (
	"errors"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/domain/patient"
)

// Coleções cobertas pelo hook de reforço de aprovação (defesa em profundidade,
// Fase 5 / 5.2). Escopo reduzido deliberadamente a appointment_request (design
// §4.3): é a única escrita que o paciente origina pelo portal e carrega
// patient_id direto. payment/ged_file/patient/colunas clínicas ficam FORA (não
// alcançáveis pelo portal; cobri-las geraria falso positivo em escrita legítima
// do terapeuta/upsert/soft-delete — ver docs/decisions.md).
const approvalGuardedCollection = "appointment_request"

// registerApprovalHooks liga o hook model-level que barra escrita (create e
// update) de appointment_request cujo paciente dono não esteja aprovado.
//
// É defesa em profundidade: o controle primário do ator paciente é o middleware
// do grupo /v1/portal/* (portal.RequireApproved); este hook cobre QUALQUER
// caminho de escrita, inclusive o adapter platform/pb (app.Save), porque usa
// OnRecordCreate/OnRecordUpdate (model-level) e não *Request (só caminho REST,
// que para o portal está trancado por regra nil).
//
// Invariante (regra de negócio, cego ao ator): nenhum appointment_request pode
// ser criado/alterado para paciente não-aprovado, venha de quem vier — agendar
// exige paciente aprovado (ver docs/decisions.md). Vive em internal/api (pode
// importar pocketbase/core); NUNCA em domain/app (archtest).
func registerApprovalHooks(app core.App) {
	guard := approvalWriteGuard(app)
	app.OnRecordCreate(approvalGuardedCollection).BindFunc(guard)
	app.OnRecordUpdate(approvalGuardedCollection).BindFunc(guard)
}

// approvalWriteGuard devolve a closure do hook: resolve o paciente dono por
// patient_id e barra a escrita (retorna erro sem e.Next()) se não estiver
// aprovado. patient_id vazio → segue (salvaguarda; appointment_request.patient_id
// é required, não deve disparar).
func approvalWriteGuard(app core.App) func(e *core.RecordEvent) error {
	return func(e *core.RecordEvent) error {
		pid := e.Record.GetString("patient_id")
		if pid == "" {
			return e.Next() // sem paciente dono não há estado de aprovação a avaliar
		}
		owner, err := e.App.FindRecordById("patient", pid)
		if err != nil {
			app.Logger().Warn("agendamento barrado: paciente dono não encontrado",
				"colecao", approvalGuardedCollection,
				"patient_id", pid,
			)
			return errors.New("Cadastro do paciente não encontrado para o registro.")
		}
		if owner.GetString("approval_status") != patient.PatientApproved {
			app.Logger().Warn("agendamento barrado: paciente não aprovado",
				"colecao", approvalGuardedCollection,
				"patient_id", pid,
			)
			return errors.New("Agendamento exige paciente aprovado.")
		}
		return e.Next()
	}
}
