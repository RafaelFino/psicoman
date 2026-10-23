package costs

import (
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
)

func TestCostCategoryValidate(t *testing.T) {
	if err := (&CostCategory{Kind: CostKindLocal, Name: "Aluguel"}).Validate(); err != nil {
		t.Fatalf("categoria válida: %v", err)
	}
	if err := (&CostCategory{Kind: "x", Name: "n"}).Validate(); err == nil {
		t.Fatal("kind inválido deveria falhar")
	}
	if err := (&CostCategory{Kind: CostKindLocal, Name: " "}).Validate(); err == nil {
		t.Fatal("nome vazio deveria falhar")
	}
}

func TestCostItemValidate(t *testing.T) {
	ok := &CostItem{Name: "Sala", Period: scheduling.PeriodMensal, Amount: 50000}
	if err := ok.Validate(); err != nil {
		t.Fatalf("item válido: %v", err)
	}
	if err := (&CostItem{Name: " ", Period: scheduling.PeriodMensal}).Validate(); err == nil {
		t.Fatal("nome vazio deveria falhar")
	}
	if err := (&CostItem{Name: "x", Period: "x"}).Validate(); err == nil {
		t.Fatal("período inválido deveria falhar")
	}
	if err := (&CostItem{Name: "x", Period: scheduling.PeriodMensal, Amount: -1}).Validate(); err == nil {
		t.Fatal("amount negativo deveria falhar")
	}
}
