package google

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

// DriveScope é o escopo OAuth MÍNIMO para o Drive: drive.file restringe o acesso
// aos arquivos CRIADOS pela própria aplicação — nunca o Drive pessoal inteiro
// (psicoman-google-api.md: jamais `drive` ou `drive.readonly` full).
const DriveScope = "https://www.googleapis.com/auth/drive.file"

// folderMimeType é o MIME de uma pasta no Google Drive.
const folderMimeType = "application/vnd.google-apps.folder"

// ErrDriveNotFound é o sentinela para "o Drive respondeu 404" (arquivo/pasta
// inexistente ou fora do escopo drive.file). Mapeado a partir de 404 em Upload
// e GetFolder. O drivesync NÃO importa este pacote, então o adapter do wiring
// traduz este sentinela para drivesync.ErrFolderNotFound.
var ErrDriveNotFound = errors.New("drive: recurso não encontrado (404)")

// driveUploadURL é a base de upload da Drive API v3 (uploadType=multipart).
// driveFilesURL é a base de metadados (files.list/get/create/update).
// Variáveis de pacote para apontar ao servidor fake nos testes.
var (
	driveUploadURL = "https://www.googleapis.com/upload/drive/v3/files"
	driveFilesURL  = "https://www.googleapis.com/drive/v3/files"
)

// DriveUploadInput descreve um upload para o Drive: nome do arquivo, pasta
// destino (folder id; vazio = raiz do escopo drive.file), MIME, conteúdo e
// appProperties (gravadas como appProperties no metadata; ex.: psicoman_kind).
type DriveUploadInput struct {
	Name     string
	FolderID string
	MIME     string
	Content  []byte
	AppProps map[string]string
}

// DriveUploadResult é o resultado do upload (id do arquivo no Drive).
type DriveUploadResult struct {
	FileID string
}

// DriveFile é a visão mínima de um arquivo no Drive.
type DriveFile struct {
	ID          string
	Name        string
	CreatedTime time.Time
}

// DriveListInput filtra arquivos criados pelo app dentro de uma pasta.
type DriveListInput struct {
	FolderID string
	AppProps map[string]string // ex.: {"psicoman_kind":"snapshot"}
}

// DriveClient é a interface LOCAL do cliente de Drive (psicoman-google-api.md:
// integração Google atrás de interface, com fake nos testes). O Drive é
// fallback/DR, fora do caminho de request — usado só pelo hook de backup
// (internal/drivesync). Todo método respeita o ctx recebido.
type DriveClient interface {
	// Upload envia o conteúdo como um arquivo novo no Drive e devolve o id.
	// 404 do Google (pasta pai inexistente) vira ErrDriveNotFound.
	Upload(ctx context.Context, in DriveUploadInput) (*DriveUploadResult, error)
	// List devolve os arquivos não-lixeira da pasta que casam AppProps, mais
	// recentes primeiro (createdTime desc).
	List(ctx context.Context, in DriveListInput) ([]DriveFile, error)
	// Trash move o arquivo para a lixeira (reversível por 30 dias).
	Trash(ctx context.Context, fileID string) error
	// FindFolder busca uma pasta pelo nome + appProperties (só o que o app criou).
	FindFolder(ctx context.Context, name string, appProps map[string]string) (id string, found bool, err error)
	// CreateFolder cria uma pasta com o nome e as appProperties dadas.
	CreateFolder(ctx context.Context, name string, appProps map[string]string) (id string, err error)
	// GetFolder revalida uma pasta por id (files.get?fields=id,trashed). 404 →
	// ErrDriveNotFound.
	GetFolder(ctx context.Context, id string) (trashed bool, err error)
}

// restDriveClient fala com a Drive API v3 (upload multipart) sobre um httpDoer
// que carrega a credencial OAuth (escopo drive.file). Respeita o ctx recebido.
type restDriveClient struct {
	http      httpDoer
	uploadURL string
	filesURL  string
}

var _ DriveClient = (*restDriveClient)(nil)

// NewRESTDriveClient cria o cliente REST de Drive. O doer DEVE carregar a
// credencial OAuth (drive.file); o wiring degrada antes quando ausente.
func NewRESTDriveClient(doer httpDoer) DriveClient {
	return &restDriveClient{http: doer, uploadURL: driveUploadURL, filesURL: driveFilesURL}
}

// Upload faz um upload multipart (metadata JSON + bytes) para a Drive API v3.
// Quando FolderID != "", o arquivo é criado dentro dela (parents). Respeita o
// ctx; status >= 400 vira erro (sem vazar o corpo bruto ao chamador).
func (c *restDriveClient) Upload(ctx context.Context, in DriveUploadInput) (*DriveUploadResult, error) {
	meta := map[string]any{"name": in.Name}
	if strings.TrimSpace(in.FolderID) != "" {
		meta["parents"] = []string{in.FolderID}
	}
	if len(in.AppProps) > 0 {
		meta["appProperties"] = in.AppProps
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
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrDriveNotFound
	}
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
// API real do Google. Registra os uploads e devolve ids determinísticos.
type FakeDriveClient struct {
	FileID     string
	UploadErr  error
	Uploads    []DriveUploadInput
	UploadCall int

	// Pasta: comportamento controlável por teste.
	FolderID         string // id devolvido por CreateFolder/FindFolder
	FolderFound      bool   // FindFolder encontra?
	CreateFolderErr  error
	FindFolderErr    error
	GetFolderErr     error // ex.: ErrDriveNotFound
	FolderTrashed    bool  // retorno de GetFolder
	CreateFolderCall int
	FindFolderCall   int
	GetFolderCall    int

	// Listagem / lixeira.
	Files    []DriveFile
	ListErr  error
	TrashErr error
	Trashed  []string // ids enviados para a lixeira
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

// List devolve os arquivos registrados (ou o erro).
func (f *FakeDriveClient) List(_ context.Context, _ DriveListInput) ([]DriveFile, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	return f.Files, nil
}

// Trash registra o id enviado para a lixeira (ou devolve o erro).
func (f *FakeDriveClient) Trash(_ context.Context, fileID string) error {
	if f.TrashErr != nil {
		return f.TrashErr
	}
	f.Trashed = append(f.Trashed, fileID)
	return nil
}

// FindFolder devolve FolderID/FolderFound (ou o erro).
func (f *FakeDriveClient) FindFolder(_ context.Context, _ string, _ map[string]string) (string, bool, error) {
	f.FindFolderCall++
	if f.FindFolderErr != nil {
		return "", false, f.FindFolderErr
	}
	if !f.FolderFound {
		return "", false, nil
	}
	return f.FolderID, true, nil
}

// CreateFolder devolve FolderID (ou o erro) e conta a chamada.
func (f *FakeDriveClient) CreateFolder(_ context.Context, _ string, _ map[string]string) (string, error) {
	f.CreateFolderCall++
	if f.CreateFolderErr != nil {
		return "", f.CreateFolderErr
	}
	id := f.FolderID
	if id == "" {
		id = "folder-fake"
	}
	return id, nil
}

// GetFolder devolve FolderTrashed (ou o erro, ex.: ErrDriveNotFound).
func (f *FakeDriveClient) GetFolder(_ context.Context, _ string) (bool, error) {
	f.GetFolderCall++
	if f.GetFolderErr != nil {
		return false, f.GetFolderErr
	}
	return f.FolderTrashed, nil
}
