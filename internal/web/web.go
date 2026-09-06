// Package web serve a interface HTML dos dois binários (admin e portal),
// com templates Go e assets embutidos no binário via embed.FS
// (docs/architecture.md §9; psicoman-web-responsivo.md).
//
// SSR sem SPA: HTML renderizado no servidor, CSS mobile-first leve e htmx/Alpine
// via CDN para interatividade pontual. WCAG AA (labels, foco visível, contraste).
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html static/*
var content embed.FS

// Surface identifica qual interface renderizar.
type Surface string

const (
	// Admin é a interface do terapeuta.
	Admin Surface = "admin"
	// Portal é a interface do paciente.
	Portal Surface = "portal"
)

// Server renderiza as páginas de uma superfície.
type Server struct {
	surface Surface
	tmpl    *template.Template
	static  http.Handler
}

// New cria o servidor web para a superfície.
func New(surface Surface) (*Server, error) {
	tmpl, err := template.ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, err
	}
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		return nil, err
	}
	return &Server{
		surface: surface,
		tmpl:    tmpl,
		static:  http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))),
	}, nil
}

// Register instala as rotas de UI e assets no mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.Handle("GET /static/", s.static)
	mux.HandleFunc("GET /{$}", s.index)
	if s.surface == Admin {
		mux.HandleFunc("GET /app/", s.adminApp)
	} else {
		mux.HandleFunc("GET /app/", s.portalApp)
	}
}

type pageData struct {
	Title   string
	Surface string
}

func (s *Server) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "erro ao renderizar a página", http.StatusInternalServerError)
	}
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	if s.surface == Admin {
		s.render(w, "admin_home.html", pageData{Title: "Administração", Surface: "admin"})
		return
	}
	s.render(w, "portal_home.html", pageData{Title: "Portal do Paciente", Surface: "portal"})
}

func (s *Server) adminApp(w http.ResponseWriter, _ *http.Request) {
	s.render(w, "admin_app.html", pageData{Title: "Painel", Surface: "admin-app"})
}

func (s *Server) portalApp(w http.ResponseWriter, _ *http.Request) {
	s.render(w, "portal_app.html", pageData{Title: "Minha área", Surface: "portal-app"})
}
