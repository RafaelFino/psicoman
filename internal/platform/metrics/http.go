package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Buckets: duração de 5 ms a 10 s; tamanho de resposta de 256 B a 1 MiB.
var (
	durationBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	bytesBuckets    = prometheus.ExponentialBuckets(256, 4, 7) // 256 B ... 1 MiB
)

// HTTPMetrics implementa o recorder do middleware HTTP (httpx.HTTPRecorder).
// Os valores de rótulo vêm de conjuntos fechados montados pelo middleware
// (ator, rota = padrão registrado, método normalizado, status numérico).
type HTTPMetrics struct {
	requests    *prometheus.CounterVec
	duration    *prometheus.HistogramVec
	inflight    *prometheus.GaugeVec
	respBytes   *prometheus.HistogramVec
	denied      *prometheus.CounterVec
	rateLimited *prometheus.CounterVec
}

func newHTTPMetrics(reg *prometheus.Registry) *HTTPMetrics {
	m := &HTTPMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "http_requisicoes_total",
			Help: "Requisições HTTP por ator, rota (padrão registrado), método e status.",
		}, []string{"ator", "rota", "metodo", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "http_duracao_segundos",
			Help: "Duração das requisições HTTP.", Buckets: durationBuckets,
		}, []string{"ator", "rota", "metodo"}),
		inflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Name: "http_em_andamento",
			Help: "Requisições HTTP em andamento por ator.",
		}, []string{"ator"}),
		respBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "http_resposta_bytes",
			Help: "Tamanho do corpo das respostas HTTP.", Buckets: bytesBuckets,
		}, []string{"ator", "rota"}),
		denied: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "seguranca_negados_total",
			Help: "Acessos negados (401/403) por ator, status e motivo.",
		}, []string{"ator", "status", "motivo"}),
		rateLimited: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "rate_limit_rejeicoes_total",
			Help: "Requisições recusadas pelo rate limit (429) por escopo.",
		}, []string{"escopo"}),
	}
	reg.MustRegister(m.requests, m.duration, m.inflight, m.respBytes, m.denied, m.rateLimited)
	return m
}

// Request registra uma requisição concluída. bytes < 0 = tamanho desconhecido
// (a amostra de tamanho é omitida).
func (m *HTTPMetrics) Request(ator, rota, metodo string, status int, d time.Duration, bytes int64) {
	m.requests.WithLabelValues(ator, rota, metodo, strconv.Itoa(status)).Inc()
	m.duration.WithLabelValues(ator, rota, metodo).Observe(d.Seconds())
	if bytes >= 0 {
		m.respBytes.WithLabelValues(ator, rota).Observe(float64(bytes))
	}
}

// InFlight ajusta o gauge de requisições em andamento (+1 / -1).
func (m *HTTPMetrics) InFlight(ator string, delta int) {
	m.inflight.WithLabelValues(ator).Add(float64(delta))
}

// Denied registra um acesso negado (401/403) com o motivo.
func (m *HTTPMetrics) Denied(ator string, status int, motivo string) {
	m.denied.WithLabelValues(ator, strconv.Itoa(status), motivo).Inc()
}

// RateLimited registra uma rejeição do limitador (escopo: cadastro|agendamento).
func (m *HTTPMetrics) RateLimited(scope string) {
	m.rateLimited.WithLabelValues(scope).Inc()
}
