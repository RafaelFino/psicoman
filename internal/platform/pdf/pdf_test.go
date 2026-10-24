package pdf

import (
	"bytes"
	"testing"
)

// TestRender_EstruturaValida: o PDF gerado tem header 1.4, xref/trailer/EOF e
// os 6 objetos esperados (catalog, pages, page, content, 2 fontes).
func TestRender_EstruturaValida(t *testing.T) {
	doc := New("Cobrança de Atendimento")
	doc.AddLine("Paciente: Fulano de Tal")
	doc.AddLine("Valor: R$ 150,00")

	out := doc.Render()
	if !bytes.HasPrefix(out, []byte("%PDF-1.4")) {
		t.Fatalf("header PDF ausente: %q", out[:min(16, len(out))])
	}
	for _, marker := range []string{"xref", "trailer", "startxref", "%%EOF"} {
		if !bytes.Contains(out, []byte(marker)) {
			t.Errorf("marcador %q ausente no PDF", marker)
		}
	}
	// Os 6 objetos: "1 0 obj".."6 0 obj".
	for _, obj := range []string{"1 0 obj", "2 0 obj", "3 0 obj", "4 0 obj", "5 0 obj", "6 0 obj"} {
		if !bytes.Contains(out, []byte(obj)) {
			t.Errorf("objeto %q ausente", obj)
		}
	}
	if !bytes.Contains(out, []byte("Cobrança de Atendimento")) {
		t.Error("título ausente no content stream")
	}
	if !bytes.Contains(out, []byte("Paciente: Fulano de Tal")) {
		t.Error("linha do corpo ausente no content stream")
	}
}

// TestRender_EscapaParenteses: parênteses e barra invertida (sintaxe de string
// literal PDF) são escapados para não corromper o stream.
func TestRender_EscapaParenteses(t *testing.T) {
	doc := New("T")
	doc.AddLine("valor (aprox) \\ teste")
	out := doc.Render()
	if !bytes.Contains(out, []byte(`\(aprox\)`)) {
		t.Error("parênteses não escapados")
	}
	if !bytes.Contains(out, []byte(`\\`)) {
		t.Error("barra invertida não escapada")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
