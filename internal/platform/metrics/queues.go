package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// Parâmetros do coletor de filas: o scrape nunca martela o SQLite.
const (
	queueSourceTimeout = 2 * time.Second  // limite por fonte (COUNT)
	queueCacheTTL      = 15 * time.Second // resultado reaproveitado entre scrapes
)

// Fontes (rótulo `fonte` de metrics_coleta_erros_total): conjunto fechado.
const (
	fontePacientes   = "pacientes_pendentes"
	fonteAgendamento = "agendamentos_pendentes"
)

// queueCollector é um prometheus.Collector custom: lê as filas de trabalho por
// COUNT, com timeout por fonte e cache de 15 s. Em erro OMITE a amostra (nunca
// reporta um zero falso) e incrementa metrics_coleta_erros_total{fonte}.
type queueCollector struct {
	src app.QueueStats
	clk clock.Clock

	patientsDesc *prometheus.Desc
	apptsDesc    *prometheus.Desc
	errsDesc     *prometheus.Desc

	mu       sync.Mutex
	patients cached
	appts    cached
	// errCount conta as falhas por fonte. É emitido pelo PRÓPRIO coletor (e não por
	// um CounterVec separado) para o contador refletir a falha no mesmo scrape: o
	// registry coleta os coletores em paralelo, então um contador externo
	// incrementado dentro de Collect poderia aparecer só no scrape seguinte.
	errCount map[string]float64
}

// cached guarda o último valor lido com sucesso e quando.
type cached struct {
	val int
	at  time.Time
	ok  bool
}

func newQueueCollector(reg *prometheus.Registry, src app.QueueStats, clk clock.Clock) *queueCollector {
	c := &queueCollector{
		src:      src,
		clk:      clk,
		errCount: map[string]float64{},
		errsDesc: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "metrics_coleta_erros_total"),
			"Falhas ao coletar uma fonte de métrica por consulta (a amostra é omitida).", []string{"fonte"}, nil),
		patientsDesc: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "fila_pacientes_pendentes"),
			"Pacientes com cadastro pendente de aprovação.", nil, nil),
		apptsDesc: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "fila_agendamentos_pendentes"),
			"Pedidos de agendamento pendentes de resposta.", nil, nil),
	}
	reg.MustRegister(c)
	return c
}

func (c *queueCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.patientsDesc
	ch <- c.apptsDesc
	ch <- c.errsDesc
}

func (c *queueCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clk.Now()

	c.refresh(now, &c.patients, fontePacientes, c.src.PendingPatients)
	c.refresh(now, &c.appts, fonteAgendamento, c.src.PendingAppointments)

	if c.patients.ok {
		ch <- prometheus.MustNewConstMetric(c.patientsDesc, prometheus.GaugeValue, float64(c.patients.val))
	}
	if c.appts.ok {
		ch <- prometheus.MustNewConstMetric(c.apptsDesc, prometheus.GaugeValue, float64(c.appts.val))
	}
	for fonte, n := range c.errCount {
		ch <- prometheus.MustNewConstMetric(c.errsDesc, prometheus.CounterValue, n, fonte)
	}
}

// refresh relê a fonte quando o cache expirou. Falha: invalida o valor (a
// amostra é omitida), conta o erro e tenta de novo no próximo scrape.
func (c *queueCollector) refresh(now time.Time, slot *cached, fonte string, read func(context.Context) (int, error)) {
	if slot.ok && now.Sub(slot.at) < queueCacheTTL {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), queueSourceTimeout)
	defer cancel()
	v, err := read(ctx)
	if err != nil {
		*slot = cached{}
		c.errCount[fonte]++
		return
	}
	*slot = cached{val: v, at: now, ok: true}
}
