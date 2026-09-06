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

func TestInlineCode(t *testing.T) {
	got := ToHTML("Use `go build` para compilar.")
	if !strings.Contains(got, "<code>go build</code>") {
		t.Errorf("código inline ausente: %q", got)
	}
}

func TestCodeFence(t *testing.T) {
	got := ToHTML("```\nlinha 1\nlinha 2\n```")
	if !strings.Contains(got, `<pre class="code">`) {
		t.Errorf("bloco de código ausente: %q", got)
	}
	if !strings.Contains(got, "linha 1\nlinha 2") {
		t.Errorf("conteúdo do bloco ausente: %q", got)
	}
}

func TestCodeFenceEscapesHTML(t *testing.T) {
	got := ToHTML("```\n<script>alert(1)</script>\n```")
	if strings.Contains(got, "<script>") {
		t.Errorf("HTML dentro do bloco não escapado: %q", got)
	}
}

func TestLink(t *testing.T) {
	got := ToHTML("Veja [o site](https://example.com).")
	if !strings.Contains(got, `<a href="https://example.com" target="_blank" rel="noopener">o site</a>`) {
		t.Errorf("link ausente/errado: %q", got)
	}
}

func TestLinkRejectsJavascript(t *testing.T) {
	got := ToHTML("[x](javascript:alert(1))")
	if strings.Contains(got, "<a ") {
		t.Errorf("esquema javascript: não deveria virar link: %q", got)
	}
}

func TestInlineCodeNotEmphasized(t *testing.T) {
	// Asteriscos dentro de código não viram ênfase.
	got := ToHTML("`a * b * c`")
	if strings.Contains(got, "<em>") {
		t.Errorf("código inline não deveria receber ênfase: %q", got)
	}
}
