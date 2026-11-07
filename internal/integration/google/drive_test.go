package google

import (
	"context"
	"encoding/json"
	"errors"
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
	// uploadURL e filesURL apontam ao mesmo servidor fake; o handler distingue
	// pela rota/método.
	return &restDriveClient{http: srv.Client(), uploadURL: srv.URL + "/upload", filesURL: srv.URL + "/files"}
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

func TestRESTDriveUpload_AppProperties(t *testing.T) {
	var gotBody string
	c := newFakeDriveServer(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "f1"})
	})
	if _, err := c.Upload(context.Background(), DriveUploadInput{
		Name:     "snap.enc",
		Content:  []byte("x"),
		AppProps: map[string]string{"psicoman_kind": "snapshot"},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, `"appProperties"`) || !strings.Contains(gotBody, `"psicoman_kind":"snapshot"`) {
		t.Errorf("metadata sem appProperties: %s", gotBody)
	}
}

func TestRESTDriveUpload_404ViraErrDriveNotFound(t *testing.T) {
	c := newFakeDriveServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.Upload(context.Background(), DriveUploadInput{Name: "x", FolderID: "sumiu", Content: []byte("y")})
	if !errors.Is(err, ErrDriveNotFound) {
		t.Fatalf("404 no upload deveria virar ErrDriveNotFound, veio %v", err)
	}
}

func TestRESTDriveCreateFolder(t *testing.T) {
	var gotBody string
	c := newFakeDriveServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("CreateFolder deveria ser POST, veio %s", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "folder-novo"})
	})
	id, err := c.CreateFolder(context.Background(), "Psicoman DR", map[string]string{"psicoman_role": "dr_folder"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "folder-novo" {
		t.Errorf("id esperado folder-novo, veio %q", id)
	}
	if !strings.Contains(gotBody, `"mimeType":"application/vnd.google-apps.folder"`) {
		t.Errorf("metadata sem o mimeType de pasta: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"psicoman_role":"dr_folder"`) {
		t.Errorf("metadata sem appProperties: %s", gotBody)
	}
}

func TestRESTDriveFindFolder(t *testing.T) {
	// Acha.
	c := newFakeDriveServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "q=") {
			t.Error("FindFolder deveria mandar a query q")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"files": []map[string]any{{"id": "achei"}}})
	})
	id, found, err := c.FindFolder(context.Background(), "Psicoman DR", nil)
	if err != nil || !found || id != "achei" {
		t.Fatalf("esperava achar folder, veio id=%q found=%v err=%v", id, found, err)
	}

	// Não acha (lista vazia).
	c2 := newFakeDriveServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"files": []map[string]any{}})
	})
	if _, found, err := c2.FindFolder(context.Background(), "x", nil); err != nil || found {
		t.Fatalf("esperava não achar, veio found=%v err=%v", found, err)
	}
}

func TestRESTDriveGetFolder_404(t *testing.T) {
	c := newFakeDriveServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := c.GetFolder(context.Background(), "sumiu"); !errors.Is(err, ErrDriveNotFound) {
		t.Fatalf("404 no GetFolder deveria virar ErrDriveNotFound, veio %v", err)
	}

	// trashed=true é lido corretamente.
	c2 := newFakeDriveServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "x", "trashed": true})
	})
	trashed, err := c2.GetFolder(context.Background(), "x")
	if err != nil || !trashed {
		t.Fatalf("esperava trashed=true, veio %v err=%v", trashed, err)
	}
}

func TestRESTDriveList(t *testing.T) {
	c := newFakeDriveServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "trashed=false") || !strings.Contains(q, "psicoman_kind") {
			t.Errorf("query inesperada: %q", q)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"files": []map[string]any{
				{"id": "a", "name": "s1.enc", "createdTime": "2026-11-05T10:00:00Z"},
				{"id": "b", "name": "s2.enc", "createdTime": "2026-11-04T10:00:00Z"},
			},
		})
	})
	files, err := c.List(context.Background(), DriveListInput{
		FolderID: "folder-1",
		AppProps: map[string]string{"psicoman_kind": "snapshot"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].ID != "a" || files[0].Name != "s1.enc" {
		t.Fatalf("listagem inesperada: %+v", files)
	}
	if files[0].CreatedTime.IsZero() {
		t.Error("createdTime não foi parseado")
	}
}

func TestRESTDriveTrash(t *testing.T) {
	var gotMethod, gotBody string
	c := newFakeDriveServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.WriteHeader(http.StatusOK)
	})
	if err := c.Trash(context.Background(), "file-x"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("Trash deveria ser PATCH, veio %s", gotMethod)
	}
	if !strings.Contains(gotBody, `"trashed":true`) {
		t.Errorf("Trash deveria mandar trashed:true, veio %s", gotBody)
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
