package app

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// Parâmetros do store de `state` do OAuth (R7.2).
const (
	// OAuthStateTTL é a validade de um state emitido (tempo para o operador
	// concluir o consentimento no Google).
	OAuthStateTTL = 10 * time.Minute
	// OAuthStateMaxPending é o teto de states pendentes em memória.
	OAuthStateMaxPending = 100
)

// OAuthStateStore emite e consome o `state` do fluxo OAuth (proteção de CSRF).
// Uso único e com expiração: o state só vale uma vez e por OAuthStateTTL.
type OAuthStateStore interface {
	// Issue gera um state imprevisível e o registra como pendente.
	Issue() (state string)
	// Consume devolve true UMA única vez para um state emitido e ainda não
	// expirado; qualquer outro caso (desconhecido, expirado, reuso) devolve false.
	Consume(state string) bool
}

// MemoryOAuthStateStore implementa OAuthStateStore em memória. O processo é
// único, então memória basta: perder os states num restart no meio do
// consentimento só obriga a repetir o fluxo.
type MemoryOAuthStateStore struct {
	clk        clock.Clock
	ttl        time.Duration
	maxPending int

	mu      sync.Mutex
	pending map[string]time.Time // state -> expiração
}

var _ OAuthStateStore = (*MemoryOAuthStateStore)(nil)

// NewMemoryOAuthStateStore cria o store. clk nil = relógio do sistema.
func NewMemoryOAuthStateStore(clk clock.Clock, ttl time.Duration, maxPending int) *MemoryOAuthStateStore {
	if clk == nil {
		clk = clock.System{}
	}
	return &MemoryOAuthStateStore{clk: clk, ttl: ttl, maxPending: maxPending, pending: make(map[string]time.Time)}
}

// Issue gera 16 bytes de crypto/rand (128 bits) em base64url. Não usa ULID: o
// prefixo temporal do ULID é previsível. Descarta os expirados e, se ainda
// estiver cheio, remove o mais antigo.
func (s *MemoryOAuthStateStore) Issue() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand falhar é irrecuperável para um valor de segurança: não há
		// fallback seguro. O handler responde 500 via panic recuperado.
		panic("oauth state: crypto/rand indisponível: " + err.Error())
	}
	state := base64.RawURLEncoding.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clk.Now()
	s.sweep(now)
	for len(s.pending) >= s.maxPending {
		s.evictOldest()
	}
	s.pending[state] = now.Add(s.ttl)
	return state
}

// Consume valida e remove o state (uso único).
func (s *MemoryOAuthStateStore) Consume(state string) bool {
	if state == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.pending[state]
	if !ok {
		return false
	}
	delete(s.pending, state) // consome mesmo se expirado: nunca reutilizável
	return s.clk.Now().Before(exp)
}

// sweep remove os states expirados. Chamar com s.mu travado.
func (s *MemoryOAuthStateStore) sweep(now time.Time) {
	for k, exp := range s.pending {
		if !now.Before(exp) {
			delete(s.pending, k)
		}
	}
}

// evictOldest remove o state com a expiração mais antiga. Chamar com s.mu travado.
func (s *MemoryOAuthStateStore) evictOldest() {
	var oldestKey string
	var oldest time.Time
	for k, exp := range s.pending {
		if oldestKey == "" || exp.Before(oldest) {
			oldestKey, oldest = k, exp
		}
	}
	delete(s.pending, oldestKey)
}
