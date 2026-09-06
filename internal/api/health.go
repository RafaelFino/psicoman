// Package api contém a montagem dos servidores HTTP (admin e portal) e os
// handlers compartilhados de observabilidade.
package api

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/platform/metrics"
)

// HealthChecker avalia a prontidão de uma dependência (ex: SQLite, token Google).
type HealthChecker interface {
	// Check devolve nil se saudável, ou um erro descrevendo o problema.
	Check() error
}

// HealthCheckFunc adapta uma função ao HealthChecker.
type HealthCheckFunc func() error

// Check implementa HealthChecker.
func (f HealthCheckFunc) Check() error { return f() }

// Health agrupa os handlers de saúde e métricas.
type Health struct {
	readiness []namedChecker
	metrics   *metrics.Registry
}

type namedChecker struct {
	name    string
	checker HealthChecker
}

// NewHealth cria o agregador de saúde.
func NewHealth(reg *metrics.Registry) *Health {
	return &Health{metrics: reg}
}

// AddReadiness registra uma checagem de readiness.
func (h *Health) AddReadiness(name string, c HealthChecker) {
	h.readiness = append(h.readiness, namedChecker{name: name, checker: c})
}

// Register instala /healthz, /readyz, /livez e /metrics no mux.
func (h *Health) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.handleHealthz)
	mux.HandleFunc("GET /livez", h.handleLivez)
	mux.HandleFunc("GET /readyz", h.handleReadyz)
	mux.HandleFunc("GET /metrics", h.handleMetrics)
}

func (h *Health) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Health) handleLivez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"alive"}`))
}

func (h *Health) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	degraded := map[string]string{}
	for _, c := range h.readiness {
		if err := c.checker.Check(); err != nil {
			degraded[c.name] = err.Error()
		}
	}
	if len(degraded) > 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"status": "degraded", "checks": degraded})
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

func (h *Health) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if h.metrics != nil {
		_, _ = w.Write([]byte(h.metrics.Render()))
	}
}
