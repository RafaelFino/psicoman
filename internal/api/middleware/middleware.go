// Package middleware implementa a chain padrão de toda rota HTTP:
//
//	recover → request-id → timing → logging → auth → handler
//
// (docs/architecture.md §5). O auth é específico de cada binário e injetado
// separadamente (admin: Pangolin; portal: sessão social).
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/logger"
	"github.com/RafaelFino/psicoman/internal/platform/metrics"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// Middleware é a assinatura de um wrapper de handler.
type Middleware func(http.Handler) http.Handler

// Chain compõe middlewares na ordem informada (o primeiro é o mais externo).
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusRecorder captura o status code escrito pelo handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status = code
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status = http.StatusOK
		s.wrote = true
	}
	return s.ResponseWriter.Write(b)
}

// Recover captura panics e devolve 500 com envelope PT-BR, logando o stack.
func Recover(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recuperado",
						"panic", rec,
						"path", r.URL.Path,
						"request_id", httpx.RequestID(r.Context()),
					)
					httpx.RespondError(w, r, httpx.ErrInternal(nil))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID injeta um identificador de requisição (ULID) no contexto e no header.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = ulid.New()
			}
			ctx := httpx.WithRequestID(r.Context(), id)
			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Timing marca o início da requisição no contexto (usado pelo envelope) e
// registra a latência nas métricas.
func Timing(reg *metrics.Registry) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := clock.Now()
			ctx := context.WithValue(r.Context(), httpx.CtxKeyStart(), start)
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r.WithContext(ctx))
			if reg != nil {
				reg.ObserveRequest(routeLabel(r), rec.status, time.Since(start))
			}
		})
	}
}

// Logging registra cada requisição concluída.
func Logging(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := clock.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info("requisição",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", httpx.RequestID(r.Context()),
			)
		})
	}
}

// routeLabel produz um rótulo de rota para métricas (método + path).
func routeLabel(r *http.Request) string {
	if pat := r.Pattern; pat != "" {
		return pat
	}
	return r.Method + " " + r.URL.Path
}
