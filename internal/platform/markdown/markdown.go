// Package markdown converte um subconjunto de Markdown em HTML, em Go puro.
//
// Suporta: cabeçalhos (# a ######), listas não ordenadas (- / *), ênfase
// (**negrito** e *itálico*), código inline (`code`), blocos de código cercados
// por ``` , links [texto](url) (apenas http/https/mailto), parágrafos e quebras.
// Escapa HTML por segurança (anti-XSS). Este subconjunto é espelhado no cliente
// (internal/web/static/api.js → Md.toHtml) para que o preview no admin bata com
// o HTML efetivamente enviado ao paciente (requirements §3.6).
package markdown

import (
	"html"
	"regexp"
	"strings"
)

var (
	codeRe   = regexp.MustCompile("`([^`]+)`")
	boldRe   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRe = regexp.MustCompile(`\*([^*]+)\*`)
	// Links [texto](url): só http/https/mailto para evitar esquemas perigosos
	// como javascript:. O texto e a URL já vêm escapados quando isto roda.
	linkRe = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\s)]+|mailto:[^\s)]+)\)`)
)

// ToHTML converte o Markdown em HTML.
func ToHTML(md string) string {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out strings.Builder
	inList := false
	inCode := false
	var para []string
	var code []string

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
	flushCode := func() {
		out.WriteString(`<pre class="code">`)
		out.WriteString(html.EscapeString(strings.Join(code, "\n")))
		out.WriteString("</pre>\n")
		code = nil
		inCode = false
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " ")
		trimmed := strings.TrimSpace(line)

		// Bloco de código cercado por ``` (abre/fecha).
		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				flushCode()
			} else {
				flushPara()
				closeList()
				inCode = true
			}
			continue
		}
		if inCode {
			code = append(code, raw)
			continue
		}

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
	if inCode {
		flushCode()
	}
	flushPara()
	closeList()
	return strings.TrimRight(out.String(), "\n")
}

// inline aplica código, ênfase e links após escapar HTML.
//
// A ordem importa: o código inline é extraído primeiro (em placeholders) para
// que seu conteúdo não sofra ênfase/link; depois aplicam-se negrito, itálico e
// links, e por fim o código é reinserido já escapado.
func inline(s string) string {
	s = html.EscapeString(s)

	// Protege trechos de código inline com placeholders.
	var codes []string
	s = codeRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[1 : len(m)-1] // remove as crases
		codes = append(codes, inner)
		return "\x00CODE" + itoa(len(codes)-1) + "\x00"
	})

	s = boldRe.ReplaceAllString(s, "<strong>$1</strong>")
	s = italicRe.ReplaceAllString(s, "<em>$1</em>")
	s = linkRe.ReplaceAllString(s, `<a href="$2" target="_blank" rel="noopener">$1</a>`)

	// Reinsere o código inline (já escapado por EscapeString acima).
	for i, c := range codes {
		s = strings.Replace(s, "\x00CODE"+itoa(i)+"\x00", "<code>"+c+"</code>", 1)
	}
	return s
}

// itoa converte um inteiro pequeno e não-negativo em string sem alocar via fmt.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
