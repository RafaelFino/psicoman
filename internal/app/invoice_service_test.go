package app

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func seedDebt(repo *fakeDebtRepo, d *billing.Debt) {
	repo.byID[d.ID] = d
}

func newInvoiceSvc() (*InvoiceService, *fakeDebtRepo, *fakeGEDMetaRepo, *fakeGEDBlob) {
	debts := newDebtRepo()
	patients := newPatientRepo()
	seedPatient(patients, "p1")
	meta := newGEDMetaRepo()
	blob := newGEDBlob()
	svc := NewInvoiceService(debts, patients, meta, blob, nil)
	svc.clock = clock.Fixed{T: time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location())}
	return svc, debts, meta, blob
}

// TestGeneratePDF_PersisteNoGEDEVincula: gerar a cobrança produz um PDF no GED
// (bytes + metadado) e vincula o arquivo ao débito (D9b / R9.3).
func TestGeneratePDF_PersisteNoGEDEVincula(t *testing.T) {
	ctx := context.Background()
	svc, debts, meta, blob := newInvoiceSvc()
	seedDebt(debts, &billing.Debt{
		ID: "d1", PatientID: "p1", Amount: 15000, Status: billing.DebtAberto,
		DueDate: time.Date(2026, 3, 20, 0, 0, 0, 0, clock.Location()),
	})

	f, err := svc.GeneratePDF(ctx, "d1")
	if err != nil {
		t.Fatal(err)
	}
	if f.MIME != "application/pdf" || f.Size == 0 || f.SHA256 == "" {
		t.Fatalf("metadado incompleto: %+v", f)
	}
	if f.DebtID != "d1" || f.PatientID != "p1" {
		t.Fatalf("metadado não vinculado ao débito/paciente: %+v", f)
	}
	// Bytes gravados no blob store sob o relPath.
	raw, err := blob.Get(ctx, f.RelPath)
	if err != nil {
		t.Fatalf("bytes não gravados no GED: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte("%PDF-1.4")) {
		t.Fatalf("conteúdo gravado não é um PDF válido")
	}
	// Metadado persistido e débito vinculado.
	if len(meta.all) != 1 {
		t.Fatalf("esperava 1 metadado no GED, veio %d", len(meta.all))
	}
	if debts.byID["d1"].PDFFileID != f.ID {
		t.Fatalf("débito não vinculado ao PDF: %q", debts.byID["d1"].PDFFileID)
	}
}

// TestGeneratePDF_Idempotente_DedupPorHash: regerar a cobrança do mesmo débito
// reaproveita o arquivo existente (dedup por SHA-256), sem regravar bytes nem
// duplicar metadado.
func TestGeneratePDF_Idempotente_DedupPorHash(t *testing.T) {
	ctx := context.Background()
	svc, debts, meta, blob := newInvoiceSvc()
	seedDebt(debts, &billing.Debt{ID: "d1", PatientID: "p1", Amount: 15000, Status: billing.DebtAberto})

	f1, err := svc.GeneratePDF(ctx, "d1")
	if err != nil {
		t.Fatal(err)
	}
	f2, err := svc.GeneratePDF(ctx, "d1")
	if err != nil {
		t.Fatal(err)
	}
	if f1.ID != f2.ID {
		t.Fatalf("dedup falhou: ids diferentes %q != %q", f1.ID, f2.ID)
	}
	if len(meta.all) != 1 {
		t.Fatalf("dedup deveria manter 1 metadado, veio %d", len(meta.all))
	}
	if blob.puts != 1 {
		t.Fatalf("dedup não deveria regravar bytes: puts=%d", blob.puts)
	}
}

// TestGetPDF_GeraOnDemand: GetPDF de um débito sem PDF gera on-demand e devolve
// os bytes do PDF.
func TestGetPDF_GeraOnDemand(t *testing.T) {
	ctx := context.Background()
	svc, debts, _, _ := newInvoiceSvc()
	seedDebt(debts, &billing.Debt{ID: "d1", PatientID: "p1", Amount: 15000, Status: billing.DebtAberto})

	data, err := svc.GetPDF(ctx, "d1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-1.4")) {
		t.Fatal("GetPDF não devolveu um PDF válido")
	}
	if debts.byID["d1"].PDFFileID == "" {
		t.Fatal("GetPDF on-demand deveria vincular o PDF ao débito")
	}
}

// TestGeneratePDF_AuditaSemClinico: a auditoria do débito (quando habilitada)
// registra só ids, sem conteúdo clínico nem valor (minimização).
func TestGeneratePDF_AuditaSemClinico(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	patients := newPatientRepo()
	seedPatient(patients, "p1")
	audit, arepo := newAudit()
	svc := NewInvoiceService(debts, patients, newGEDMetaRepo(), newGEDBlob(), audit)
	svc.clock = clock.Fixed{T: time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location())}
	seedDebt(debts, &billing.Debt{ID: "d1", PatientID: "p1", Amount: 15000, Status: billing.DebtAberto})

	if _, err := svc.GeneratePDF(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	acts := arepo.actionsFor("debt", "d1")
	if len(acts) != 1 || acts[0] != domain.AuditActionDebtGenerate {
		t.Fatalf("esperava 1 auditoria debito_gerar, veio %v", acts)
	}
	// Minimização: metadata só com ged_file_id (sem valor/clínico).
	entry := arepo.entries[len(arepo.entries)-1]
	if _, ok := entry.Metadata["ged_file_id"]; !ok {
		t.Fatalf("auditoria deveria ter ged_file_id: %+v", entry.Metadata)
	}
	for k := range entry.Metadata {
		if k != "ged_file_id" {
			t.Fatalf("auditoria não deveria carregar %q (minimização)", k)
		}
	}
}
