// Command psicoman-admin é o binário administrativo (terapeuta), atrás do
// Pangolin com controle de acesso. Acesso total à operação do consultório.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RafaelFino/psicoman/internal/app"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "caminho do arquivo de configuração")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.RunAdmin(ctx, *cfgPath); err != nil {
		// RunAdmin já loga; garante código de saída não-zero.
		os.Exit(1)
	}
	// Aguarda encerramento gracioso.
	_ = time.Second
}
