// Package metrics coleta métricas simples em memória e as expõe em texto
// (formato compatível com Prometheus) no endpoint /metrics.
//
// MVP: contadores e latências agregadas sem dependência externa
// (docs/architecture.md §6).
package metrics

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Registry acumula métricas do processo.
type Registry struct {
	mu sync.Mutex

	counters map[string]int64
	// latency: soma em ms e contagem por rota, para média.
	latSum   map[string]float64
	latCount map[string]int64
}

// New cria um Registry vazio.
func New() *Registry {
	return &Registry{
		counters: map[string]int64{},
		latSum:   map[string]float64{},
		latCount: map[string]int64{},
	}
}

// Inc incrementa um contador nomeado.
func (r *Registry) Inc(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[name]++
}

// Add soma um valor a um contador nomeado.
func (r *Registry) Add(name string, delta int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[name] += delta
}

// ObserveRequest registra latência e status de uma requisição.
func (r *Registry) ObserveRequest(route string, status int, dur time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[fmt.Sprintf("psicoman_requests_total{route=%q,status=\"%d\"}", route, status)]++
	r.latSum[route] += float64(dur.Microseconds()) / 1000.0
	r.latCount[route]++
}

// Render serializa as métricas em texto exposition format.
func (r *Registry) Render() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var b []byte
	keys := make([]string, 0, len(r.counters))
	for k := range r.counters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b = append(b, fmt.Sprintf("%s %d\n", k, r.counters[k])...)
	}

	routes := make([]string, 0, len(r.latCount))
	for k := range r.latCount {
		routes = append(routes, k)
	}
	sort.Strings(routes)
	for _, route := range routes {
		avg := r.latSum[route] / float64(r.latCount[route])
		b = append(b, fmt.Sprintf("psicoman_request_latency_ms_avg{route=%q} %.3f\n", route, avg)...)
	}
	return string(b)
}
