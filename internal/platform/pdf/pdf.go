// Package pdf gera PDFs simples de uma página em Go puro, sem dependência
// externa. Suficiente para o lastro de cobrança do MVP (texto em linhas).
//
// Produz um PDF 1.4 válido com a fonte padrão Helvetica. Não suporta imagens
// nem múltiplas páginas — escopo deliberadamente mínimo.
package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

// Document acumula linhas de texto a renderizar.
type Document struct {
	title string
	lines []string
}

// New cria um documento com um título.
func New(title string) *Document {
	return &Document{title: title}
}

// AddLine adiciona uma linha de texto ao corpo.
func (d *Document) AddLine(line string) {
	d.lines = append(d.lines, line)
}

// AddLines adiciona várias linhas.
func (d *Document) AddLines(lines ...string) {
	d.lines = append(d.lines, lines...)
}

// escapeText escapa caracteres especiais de string literal PDF.
func escapeText(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`, "\r", " ", "\n", " ")
	return r.Replace(s)
}

// Render devolve os bytes do PDF.
func (d *Document) Render() []byte {
	// Monta o content stream: título maior, depois as linhas do corpo.
	var content bytes.Buffer
	content.WriteString("BT\n")
	// Título (Helvetica-Bold 16) no topo.
	content.WriteString("/F2 16 Tf\n")
	content.WriteString("72 770 Td\n")
	content.WriteString(fmt.Sprintf("(%s) Tj\n", escapeText(d.title)))
	// Corpo (Helvetica 11), avançando 18pt por linha.
	content.WriteString("/F1 11 Tf\n")
	content.WriteString("0 -30 Td\n")
	content.WriteString("18 TL\n")
	for _, line := range d.lines {
		content.WriteString(fmt.Sprintf("(%s) Tj\n", escapeText(line)))
		content.WriteString("T*\n")
	}
	content.WriteString("ET\n")

	contentBytes := content.Bytes()

	// Objetos do PDF.
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] " +
			"/Resources << /Font << /F1 5 0 R /F2 6 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contentBytes), contentBytes),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>",
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = buf.Len()
		buf.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", i+1, obj))
	}

	xrefStart := buf.Len()
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n", len(objects)+1))
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	buf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\n", len(objects)+1))
	buf.WriteString(fmt.Sprintf("startxref\n%d\n%%%%EOF\n", xrefStart))

	return buf.Bytes()
}
