package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/RafaelFino/psicoman/internal/api/middleware"
	"github.com/RafaelFino/psicoman/internal/platform/logger"
	"github.com/RafaelFino/psicoman/internal/platform/metrics"
)

// Server encapsula um http.Server com a chain padrão de middleware já aplicada.
type Server struct {
	http    *http.Server
	log     *logger.Logger
	mux     *http.ServeMux
	metrics *metrics.Registry
}

// NewServer cria um servidor ouvindo em addr, com a chain base
// (recover → request-id → timing → logging) aplicada a todas as rotas do mux.
func NewServer(addr string, log *logger.Logger, reg *metrics.Registry) *Server {
	mux := http.NewServeMux()
	handler := middleware.Chain(mux,
		middleware.Recover(log),
		middleware.RequestID(),
		middleware.Timing(reg),
		middleware.Logging(log),
	)
	return &Server{
		http: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
		log:     log,
		mux:     mux,
		metrics: reg,
	}
}

// Mux devolve o mux para registro de rotas.
func (s *Server) Mux() *http.ServeMux { return s.mux }

// Start sobe o servidor de forma bloqueante.
func (s *Server) Start() error {
	s.log.Info("servidor pronto, aguardando requisições", "endereco", "http://"+s.http.Addr)
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown encerra o servidor graciosamente.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
