package clinical

import (
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain"
)

func TestNoteValidate(t *testing.T) {
	ok := &Note{PatientID: "p1", Content: domain.NewSealedText("conteúdo clínico")}
	if err := ok.Validate(); err != nil {
		t.Fatalf("nota válida: %v", err)
	}
	if err := (&Note{Content: domain.NewSealedText("x")}).Validate(); err == nil {
		t.Fatal("nota sem paciente deveria falhar")
	}
	if err := (&Note{PatientID: "p1"}).Validate(); err == nil {
		t.Fatal("nota com conteúdo vazio deveria falhar")
	}
}

func TestNoteIsFree(t *testing.T) {
	if !(&Note{}).IsFree() {
		t.Error("nota sem sessão é livre")
	}
	if (&Note{SessionID: "s1"}).IsFree() {
		t.Error("nota com sessão não é livre")
	}
}

func TestAnamnesisValidate(t *testing.T) {
	ok := &Anamnesis{PatientID: "p1", Content: domain.NewSealedText("história")}
	if err := ok.Validate(); err != nil {
		t.Fatalf("anamnese válida: %v", err)
	}
	if err := (&Anamnesis{PatientID: "p1"}).Validate(); err == nil {
		t.Fatal("anamnese vazia deveria falhar")
	}
}

func TestAISummaryValidate(t *testing.T) {
	ok := &AISummary{SessionID: "s1", Resumo: domain.NewSealedText("resumo"), Status: AISummarySugerido}
	if err := ok.Validate(); err != nil {
		t.Fatalf("resumo válido: %v", err)
	}
	if err := (&AISummary{SessionID: "s1", Resumo: domain.NewSealedText("r"), Status: "x"}).Validate(); err == nil {
		t.Fatal("status inválido deveria falhar")
	}
}

func TestTagIndexValidate(t *testing.T) {
	ok := &TagIndex{PatientID: "p1", Ano: 2026, Tag: "ansiedade", SessionIDs: []string{"s1"}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("tag válida: %v", err)
	}
	// Tag fica em claro.
	if ok.Tag != "ansiedade" {
		t.Error("tag deveria permanecer em claro")
	}
	if err := (&TagIndex{Ano: 2026, Tag: "x"}).Validate(); err == nil {
		t.Fatal("tag sem paciente deveria falhar")
	}
	if err := (&TagIndex{PatientID: "p1", Tag: "x"}).Validate(); err == nil {
		t.Fatal("ano inválido deveria falhar")
	}
	if err := (&TagIndex{PatientID: "p1", Ano: 2026, Tag: " "}).Validate(); err == nil {
		t.Fatal("tag vazia deveria falhar")
	}
}
