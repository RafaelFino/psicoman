package pdf

import (
	"bytes"
	"testing"
)

func TestRenderProducesValidPDF(t *testing.T) {
	d := New("Cobrança")
	d.AddLines("Paciente: Maria", "Valor: R$ 150,00")
	out := d.Render()

	if !bytes.HasPrefix(out, []byte("%PDF-1.4")) {
		t.Error("PDF não começa com %PDF-1.4")
	}
	if !bytes.Contains(out, []byte("%%EOF")) {
		t.Error("PDF sem marcador EOF")
	}
	if !bytes.Contains(out, []byte("startxref")) {
		t.Error("PDF sem startxref")
	}
	if !bytes.Contains(out, []byte("/Type /Catalog")) {
		t.Error("PDF sem catálogo")
	}
}

func TestEscapeText(t *testing.T) {
	d := New("Título (com parênteses)")
	d.AddLine(`Barra \ e (parênteses)`)
	out := d.Render()
	// Não deve conter parênteses não escapados que quebrariam a sintaxe.
	if bytes.Contains(out, []byte("(Título (com")) {
		t.Error("parênteses não escapados no título")
	}
}
