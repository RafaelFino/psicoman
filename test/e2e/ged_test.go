package e2e

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

// uploadFile envia um arquivo multipart autenticado como admin.
func (e *Env) uploadFile(t *testing.T, path, filename string, content []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("multipart: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("escrevendo parte: %v", err)
	}
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, e.BaseURL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Pangolin-Email", testAdminEmail)
	req.Header.Set("X-Pangolin-Secret", testAdminSecret)
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	return resp
}

func TestGEDUploadDownloadDedup(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Cria paciente.
	resp := env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Paciente GED", "phone": "1199", "email": "ged@example.com",
	})
	pe := DecodeEnvelope(t, resp)
	var p struct {
		ID string `json:"id"`
	}
	pe.DataAs(t, &p)

	content := []byte("relatório clínico confidencial")

	// Upload.
	resp = env.uploadFile(t, "/v1/admin/patients/"+p.ID+"/files", "laudo.txt", content)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, quer 201", resp.StatusCode)
	}
	fe := DecodeEnvelope(t, resp)
	var f struct {
		ID     string `json:"id"`
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	}
	fe.DataAs(t, &f)
	if f.ID == "" || f.SHA256 == "" {
		t.Fatal("upload sem id/sha")
	}

	// Download recupera íntegro.
	resp = env.AdminGET(t, "/v1/admin/files/"+f.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d, quer 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.Equal(got, content) {
		t.Error("conteúdo baixado difere do enviado")
	}

	// Dedup: reenviar o mesmo conteúdo devolve o mesmo id.
	resp = env.uploadFile(t, "/v1/admin/patients/"+p.ID+"/files", "laudo-copia.txt", content)
	de := DecodeEnvelope(t, resp)
	var f2 struct {
		ID string `json:"id"`
	}
	de.DataAs(t, &f2)
	if f2.ID != f.ID {
		t.Errorf("dedup falhou: id %q != %q", f2.ID, f.ID)
	}

	// Listar traz 1 arquivo (dedup não duplica).
	resp = env.AdminGET(t, "/v1/admin/patients/"+p.ID+"/files")
	le := DecodeEnvelope(t, resp)
	var list []map[string]any
	le.DataAs(t, &list)
	if len(list) != 1 {
		t.Errorf("arquivos = %d, quer 1 (dedup)", len(list))
	}
}

func TestGEDUploadRequiresExistingPatient(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	resp := env.uploadFile(t, "/v1/admin/patients/inexistente/files", "x.txt", []byte("x"))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, quer 404", resp.StatusCode)
	}
	resp.Body.Close()
}
