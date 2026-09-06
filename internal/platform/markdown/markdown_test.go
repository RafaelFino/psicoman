package markdown

import (
	"strings"
	"testing"
)

func TestHeadings(t *testing.T) {
	got := ToHTML("# Título\n## Subtítulo")
	if !strings.Contains(got, "<h1>Título</h1>") {
		t.Errorf("h1 ausente: %q", got)
	}
	if !strings.Contains(got, "<h2>Subtítulo</h2>") {
		t.Errorf("h2 ausente: %q", got)
	}
}

func TestEmphasis(t *testing.T) {
	got := ToHTML("Texto com **negrito** e *itálico*.")
	if !strings.Contains(got, "<strong>negrito</strong>") {
		t.Errorf("negrito ausente: %q", got)
	}
	if !strings.Contains(got, "<em>itálico</em>") {
		t.Errorf("itálico ausente: %q", got)
	}
}

func TestList(t *testing.T) {
	got := ToHTML("- item 1\n- item 2")
	if !strings.Contains(got, "<ul>") || strings.Count(got, "<li>") != 2 {
		t.Errorf("lista malformada: %q", got)
	}
}

func TestParagraph(t *testing.T) {
	got := ToHTML("Primeiro parágrafo.\n\nSegundo parágrafo.")
	if strings.Count(got, "<p>") != 2 {
		t.Errorf("esperava 2 parágrafos: %q", got)
	}
}

func TestEscapesHTML(t *testing.T) {
	got := ToHTML("Texto com <script>alert(1)</script>")
	if strings.Contains(got, "<script>") {
		t.Errorf("HTML não escapado: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("escape ausente: %q", got)
	}
}
