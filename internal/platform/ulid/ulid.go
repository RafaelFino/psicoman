// Package ulid é o gerador compartilhado de identificadores do projeto.
//
// Regra do projeto: toda entidade usa ULID (lexicograficamente ordenável e
// único), nunca int autoincrement (psicoman-golang.md, psicoman-sqlite.md).
package ulid

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

// New gera um novo ULID como string, monotônico dentro do mesmo milissegundo.
func New() string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

// NewAt gera um ULID com timestamp específico (útil em testes/backfill).
func NewAt(t time.Time) string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}

// Valid indica se s é um ULID sintaticamente válido.
func Valid(s string) bool {
	_, err := ulid.ParseStrict(s)
	return err == nil
}
