// Package logger fornece logging estruturado (JSON) com níveis e rotação diária.
//
// Requisitos: níveis debug/warn/info/error e rotação diária por data
// (docs/requirements.md §4.3, docs/architecture.md §6). Structured logging em
// JSON com request-id quando disponível.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// Logger encapsula um slog.Logger com rotação diária opcional em arquivo.
type Logger struct {
	*slog.Logger
	rot *dailyRotator
}

// Options configura a criação do logger.
type Options struct {
	Level     string // debug|info|warn|error
	Dir       string // diretório de logs; vazio => só stdout
	Component string // nome do binário/componente (admin|portal)
}

// New cria um Logger conforme as opções. Escreve sempre em stdout e, se Dir for
// informado, também em arquivo com rotação diária.
func New(opts Options) (*Logger, error) {
	level := parseLevel(opts.Level)

	var writers []io.Writer
	writers = append(writers, os.Stdout)

	var rot *dailyRotator
	if opts.Dir != "" {
		if err := os.MkdirAll(opts.Dir, 0o750); err != nil {
			return nil, fmt.Errorf("criando diretório de log %q: %w", opts.Dir, err)
		}
		rot = &dailyRotator{dir: opts.Dir, component: opts.Component}
		writers = append(writers, rot)
	}

	handler := slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level})
	l := slog.New(handler)
	if opts.Component != "" {
		l = l.With("component", opts.Component)
	}
	return &Logger{Logger: l, rot: rot}, nil
}

// Close libera o arquivo de log corrente, se houver.
func (l *Logger) Close() error {
	if l.rot != nil {
		return l.rot.Close()
	}
	return nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// dailyRotator escreve num arquivo cujo nome carrega a data corrente,
// abrindo um novo arquivo quando o dia muda.
type dailyRotator struct {
	dir       string
	component string

	mu      sync.Mutex
	current *os.File
	day     string
}

func (r *dailyRotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	today := clock.Now().Format("2006-01-02")
	if r.current == nil || r.day != today {
		if err := r.rotate(today); err != nil {
			return 0, err
		}
	}
	return r.current.Write(p)
}

func (r *dailyRotator) rotate(day string) error {
	if r.current != nil {
		_ = r.current.Close()
	}
	name := day + ".log"
	if r.component != "" {
		name = r.component + "-" + name
	}
	f, err := os.OpenFile(filepath.Join(r.dir, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	r.current = f
	r.day = day
	return nil
}

func (r *dailyRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current != nil {
		err := r.current.Close()
		r.current = nil
		return err
	}
	return nil
}

// Nop devolve um logger que descarta tudo (útil em testes).
func Nop() *Logger {
	return &Logger{Logger: slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))}
}
