package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Escopos do limitador. Enum fechado: é o prefixo da chave (EmailKey), o rótulo
// "escopo" da métrica de rejeições (R8) e o texto do log.
const (
	ScopeCadastro    = "cadastro"
	ScopeAgendamento = "agendamento"
)

// saturationWarnEvery limita o Warn de tabela saturada a no máximo 1 por minuto.
const saturationWarnEvery = time.Minute

// sweepEvery é o intervalo (em chamadas) da varredura de janelas expiradas.
const sweepEvery = 256

// LimiterOptions configura o Limiter.
type LimiterOptions struct {
	Limit    int                // tentativas permitidas por janela e por chave
	Window   time.Duration      // duração da janela fixa
	MaxKeys  int                // teto de chaves distintas (fail-closed de memória)
	Scope    string             // ScopeCadastro | ScopeAgendamento
	Log      *slog.Logger       // Warn de saturação; nil = slog.Default()
	OnReject func(scope string) // chamado em TODO 429 (estouro e saturação); nil = no-op
	Clock    func() time.Time   // nil = time.Now
}

type window struct {
	count int
	start time.Time
}

// Limiter é um rate limit em memória, de janela fixa por chave, sem dependência
// externa e sem goroutine. Seguro para uso concorrente.
//
// Anti-enumeração: a chave é um HASH (EmailKey) e toda tentativa conta, exista
// ou não a conta; a decisão não consulta o banco. O Limiter nunca guarda nem
// loga o valor original.
type Limiter struct {
	opts LimiterOptions

	mu       sync.Mutex
	entries  map[string]*window
	calls    int
	lastWarn time.Time
}

// NewLimiter cria o limitador aplicando os defaults (Clock, Log).
func NewLimiter(o LimiterOptions) *Limiter {
	if o.Clock == nil {
		o.Clock = time.Now
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	return &Limiter{opts: o, entries: make(map[string]*window)}
}

// Scope devolve o escopo do limitador (para montar a chave com EmailKey).
func (l *Limiter) Scope() string { return l.opts.Scope }

// Allow registra uma tentativa para key. ok=false quando a chave estourou a
// janela OU quando a tabela está saturada e a chave é nova; retryAfter é o
// tempo até uma nova tentativa fazer sentido. Em todo ok=false chama OnReject.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	ok, retryAfter = l.allow(key)
	if !ok && l.opts.OnReject != nil {
		l.opts.OnReject(l.opts.Scope)
	}
	return ok, retryAfter
}

func (l *Limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.opts.Clock()
	l.calls++
	if len(l.entries) > l.opts.MaxKeys/2 || l.calls%sweepEvery == 0 {
		l.sweep(now)
	}

	if w, found := l.entries[key]; found {
		if now.Sub(w.start) >= l.opts.Window {
			// Janela expirada: recomeça.
			w.start, w.count = now, 1
			return true, 0
		}
		if w.count >= l.opts.Limit {
			return false, l.opts.Window - now.Sub(w.start)
		}
		w.count++
		return true, 0
	}

	// Chave nova com a tabela cheia (ataque com e-mails distintos): recusa em
	// vez de crescer sem limite. Chaves existentes dentro do limite seguem.
	if len(l.entries) >= l.opts.MaxKeys {
		l.warnSaturated(now)
		return false, l.opts.Window
	}
	l.entries[key] = &window{count: 1, start: now}
	return true, 0
}

// sweep remove as janelas expiradas.
func (l *Limiter) sweep(now time.Time) {
	for k, w := range l.entries {
		if now.Sub(w.start) >= l.opts.Window {
			delete(l.entries, k)
		}
	}
}

// warnSaturated loga a saturação no máximo 1 vez por minuto, só com a contagem
// de chaves (nunca a chave nem o valor original).
func (l *Limiter) warnSaturated(now time.Time) {
	if !l.lastWarn.IsZero() && now.Sub(l.lastWarn) < saturationWarnEvery {
		return
	}
	l.lastWarn = now
	l.opts.Log.Warn("rate limit saturado: chaves novas recusadas",
		"escopo", l.opts.Scope, "chaves", len(l.entries))
}

// EmailKey deriva a chave do limitador: sha256(scope ":" lower(trim(valor))) em
// hex. Serve para e-mail e para ids (patient_id). O valor original nunca é
// guardado nem logado; a normalização faz "A@x.com" e " a@x.com " colidirem.
func EmailKey(scope, value string) string {
	sum := sha256.Sum256([]byte(scope + ":" + strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(sum[:])
}
