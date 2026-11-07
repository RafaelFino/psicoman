package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// DriveMetrics implementa drivesync.Observer (estruturalmente, sem importar o
// pacote) e expõe as famílias de backup e de DR no Drive (R8.6).
//
// Duas séries são DERIVADAS aqui, sem métodos extras no Observer:
//   - backup_ultimo_sucesso_timestamp_segundos{destino}: relógio no momento de um
//     Backup(destino, ok=true, ...). Vale 0 até o primeiro sucesso (documentado;
//     é a base do alerta "backup velho").
//   - drive_dr_pronto: 1 quando o último Mirror foi "ok"; 0 em "erro"/"desativado".
//
// drive_snapshots_mantidos é ausente enquanto não houver contagem confiável: a
// listagem que falha NÃO atualiza (nem zera) o gauge — apaga a amostra.
type DriveMetrics struct {
	clk clock.Clock

	backups   *prometheus.CounterVec
	duration  *prometheus.HistogramVec
	size      *prometheus.GaugeVec
	lastOK    *prometheus.GaugeVec
	mirrors   *prometheus.CounterVec
	ready     prometheus.Gauge
	retFails  *prometheus.CounterVec
	keptDesc  *prometheus.Desc
	mu        sync.Mutex
	kept      int
	keptKnown bool
}

func newDriveMetrics(reg *prometheus.Registry, clk clock.Clock) *DriveMetrics {
	d := &DriveMetrics{
		clk: clk,
		backups: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "backup_total",
			Help: "Backups por destino (local|drive) e resultado.",
		}, []string{"destino", "resultado"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "backup_duracao_segundos",
			Help: "Duração do backup por destino.", Buckets: backupBuckets,
		}, []string{"destino"}),
		size: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Name: "backup_tamanho_bytes",
			Help: "Tamanho do último snapshot por destino (no drive, o blob cifrado).",
		}, []string{"destino"}),
		lastOK: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Name: "backup_ultimo_sucesso_timestamp_segundos",
			Help: "Instante (epoch, s) do último backup bem-sucedido por destino; 0 até o primeiro sucesso.",
		}, []string{"destino"}),
		mirrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "drive_espelhamento_total",
			Help: "Espelhamentos do snapshot no Drive por resultado (ok|erro|desativado).",
		}, []string{"resultado"}),
		ready: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "drive_dr_pronto",
			Help: "1 quando o DR no Drive está ativo e o último ciclo foi ok.",
		}),
		retFails: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "drive_retencao_falhas_total",
			Help: "Falhas da retenção de snapshots no Drive por etapa (listar|lixeira).",
		}, []string{"etapa"}),
		keptDesc: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "drive_snapshots_mantidos"),
			"Snapshots mantidos no Drive após a retenção; ausente se a listagem falhou.", nil, nil),
	}
	// Pré-inicializa o timestamp em 0 para os dois destinos (alerta funciona
	// desde o primeiro scrape, antes de qualquer backup).
	d.lastOK.WithLabelValues("local").Set(0)
	d.lastOK.WithLabelValues("drive").Set(0)
	reg.MustRegister(d.backups, d.duration, d.size, d.lastOK, d.mirrors, d.ready, d.retFails, d)
	return d
}

// backupBuckets: backups levam de segundos a minutos.
var backupBuckets = []float64{.1, .5, 1, 2.5, 5, 10, 30, 60, 120, 300}

// destinoLabel/resultadoLabel/etapaLabel fecham os conjuntos de rótulo.
func destinoLabel(d string) string {
	if d == "local" || d == "drive" {
		return d
	}
	return "outro"
}

func mirrorLabel(r string) string {
	switch r {
	case "ok", "erro", "desativado":
		return r
	}
	return "outro"
}

func etapaLabel(e string) string {
	if e == "listar" || e == "lixeira" {
		return e
	}
	return "outro"
}

// Backup registra um backup concluído. size só vira gauge em caso de sucesso.
func (d *DriveMetrics) Backup(destino string, ok bool, dur time.Duration, sizeBytes int64) {
	dest := destinoLabel(destino)
	res := "erro"
	if ok {
		res = "ok"
	}
	d.backups.WithLabelValues(dest, res).Inc()
	d.duration.WithLabelValues(dest).Observe(dur.Seconds())
	if ok {
		d.size.WithLabelValues(dest).Set(float64(sizeBytes))
		d.lastOK.WithLabelValues(dest).Set(float64(d.clk.Now().Unix()))
	}
}

// Mirror registra o resultado do espelhamento e deriva drive_dr_pronto.
func (d *DriveMetrics) Mirror(resultado string) {
	r := mirrorLabel(resultado)
	d.mirrors.WithLabelValues(r).Inc()
	if r == "ok" {
		d.ready.Set(1)
	} else {
		d.ready.Set(0)
	}
}

// Retention atualiza a contagem de snapshots mantidos.
func (d *DriveMetrics) Retention(kept int) {
	d.mu.Lock()
	d.kept, d.keptKnown = kept, true
	d.mu.Unlock()
}

// RetentionFailure conta a falha por etapa. Falha ao LISTAR invalida a contagem
// (a amostra some) — nunca se reporta um número que não foi confirmado.
func (d *DriveMetrics) RetentionFailure(etapa string) {
	e := etapaLabel(etapa)
	d.retFails.WithLabelValues(e).Inc()
	if e == "listar" {
		d.mu.Lock()
		d.keptKnown = false
		d.mu.Unlock()
	}
}

// Describe/Collect fazem do DriveMetrics um Collector apenas para o gauge
// drive_snapshots_mantidos (que precisa poder estar ausente).
func (d *DriveMetrics) Describe(ch chan<- *prometheus.Desc) { ch <- d.keptDesc }

func (d *DriveMetrics) Collect(ch chan<- prometheus.Metric) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.keptKnown {
		ch <- prometheus.MustNewConstMetric(d.keptDesc, prometheus.GaugeValue, float64(d.kept))
	}
}
