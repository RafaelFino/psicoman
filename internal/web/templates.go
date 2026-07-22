package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed all:templates
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// TemplateRenderer manages template loading and rendering.
type TemplateRenderer struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

func NewTemplateRenderer() *TemplateRenderer {
	r := &TemplateRenderer{
		templates: make(map[string]*template.Template),
		funcMap:   defaultFuncMap(),
	}
	r.loadTemplates()
	return r
}

func defaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("15:04")
		},
		"formatDate": func(t time.Time) string {
			return t.Format("02/01/2006")
		},
		"formatDateTime": func(t time.Time) string {
			return t.Format("02/01/2006 15:04")
		},
		"formatBRL": func(cents int64) string {
			r := float64(cents) / 100
			return fmt.Sprintf("R$ %.2f", r)
		},
		"derefTime": func(t *time.Time) time.Time {
			if t == nil {
				return time.Time{}
			}
			return *t
		},
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
	}
}

func (r *TemplateRenderer) loadTemplates() {
	// Load layout files
	layoutFiles := []string{
		"templates/layouts/base.html",
		"templates/layouts/psych.html",
	}
	patientLayoutFiles := []string{
		"templates/layouts/base.html",
		"templates/layouts/patient.html",
	}

	// Load psych pages
	psychPages := []string{
		"dashboard", "patients", "patient_detail", "appointments",
		"session_notes", "anamnesis", "contracts", "finance",
		"supervisions", "spaces", "settings",
	}
	for _, page := range psychPages {
		path := fmt.Sprintf("templates/psych/%s.html", page)
		content, err := fs.ReadFile(templatesFS, path)
		if err != nil {
			continue // Template not yet created
		}
		t := template.New(page).Funcs(r.funcMap)
		for _, lf := range layoutFiles {
			lContent, _ := fs.ReadFile(templatesFS, lf)
			template.Must(t.Parse(string(lContent)))
		}
		template.Must(t.Parse(string(content)))
		r.templates["psych/"+page] = t
	}

	// Load patient pages
	patientPages := []string{
		"login", "dashboard", "book", "anamnesis", "contracts", "documents",
	}
	for _, page := range patientPages {
		path := fmt.Sprintf("templates/patient/%s.html", page)
		content, err := fs.ReadFile(templatesFS, path)
		if err != nil {
			continue
		}
		t := template.New(page).Funcs(r.funcMap)
		for _, lf := range patientLayoutFiles {
			lContent, _ := fs.ReadFile(templatesFS, lf)
			template.Must(t.Parse(string(lContent)))
		}
		template.Must(t.Parse(string(content)))
		r.templates["patient/"+page] = t
	}
}

// Render executes the named template with the "psych" or "patient" layout.
func (r *TemplateRenderer) Render(w io.Writer, name string, data any) error {
	t, ok := r.templates[name]
	if !ok {
		return fmt.Errorf("template %q not found", name)
	}
	// Determine layout: psych or patient based on prefix
	layout := "psych"
	if strings.HasPrefix(name, "patient/") {
		layout = "patient"
	}
	return t.ExecuteTemplate(w, layout, data)
}

// RenderPage is a Gin helper that renders a template to the response.
func (r *TemplateRenderer) RenderPage(c *gin.Context, status int, name string, data any) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	if err := r.Render(c.Writer, name, data); err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
	}
}

// ServeStatic sets up the static file server from embedded assets.
func ServeStatic(r *gin.Engine) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return
	}
	fileServer := http.FileServer(http.FS(sub))
	r.GET("/static/*filepath", func(c *gin.Context) {
		c.Request.URL.Path = c.Param("filepath")
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}

// PageData is the base struct for all page renders.
type PageData struct {
	ActiveNav string
	Today     string
}

func basePsychData(nav string) PageData {
	return PageData{
		ActiveNav: nav,
		Today:     time.Now().Format("02/01/2006 · Monday"),
	}
}

func basePatientData(nav string) PageData {
	return PageData{
		ActiveNav: nav,
		Today:     time.Now().Format("02/01/2006"),
	}
}
