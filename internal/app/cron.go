package app

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/logger"
	"github.com/RafaelFino/psicoman/internal/service"
)

// startCycleCloseJob roda o fechamento de ciclo dos planos fechados uma vez ao
// dia (docs/architecture.md §4.1.1). É idempotente, então rodar mais de uma vez
// no mesmo dia não duplica débitos.
func startCycleCloseJob(ctx context.Context, log *logger.Logger, billing *service.BillingService) {
	// Executa uma vez no boot (cobre reinícios) e depois a cada 24h.
	run := func() {
		n, err := billing.CloseCycles(ctx, clock.Now())
		if err != nil {
			log.Error("job de fechamento de ciclo falhou", "error", err)
			return
		}
		if n > 0 {
			log.Info("fechamento de ciclo gerou débitos", "debts_created", n)
		}
	}

	go func() {
		run()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

// startBackupJob roda o backup cifrado no Drive uma vez ao dia
// (docs/architecture.md §4.4). Falha de backup é logada, não derruba o processo.
func startBackupJob(ctx context.Context, log *logger.Logger, backup *service.BackupService) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				res, err := backup.Backup(ctx)
				if err != nil {
					log.Error("job de backup falhou", "error", err)
					continue
				}
				log.Info("backup diário concluído",
					"ged_uploaded", res.GEDUploaded, "ged_skipped", res.GEDSkipped)
			}
		}
	}()
}
