package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// HTTPRecorder é a interface de observabilidade HTTP que o middleware usa.
// httpx NÃO importa a lib de métricas: platform/metrics a implementa (R8.11).
// Todos os valores recebidos vêm de conjuntos fechados montados aqui (ator, rota
// = padrão registrado, método normalizado, status numérico, motivo) — nunca de
// dado do request (R8.9).
type HTTPRecorder interface {
	// Request registra uma requisição concluída. bytes < 0 = desconhecido.
	Request(ator, rota, metodo string, status int, d time.Duration, bytes int64)
	InFlight(ator string, delta int) // +1 / -1
	Denied(ator string, status int, motivo string)
	RateLimited(scope string) // ligado a Limiter.OnReject (R7.1)
}

// DenyReasonCode é o motivo de um 401/403, conjunto FECHADO (rótulo de métrica).
type DenyReasonCode string

const (
	DenySemIdentidade DenyReasonCode = "sem_identidade"
	DenySemSessao     DenyReasonCode = "sem_sessao"
	DenyNaoAprovado   DenyReasonCode = "nao_aprovado"
	DenyPocketBase    DenyReasonCode = "pocketbase"
	DenyOutro         DenyReasonCode = "outro"
)

// SetDenyReason grava o motivo da negação no store do evento; o middleware de
// métricas o lê após Next(). Chamado por RequirePangolinIdentity e pelo gate do
// portal, imediatamente antes de responder 401/403.
func SetDenyReason(e *core.RequestEvent, r DenyReasonCode) { e.Set(storeDenyReason, string(r)) }

// DenyReason devolve o motivo gravado ("" se ausente).
func DenyReason(e *core.RequestEvent) DenyReasonCode {
	if v, ok := e.Get(storeDenyReason).(string); ok {
		return DenyReasonCode(v)
	}
	return ""
}

// Atores (rótulo `ator`): 4 valores.
const (
	atorApp    = "app"
	atorPortal = "portal"
	atorAdmin  = "admin"
	atorInfra  = "infra"
)

// Rotas sintéticas (nunca o URL.Path, para não explodir a cardinalidade).
const (
	rotaNaoEncontrada = "nao_encontrada"
	rotaDesconhecida  = "desconhecida"
)

// atorOfPath classifica um caminho pelo prefixo fixo. Usado SÓ para o gauge de
// requisições em andamento (antes de Next, quando o Pattern ainda não importa).
func atorOfPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/v1/app/"):
		return atorApp
	case strings.HasPrefix(path, "/v1/portal/"):
		return atorPortal
	case strings.HasPrefix(path, "/v1/admin/"):
		return atorAdmin
	default:
		return atorInfra
	}
}

// RouteLabels deriva (ator, rota) do padrão que o http.ServeMux casou
// (Request.Pattern, ex.: "GET /v1/app/patients/{id}"). Remove o prefixo de
// método; "/" (catch-all do PocketBase) vira "nao_encontrada"; vazio vira
// "desconhecida". NUNCA usa o URL.Path, então um id (ULID) jamais vira rótulo.
func RouteLabels(pattern string) (ator, rota string) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return atorInfra, rotaDesconhecida
	}
	// Remove "MÉTODO " quando o padrão tem espaço antes da primeira barra.
	if i := strings.IndexByte(pattern, ' '); i >= 0 && i < strings.IndexByte(pattern, '/') {
		pattern = strings.TrimSpace(pattern[i+1:])
	}
	if pattern == "/" {
		return atorInfra, rotaNaoEncontrada
	}
	return atorOfPath(pattern), pattern
}

// metodoLabel normaliza o método HTTP para um conjunto fechado.
func metodoLabel(m string) string {
	switch m {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions:
		return m
	default:
		return "OUTRO"
	}
}

// countingWriter conta os bytes escritos no corpo e delega tudo ao writer
// original. Expõe Unwrap() para o PocketBase continuar enxergando o
// StatusTracker/WriteTracker, o Flush (SSE de /api/realtime) e o
// http.ResponseController por baixo.
type countingWriter struct {
	http.ResponseWriter
	n int64
}

func (w *countingWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.n += int64(n)
	return n, err
}

// Unwrap devolve o writer original (http.ResponseController / router.RWUnwrapper).
func (w *countingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// safely executa fn e engole qualquer panic: uma falha de métrica NUNCA pode
// virar 500 nem alterar a resposta (R8.10).
func safely(e *core.RequestEvent, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			e.App.Logger().Debug("métrica HTTP: recorder falhou (ignorado)", "panic", r)
		}
	}()
	fn()
}

// Instrument devolve o handler do middleware RAIZ que mede toda requisição
// (inclusive 404 de rota inexistente e /api/*). Registrado em api.go com
// Priority -5000 (abaixo do activity logger do PocketBase). Nunca altera o
// `err` devolvido por Next().
func Instrument(rec HTTPRecorder) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		start := time.Now()
		inflight := atorOfPath(e.Request.URL.Path)
		safely(e, func() { rec.InFlight(inflight, +1) })

		orig := e.Response
		cw := &countingWriter{ResponseWriter: orig}
		e.Response = cw

		err := e.Next()

		e.Response = orig // o PB/cleanup devem ver o writer original

		safely(e, func() {
			ator, rota := RouteLabels(e.Request.Pattern)

			// Status: com erro devolvido o PB o escreve DEPOIS da cadeia (e.Status()
			// ainda é 0); sem erro, o status já foi escrito.
			var status int
			bytes := cw.n
			if err != nil {
				status = router.ToApiError(err).Status
				bytes = -1 // o corpo do erro vai para o writer original: tamanho desconhecido
			} else {
				status = e.Status()
			}
			if status == 0 {
				status = http.StatusOK
			}

			rec.Request(ator, rota, metodoLabel(e.Request.Method), status, time.Since(start), bytes)

			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				motivo := DenyReason(e)
				if motivo == "" {
					if strings.HasPrefix(rota, "/api/") {
						motivo = DenyPocketBase
					} else {
						motivo = DenyOutro
					}
				}
				rec.Denied(ator, status, string(motivo))
			}
		})

		safely(e, func() { rec.InFlight(inflight, -1) })
		return err
	}
}
