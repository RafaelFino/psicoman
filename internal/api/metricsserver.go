package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// listen é o seam de teste para abrir o socket do listener de métricas: os
// testes o substituem por uma função que registra a chamada (provando que, sem
// metrics.addr, NENHUM socket é aberto) ou que devolve um listener em porta
// efêmera.
var listen = net.Listen

// StartMetricsServer sobe o listener DEDICADO de métricas (R8.1), separado da
// API. Fail-closed: addr == "" retorna nil SEM chamar listen (nenhum socket
// extra). O mux próprio expõe SÓ `GET /metrics`; qualquer outra rota é 404 do
// mux padrão — o listener não serve a API e a API não serve /metrics. Sem
// envelope JSON (formato de exposição do Prometheus) e sem gate do Pangolin: o
// isolamento é de rede (ver decisions.md e deploy.md).
//
// O bind é síncrono: falha (porta em uso) ABORTA o boot, pois a config foi
// pedida deliberadamente e um listener silenciosamente ausente esconderia a
// falta de observabilidade. O Shutdown é registrado em OnTerminate. O listener
// não é instrumentado por si mesmo (evita ruído de scrape nas séries).
func StartMetricsServer(app core.App, addr string, h http.Handler, log *slog.Logger) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	if log == nil {
		log = slog.Default()
	}

	ln, err := listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("métricas: abrir listener em %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", h)
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	go func() {
		if serr := srv.Serve(ln); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			log.Error("listener de métricas encerrou com erro", "erro", serr.Error())
		}
	}()

	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		return e.Next()
	})

	log.Info("listener de métricas ativo", "addr", ln.Addr().String())
	if host, _, serr := net.SplitHostPort(addr); serr == nil && (host == "" || host == "0.0.0.0" || host == "::") {
		log.Warn("listener de métricas em TODAS as interfaces: garanta que a porta NÃO está publicada na borda (rede interna/firewall do coletor)")
	}
	return nil
}
