package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// consoleHandler é um slog.Handler que escreve linhas legíveis para humanos,
// pensado para o stdout durante o desenvolvimento (make run-local). O arquivo
// de log continua em JSON (via slog.JSONHandler), preservando a observabilidade
// estruturada; aqui priorizamos clareza no terminal.
//
// Formato geral:
//
//	15:04:05 INFO  [admin] mensagem  chave=valor chave2=valor2
//
// Requisições HTTP (msg="requisição") ganham um formato dedicado, mais direto:
//
//	15:04:05 INFO  [admin] GET /v1/admin/patients → 200 (3ms)
type consoleHandler struct {
	mu    *sync.Mutex
	w     io.Writer
	level slog.Level
	color bool
	// attrs acumulados via WithAttrs (ex: component).
	attrs []slog.Attr
	group string
}

func newConsoleHandler(w io.Writer, level slog.Level, color bool) *consoleHandler {
	return &consoleHandler{mu: &sync.Mutex{}, w: w, level: level, color: color}
}

func (h *consoleHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := *h
	nh.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &nh
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	nh := *h
	nh.group = name
	return &nh
}

// ANSI colors (só quando color=true).
const (
	cReset  = "\033[0m"
	cDim    = "\033[2m"
	cBold   = "\033[1m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cBlue   = "\033[34m"
	cCyan   = "\033[36m"
	cGray   = "\033[90m"
)

func (h *consoleHandler) paint(c, s string) string {
	if !h.color {
		return s
	}
	return c + s + cReset
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	// Coleta atributos (herdados + do record) num mapa para lookup e ordenação.
	fields := make(map[string]slog.Value, len(h.attrs)+r.NumAttrs())
	order := make([]string, 0, len(h.attrs)+r.NumAttrs())
	add := func(a slog.Attr) {
		if a.Key == "" {
			return
		}
		if _, seen := fields[a.Key]; !seen {
			order = append(order, a.Key)
		}
		fields[a.Key] = a.Value
	}
	for _, a := range h.attrs {
		add(a)
	}
	r.Attrs(func(a slog.Attr) bool { add(a); return true })

	component := ""
	if v, ok := fields["component"]; ok {
		component = v.String()
	}

	var b strings.Builder
	// Timestamp curto (HH:MM:SS) — a data completa fica no arquivo JSON.
	ts := r.Time.Format("15:04:05")
	b.WriteString(h.paint(cGray, ts))
	b.WriteByte(' ')
	b.WriteString(h.levelTag(r.Level))
	b.WriteByte(' ')
	if component != "" {
		b.WriteString(h.paint(cCyan, "["+component+"]"))
		b.WriteByte(' ')
	}

	// Caso especial: linha de requisição HTTP, formatada de forma direta.
	if r.Message == "requisição" {
		h.writeRequestLine(&b, fields)
	} else {
		b.WriteString(r.Message)
		h.writeFields(&b, order, fields, requestKeys)
	}

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

// requestKeys são as chaves já consumidas pela linha de requisição; não devem
// ser repetidas como key=value.
var requestKeys = map[string]bool{
	"component": true, "method": true, "path": true,
	"status": true, "duration_ms": true, "request_id": true,
}

// nonRequestSkip são chaves que não repetimos em mensagens comuns.
var nonRequestSkip = map[string]bool{"component": true}

func (h *consoleHandler) writeRequestLine(b *strings.Builder, f map[string]slog.Value) {
	method := valStr(f, "method")
	path := valStr(f, "path")
	status := 0
	if v, ok := f["status"]; ok {
		status = int(v.Int64())
	}
	dur := valStr(f, "duration_ms")

	b.WriteString(h.paint(cBold, method))
	b.WriteByte(' ')
	b.WriteString(path)
	b.WriteString(" → ")
	b.WriteString(h.statusTag(status))
	if dur != "" {
		b.WriteString(" ")
		b.WriteString(h.paint(cDim, "("+dur+"ms)"))
	}
}

func (h *consoleHandler) writeFields(b *strings.Builder, order []string, f map[string]slog.Value, skip map[string]bool) {
	keys := make([]string, 0, len(order))
	for _, k := range order {
		if skip[k] || nonRequestSkip[k] {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString("  ")
		b.WriteString(h.paint(cDim, k+"="))
		b.WriteString(quoteIfNeeded(f[k].String()))
	}
}

func (h *consoleHandler) levelTag(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return h.paint(cRed, "ERRO ")
	case l >= slog.LevelWarn:
		return h.paint(cYellow, "AVISO")
	case l >= slog.LevelInfo:
		return h.paint(cGreen, "INFO ")
	default:
		return h.paint(cBlue, "DEBUG")
	}
}

func (h *consoleHandler) statusTag(status int) string {
	s := strconv.Itoa(status)
	switch {
	case status >= 500:
		return h.paint(cRed, s)
	case status >= 400:
		return h.paint(cYellow, s)
	case status >= 200 && status < 400:
		return h.paint(cGreen, s)
	default:
		return s
	}
}

func valStr(f map[string]slog.Value, k string) string {
	if v, ok := f[k]; ok {
		return v.String()
	}
	return ""
}

func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\"") {
		return strconv.Quote(s)
	}
	return s
}

var _ = fmt.Sprintf

// multiHandler despacha cada record para todos os handlers subjacentes,
// permitindo formatos distintos por destino (console de texto + arquivo JSON).
type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(hs ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: hs}
}

func (m *multiHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: next}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: next}
}
