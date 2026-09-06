// Command psicoman-portal é o binário do portal do paciente (self-service),
// atrás do Pangolin apenas para TLS. Autenticação própria via login social
// Google; sem acesso a dados clínicos.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/RafaelFino/psicoman/internal/app"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "caminho do arquivo de configuração")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.RunPortal(ctx, *cfgPath); err != nil {
		os.Exit(1)
	}
}
