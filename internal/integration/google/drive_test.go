package google

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newFakeDriveServer(t *testing.T, h http.HandlerFunc) DriveClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &restDriveClient{http: srv.Client(), uploadURL: srv.URL}
}

func TestRESTDriveUpload_MultipartComPasta(t *testing.T) {
	var gotUploadType, gotContentType string
	var gotBody string
	c := newFakeDriveServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUploadType = r.URL.Query().Get("uploadType")
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "file-xyz"})
	})
	out, err := c.Upload(context.Background(), DriveUploadInput{
		Name:     "backup.enc",
		FolderID: "folder-123",
		MIME:     "application/octet-stream",
		Content:  []byte("conteúdo-cifrado"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.FileID != "file-xyz" {
		t.Fatalf("id inesperado: %q", out.FileID)
	}
	if gotUploadType != "multipart" {
		t.Errorf("uploadType esperado multipart, veio %q", gotUploadType)
	}
	if !strings.HasPrefix(gotContentType, "multipart/related") {
		t.Errorf("Content-Type esperado multipart/related, veio %q", gotContentType)
	}
	// metadata deve conter o nome e a pasta (parents).
	if !strings.Contains(gotBody, `"name":"backup.enc"`) {
		t.Error("metadata sem o nome do arquivo")
	}
	if !strings.Contains(gotBody, `"parents":["folder-123"]`) {
		t.Error("metadata sem a pasta destino (parents)")
	}
	if !strings.Contains(gotBody, "conteúdo-cifrado") {
		t.Error("corpo sem o conteúdo enviado")
	}
}

func TestRESTDriveUpload_SemPasta(t *testing.T) {
	var gotBody string
	c := newFakeDriveServer(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "f1"})
	})
	if _, err := c.Upload(context.Background(), DriveUploadInput{Name: "x", Content: []byte("y")}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gotBody, "parents") {
		t.Error("sem FolderID não deveria enviar parents")
	}
}

func TestRESTDriveUpload_ErroHTTP(t *testing.T) {
	c := newFakeDriveServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	if _, err := c.Upload(context.Background(), DriveUploadInput{Name: "x", Content: []byte("y")}); err == nil {
		t.Fatal("status 403 deveria virar erro")
	}
}

func TestRESTDriveUpload_RespeitaContext(t *testing.T) {
	c := newFakeDriveServer(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Upload(ctx, DriveUploadInput{Name: "x", Content: []byte("y")}); err == nil {
		t.Fatal("ctx expirado deveria abortar o upload")
	}
}

func TestFakeDrive_RegistraUpload(t *testing.T) {
	f := &FakeDriveClient{FileID: "fake-1"}
	out, err := f.Upload(context.Background(), DriveUploadInput{Name: "snap", Content: []byte("z")})
	if err != nil {
		t.Fatal(err)
	}
	if out.FileID != "fake-1" || f.UploadCall != 1 || len(f.Uploads) != 1 {
		t.Fatalf("fake não registrou o upload: %+v", f)
	}
	if f.Uploads[0].Name != "snap" {
		t.Errorf("input não registrado: %+v", f.Uploads[0])
	}
}
