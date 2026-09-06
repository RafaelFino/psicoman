package portal

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// RateLimiter aplica um limite de requisições por chave (IP e/ou email), usando
// token bucket simples em memória. Protege as rotas públicas do portal, já que
// o Pangolin não faz controle de acesso aqui (psicoman-seguranca-lgpd.md).
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens por segundo
	burst   float64
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewRateLimiter cria o limitador a partir de requisições/minuto e burst.
func NewRateLimiter(perMinute, burst int) *RateLimiter {
	if perMinute <= 0 {
		perMinute = 30
	}
	if burst <= 0 {
		burst = 10
	}
	return &RateLimiter{
		buckets: map[string]*bucket{},
		rate:    float64(perMinute) / 60.0,
		burst:   float64(burst),
	}
}

// allow consome um token da chave; devolve false se estourou o limite.
func (l *RateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := clock.Now()
	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Middleware limita por IP. Para limitar também por email, o handler pode
// chamar AllowEmail antes de processar.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.allow("ip:" + ip) {
			httpx.RespondError(w, r, httpx.ErrTooManyRequests(
				"Muitas requisições. Aguarde um instante e tente novamente."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AllowEmail verifica o limite por email (usado nas rotas de cadastro/pedido).
func (l *RateLimiter) AllowEmail(email string) bool {
	if email == "" {
		return true
	}
	return l.allow("email:" + email)
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := indexComma(fwd); i >= 0 {
			return fwd[:i]
		}
		return fwd
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}
