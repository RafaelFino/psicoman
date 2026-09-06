// Package markdown converte um subconjunto de Markdown em HTML, em Go puro.
//
// Suporta: cabeçalhos (# a ######), listas não ordenadas (- / *), ênfase
// (**negrito** e *itálico*), parágrafos e quebras. Escapa HTML por segurança.
// Escopo deliberadamente mínimo para os templates do MVP (requirements §3.6).
package markdown

import (
	"html"
	"regexp"
	"strings"
)

var (
	boldRe   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRe = regexp.MustCompile(`\*([^*]+)\*`)
)

// ToHTML converte o Markdown em HTML.
func ToHTML(md string) string {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out strings.Builder
	inList := false
	var para []string

	flushPara := func() {
		if len(para) > 0 {
			out.WriteString("<p>")
			out.WriteString(inline(strings.Join(para, " ")))
			out.WriteString("</p>\n")
			para = nil
		}
	}
	closeList := func() {
		if inList {
			out.WriteString("</ul>\n")
			inList = false
		}
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " ")
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "":
			flushPara()
			closeList()

		case strings.HasPrefix(trimmed, "#"):
			flushPara()
			closeList()
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level > 6 {
				level = 6
			}
			text := strings.TrimSpace(trimmed[level:])
			out.WriteString("<h")
			out.WriteByte(byte('0' + level))
			out.WriteString(">")
			out.WriteString(inline(text))
			out.WriteString("</h")
			out.WriteByte(byte('0' + level))
			out.WriteString(">\n")

		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			flushPara()
			if !inList {
				out.WriteString("<ul>\n")
				inList = true
			}
			item := strings.TrimSpace(trimmed[2:])
			out.WriteString("<li>")
			out.WriteString(inline(item))
			out.WriteString("</li>\n")

		default:
			closeList()
			para = append(para, trimmed)
		}
	}
	flushPara()
	closeList()
	return strings.TrimRight(out.String(), "\n")
}

// inline aplica ênfase (negrito/itálico) após escapar HTML.
func inline(s string) string {
	s = html.EscapeString(s)
	s = boldRe.ReplaceAllString(s, "<strong>$1</strong>")
	s = italicRe.ReplaceAllString(s, "<em>$1</em>")
	return s
}
