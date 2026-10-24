package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// DriveScope é o escopo OAuth MÍNIMO para o Drive: drive.file restringe o acesso
// aos arquivos CRIADOS pela própria aplicação — nunca o Drive pessoal inteiro
// (psicoman-google-api.md: jamais `drive` ou `drive.readonly` full).
const DriveScope = "https://www.googleapis.com/auth/drive.file"

// driveUploadURL é a base de upload da Drive API v3 (uploadType=multipart).
// Variável de pacote para apontar ao servidor fake nos testes.
var driveUploadURL = "https://www.googleapis.com/upload/drive/v3/files"

// DriveUploadInput descreve um upload para o Drive: nome do arquivo, pasta
// destino (folder id; vazio = raiz do escopo drive.file), MIME e conteúdo.
type DriveUploadInput struct {
	Name     string
	FolderID string
	MIME     string
	Content  []byte
}

// DriveUploadResult é o resultado do upload (id do arquivo no Drive).
type DriveUploadResult struct {
	FileID string
}

// DriveClient é a interface LOCAL do cliente de Drive (psicoman-google-api.md:
// integração Google atrás de interface, com fake nos testes). O Drive é
// fallback/DR, fora do caminho de request — usado só pelo hook de backup
// (internal/drivesync). Todo método respeita o ctx recebido.
type DriveClient interface {
	// Upload envia o conteúdo como um arquivo novo no Drive e devolve o id.
	Upload(ctx context.Context, in DriveUploadInput) (*DriveUploadResult, error)
}

// restDriveClient fala com a Drive API v3 (upload multipart) sobre um httpDoer
// que carrega a credencial OAuth (escopo drive.file). Respeita o ctx recebido.
type restDriveClient struct {
	http      httpDoer
	uploadURL string
}

var _ DriveClient = (*restDriveClient)(nil)

// NewRESTDriveClient cria o cliente REST de Drive. O doer DEVE carregar a
// credencial OAuth (drive.file); o wiring degrada antes quando ausente.
func NewRESTDriveClient(doer httpDoer) DriveClient {
	return &restDriveClient{http: doer, uploadURL: driveUploadURL}
}

// Upload faz um upload multipart (metadata JSON + bytes) para a Drive API v3.
// Quando FolderID != "", o arquivo é criado dentro dela (parents). Respeita o
// ctx; status >= 400 vira erro (sem vazar o corpo bruto ao chamador).
func (c *restDriveClient) Upload(ctx context.Context, in DriveUploadInput) (*DriveUploadResult, error) {
	meta := map[string]any{"name": in.Name}
	if strings.TrimSpace(in.FolderID) != "" {
		meta["parents"] = []string{in.FolderID}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("drive: serializar metadata: %w", err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	// Parte 1: metadata (application/json).
	metaPart, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type": []string{"application/json; charset=UTF-8"},
	})
	if err != nil {
		return nil, fmt.Errorf("drive: parte de metadata: %w", err)
	}
	if _, err := metaPart.Write(metaJSON); err != nil {
		return nil, fmt.Errorf("drive: escrever metadata: %w", err)
	}

	// Parte 2: conteúdo (MIME informado; default octet-stream).
	contentType := in.MIME
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	dataPart, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type": []string{contentType},
	})
	if err != nil {
		return nil, fmt.Errorf("drive: parte de conteúdo: %w", err)
	}
	if _, err := dataPart.Write(in.Content); err != nil {
		return nil, fmt.Errorf("drive: escrever conteúdo: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("drive: fechar multipart: %w", err)
	}

	url := c.uploadURL + "?uploadType=multipart"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("drive: montar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "multipart/related; boundary="+mw.Boundary())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("drive: chamada ao Google: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("drive: Google respondeu %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("drive: decodificar resposta: %w", err)
		}
	}
	return &DriveUploadResult{FileID: out.ID}, nil
}

// FakeDriveClient é um DriveClient em memória para testes — NENHUM teste toca a
// API real do Google. Registra os uploads e devolve um id determinístico.
type FakeDriveClient struct {
	FileID     string
	UploadErr  error
	Uploads    []DriveUploadInput
	UploadCall int
}

var _ DriveClient = (*FakeDriveClient)(nil)

// Upload registra o input e devolve o resultado determinístico (ou o erro).
func (f *FakeDriveClient) Upload(_ context.Context, in DriveUploadInput) (*DriveUploadResult, error) {
	f.UploadCall++
	f.Uploads = append(f.Uploads, in)
	if f.UploadErr != nil {
		return nil, f.UploadErr
	}
	id := f.FileID
	if id == "" {
		id = "drive-file-fake"
	}
	return &DriveUploadResult{FileID: id}, nil
}
