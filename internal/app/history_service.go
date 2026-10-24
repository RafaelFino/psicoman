package app

import (
	"context"
	"sort"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
)

// HistoryService compõe, no servidor, a história completa de um paciente a
// partir das fontes existentes (sessões, notas, anamnese, débitos, pagamentos,
// arquivos, aprovação) e das remarcações registradas no audit log. A composição
// é derivada; os construtores puros de item vivem no domínio clinical e nunca
// revelam conteúdo clínico (SealedText). Portado de c3ab585:internal/service/history.go.
type HistoryService struct {
	patients PatientRepository
	sessions SessionRepository
	notes    ProntuarioRepository
	debts    DebtRepository
	payments PaymentRepository
	ged      GEDMetaRepository
	audit    *AuditService
}

// NewHistoryService cria o serviço de composição do histórico.
func NewHistoryService(
	patients PatientRepository,
	sessions SessionRepository,
	notes ProntuarioRepository,
	debts DebtRepository,
	payments PaymentRepository,
	ged GEDMetaRepository,
	audit *AuditService,
) *HistoryService {
	return &HistoryService{
		patients: patients, sessions: sessions, notes: notes,
		debts: debts, payments: payments, ged: ged, audit: audit,
	}
}

// PatientHistory devolve os eventos do paciente, ordenados do mais recente ao
// mais antigo. A leitura agrega conteúdo clínico (notas/anamnese), portanto
// gera um registro de auditoria AuditActionRead (metadata sem conteúdo clínico).
func (s *HistoryService) PatientHistory(ctx context.Context, patientID string) ([]clinical.HistoryItem, error) {
	p, err := s.patients.Get(ctx, patientID)
	if err != nil {
		return nil, err
	}
	var items []clinical.HistoryItem
	// Cadastro / primeiro contato.
	items = append(items, clinical.HistoryItem{
		Tipo: clinical.HistoryCadastro, Categoria: clinical.HistoryCatGeral, Data: p.CreatedAt,
		Titulo: "Cadastro do paciente", Resumo: "Início da relação no consultório.",
		Ref: clinical.HistoryRef{Kind: "patient", ID: p.ID},
	})
	// Aprovação (quando aprovado/rejeitado — estado atual do paciente).
	if p.ApprovalStatus == patient.PatientApproved || p.ApprovalStatus == patient.PatientRejected {
		titulo := "Acesso ao portal aprovado"
		if p.ApprovalStatus == patient.PatientRejected {
			titulo = "Acesso ao portal rejeitado"
		}
		items = append(items, clinical.HistoryItem{
			Tipo: clinical.HistoryAprovacao, Categoria: clinical.HistoryCatGeral, Data: p.UpdatedAt,
			Titulo: titulo, Resumo: "", Ref: clinical.HistoryRef{Kind: "patient", ID: p.ID},
		})
	}
	// Sessões + remarcações (do audit por sessão).
	sessions, err := s.sessions.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	for _, sess := range sessions {
		items = append(items, clinical.SessionItem(sess))
		if s.audit != nil {
			entries, aerr := s.audit.ListByEntity(ctx, "session", sess.ID)
			if aerr == nil {
				for _, e := range entries {
					if e.Action != domain.AuditActionSessionReschedule {
						continue
					}
					items = append(items, clinical.RescheduleItem(sess, e))
				}
			}
		}
	}
	// Notas (registro de sessão / nota livre) e anamnese.
	notes, err := s.notes.ListNotes(ctx, patientID)
	if err != nil {
		return nil, err
	}
	for _, n := range notes {
		items = append(items, clinical.NoteItem(n))
	}
	if anam, aerr := s.notes.GetAnamnesis(ctx, patientID); aerr == nil && anam != nil && !anam.Content.Empty() {
		data := anam.UpdatedAt
		if data.IsZero() {
			data = anam.CreatedAt
		}
		items = append(items, clinical.HistoryItem{
			Tipo: clinical.HistoryAnamnese, Categoria: clinical.HistoryCatClinico, Data: data,
			Titulo: "Anamnese", Resumo: "Ficha inicial da história clínica.",
			Ref: clinical.HistoryRef{Kind: "anamnesis", ID: p.ID},
		})
	}
	// Débitos + pagamentos.
	debts, err := s.debts.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	for _, d := range debts {
		items = append(items, clinical.DebtItem(d))
		if s.payments != nil {
			pays, perr := s.payments.ListByDebt(ctx, d.ID)
			if perr == nil {
				for _, pay := range pays {
					items = append(items, clinical.PaymentItem(d, pay))
				}
			}
		}
	}
	// Arquivos (GED).
	files, err := s.ged.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		items = append(items, clinical.FileItem(f))
	}
	// Ordenação desc por data (mais recente no topo).
	sort.SliceStable(items, func(i, j int) bool { return items[i].Data.After(items[j].Data) })
	// Auditoria de leitura clínica (R4.5): sem conteúdo em metadata.
	if s.audit != nil {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionRead, "patient_history", patientID,
			map[string]any{"patient_id": patientID, "itens": len(items)})
	}
	return items, nil
}
