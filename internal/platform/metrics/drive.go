package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DriveMetrics implementa drivesync.Observer. Na Fase 7 é só o ponto de ligação
// (sem famílias); as famílias de backup/Drive entram na Fase 8. Todos os métodos
// são no-op por ora, então ligá-lo ao Syncer já é seguro.
type DriveMetrics struct{}

func newDriveMetrics(_ *prometheus.Registry) *DriveMetrics { return &DriveMetrics{} }

func (*DriveMetrics) Backup(string, bool, time.Duration, int64) {}
func (*DriveMetrics) Mirror(string)                             {}
func (*DriveMetrics) Retention(int)                             {}
func (*DriveMetrics) RetentionFailure(string)                   {}
