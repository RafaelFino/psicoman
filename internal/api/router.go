package api

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
)

// Router organiza o registro de rotas versionadas (/v1) com namespaces
// distintos para admin e portal (docs/architecture.md §5).
type Router struct {
	mux    *http.ServeMux
	prefix string
}

// V1 devolve um Router com prefixo /v1 sobre o mux informado.
func V1(mux *http.ServeMux) *Router {
	return &Router{mux: mux, prefix: "/v1"}
}

// Admin devolve um sub-router no namespace /v1/admin.
func (r *Router) Admin() *Group {
	return &Group{mux: r.mux, prefix: r.prefix + "/admin"}
}

// Portal devolve um sub-router no namespace /v1/portal.
func (r *Router) Portal() *Group {
	return &Group{mux: r.mux, prefix: r.prefix + "/portal"}
}

// Group registra handlers sob um prefixo, com auth opcional aplicada a todos.
type Group struct {
	mux    *http.ServeMux
	prefix string
	auth   func(http.Handler) http.Handler
}

// WithAuth aplica um middleware de autenticação a todas as rotas do grupo.
func (g *Group) WithAuth(mw func(http.Handler) http.Handler) *Group {
	g.auth = mw
	return g
}

// Handle registra "METHOD /caminho" relativo ao prefixo do grupo.
// O method é obrigatório (ex: "GET", "POST").
func (g *Group) Handle(method, path string, h http.HandlerFunc) {
	full := method + " " + g.prefix + path
	var handler http.Handler = h
	if g.auth != nil {
		handler = g.auth(handler)
	}
	g.mux.Handle(full, handler)
}

// Prefix expõe o prefixo do grupo (ex: "/v1/admin").
func (g *Group) Prefix() string { return g.prefix }

// RegisterVersion registra GET {prefix}/version, útil para validar o envelope
// e o versionamento da API sem autenticação de negócio.
func RegisterVersion(g *Group, surface string) {
	g.Handle("GET", "/version", func(w http.ResponseWriter, req *http.Request) {
		httpx.Respond(w, req, http.StatusOK, "Psicoman API", map[string]any{
			"api_version": "v1",
			"surface":     surface,
		})
	})
}
