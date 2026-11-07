// Package metrics é a implementação Prometheus da observabilidade do Psicoman
// (R8). É a ÚNICA camada, junto com internal/api/metricsserver.go (que serve o
// Handler), que importa a lib client_golang: domain, app e config só conhecem a
// porta app.Metrics (ver archtest).
//
// Princípios (R8.9): registry PRÓPRIO (nunca o DefaultRegisterer, para testes
// isolados); rótulos SÓ de enums fechados — nunca id, e-mail, nome, identidade
// do ator ou texto livre; nada de dado de paciente em nenhuma série.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// namespace é o prefixo de todas as famílias (psicoman_*).
const namespace = "psicoman"

// BuildInfo identifica a build (rótulos da série build_info).
type BuildInfo struct {
	Version   string // versão do módulo/commit (debug.ReadBuildInfo)
	GoVersion string
}

// Config parametriza o Obs.
type Config struct {
	Build BuildInfo
	// DevIdentity alimenta identidade_dev_ativa (1 se pangolin.dev_identity não
	// vazio): em produção deve ser 0 — vira alerta.
	DevIdentity bool
	// Queues alimenta os gauges de fila (pacientes e pedidos pendentes), lidos
	// por COUNT com timeout e cache de 15 s. nil = omite as famílias de fila.
	Queues app.QueueStats
	// AIEnabled alimenta gemini_ligada (1 se há api_key).
	AIEnabled bool
	// GoogleOK é a fonte de google_oauth_ok (mesmo critério do /readyz). nil =
	// omite a família.
	GoogleOK func() bool
	// Clock é a fonte de tempo do uptime e do cache das filas; nil = relógio real.
	Clock clock.Clock
}

// Obs agrega o registry e os recorders. Criado SEMPRE no boot (o registry é
// barato); só o listener depende de metrics.addr.
type Obs struct {
	registry *prometheus.Registry
	http     *HTTPMetrics
	drive    *DriveMetrics
	app      app.Metrics
}

// New cria o Obs com o registry próprio, os coletores padrão (Go e processo) e
// as famílias de runtime/segurança.
func New(c Config) *Obs {
	if c.Clock == nil {
		c.Clock = clock.System{}
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	registerRuntime(reg, c)
	registerIntegrationGauges(reg, c)
	if c.Queues != nil {
		newQueueCollector(reg, c.Queues, c.Clock)
	}

	return &Obs{
		registry: reg,
		http:     newHTTPMetrics(reg),
		drive:    newDriveMetrics(reg, c.Clock),
		app:      newAppMetrics(reg),
	}
}

// registerIntegrationGauges registra gemini_ligada e google_oauth_ok.
func registerIntegrationGauges(reg *prometheus.Registry, c Config) {
	ai := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Name: "gemini_ligada",
		Help: "1 se a IA (Gemini) está ligada (há api_key).",
	})
	if c.AIEnabled {
		ai.Set(1)
	}
	reg.MustRegister(ai)

	if c.GoogleOK != nil {
		reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: namespace, Name: "google_oauth_ok",
			Help: "1 se a autorização OAuth do Google está ok; 0 se degradada (mesmo critério do /readyz).",
		}, func() float64 {
			if c.GoogleOK() {
				return 1
			}
			return 0
		}))
	}
}

// registerRuntime registra build_info, uptime_segundos e identidade_dev_ativa.
func registerRuntime(reg *prometheus.Registry, c Config) {
	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace, Name: "build_info",
		Help: "Informações da build (valor sempre 1).",
	}, []string{"versao", "go"})
	buildInfo.WithLabelValues(c.Build.Version, c.Build.GoVersion).Set(1)

	start := c.Clock.Now()
	uptime := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace, Name: "uptime_segundos",
		Help: "Segundos desde o início do processo.",
	}, func() float64 { return c.Clock.Now().Sub(start).Seconds() })

	dev := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Name: "identidade_dev_ativa",
		Help: "1 se pangolin.dev_identity está ativa (deve ser 0 em produção).",
	})
	if c.DevIdentity {
		dev.Set(1)
	}

	reg.MustRegister(buildInfo, uptime, dev)
}

// Handler serve o registry no formato de exposição do Prometheus. Sem envelope
// JSON e sem gate do Pangolin: roda no listener dedicado (internal/api).
func (o *Obs) Handler() http.Handler {
	return promhttp.HandlerFor(o.registry, promhttp.HandlerOpts{
		ErrorHandling:       promhttp.ContinueOnError,
		Timeout:             10 * time.Second,
		MaxRequestsInFlight: 4,
	})
}

// App devolve a porta de negócio (app.Metrics).
func (o *Obs) App() app.Metrics { return o.app }

// HTTP devolve o recorder do middleware HTTP e do limitador. O tipo concreto
// satisfaz httpx.HTTPRecorder estruturalmente (sem importar httpx).
func (o *Obs) HTTP() *HTTPMetrics { return o.http }

// Drive devolve o adapter de observabilidade do espelhamento de DR. O tipo
// concreto satisfaz drivesync.Observer estruturalmente.
func (o *Obs) Drive() *DriveMetrics { return o.drive }
