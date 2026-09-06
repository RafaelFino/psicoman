package admin

import (
	"bytes"
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// GEDHandlers expõe upload/download/listagem de arquivos do GED por paciente.
type GEDHandlers struct {
	svc     *service.GEDService
	patient *service.PatientService
	audit   *service.AuditService
}

// NewGEDHandlers cria os handlers do GED.
func NewGEDHandlers(svc *service.GEDService, patient *service.PatientService, audit *service.AuditService) *GEDHandlers {
	return &GEDHandlers{svc: svc, patient: patient, audit: audit}
}

// Register instala as rotas do GED no grupo autenticado.
func (h *GEDHandlers) Register(g *api.Group) {
	g.Handle("POST", "/patients/{id}/files", h.upload)
	g.Handle("GET", "/patients/{id}/files", h.list)
	g.Handle("GET", "/files/{fid}", h.download)
}

func gedView(f *domain.GEDFile) map[string]any {
	return map[string]any{
		"id":         f.ID,
		"patient_id": f.PatientID,
		"mime":       f.MIME,
		"size":       f.Size,
		"sha256":     f.SHA256,
		"created_at": f.CreatedAt,
	}
}

// upload recebe um arquivo multipart (campo "file") e o anexa ao paciente.
func (h *GEDHandlers) upload(w http.ResponseWriter, r *http.Request) {
	patientID := r.PathValue("id")
	// Garante que o paciente existe (segregação por paciente).
	if _, err := h.patient.Get(r.Context(), patientID); err != nil {
		respondServiceError(w, r, err)
		return
	}

	const maxUpload = 25 << 20 // 25 MiB
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Não foi possível ler o arquivo enviado."))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Envie um arquivo no campo 'file'."))
		return
	}
	defer file.Close()

	mime := header.Header.Get("Content-Type")
	f, err := h.svc.Store(r.Context(), service.GEDLink{PatientID: patientID}, mime, file)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionCreate, "ged_file", f.ID,
		map[string]any{"patient_id": patientID})
	httpx.Respond(w, r, http.StatusCreated, "Arquivo anexado.", gedView(f))
}

func (h *GEDHandlers) list(w http.ResponseWriter, r *http.Request) {
	fs, err := h.svc.ListByPatient(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(fs))
	for _, f := range fs {
		views = append(views, gedView(f))
	}
	httpx.Respond(w, r, http.StatusOK, "Arquivos listados.", views)
}

// download serve o conteúdo bruto do arquivo, validando integridade.
func (h *GEDHandlers) download(w http.ResponseWriter, r *http.Request) {
	f, data, err := h.svc.Read(r.Context(), r.PathValue("fid"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionRead, "ged_file", f.ID, nil)
	ct := f.MIME
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	http.ServeContent(w, r, f.ID, f.CreatedAt, bytes.NewReader(data))
}
