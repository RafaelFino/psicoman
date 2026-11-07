package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Teto de paginação da List: no máximo 10 páginas de 100 (1000 arquivos). Um
// acervo de snapshots com retenção pequena nunca se aproxima disso; o teto
// protege contra laço acidental.
const (
	driveListPageSize = 100
	driveListMaxPages = 10
)

// appPropsQuery monta os termos `appProperties has {...}` da query files.list.
func appPropsQuery(appProps map[string]string) string {
	var b strings.Builder
	for k, v := range appProps {
		fmt.Fprintf(&b, " and appProperties has {key='%s' and value='%s'}", k, v)
	}
	return b.String()
}

// List devolve os snapshots não-lixeira da pasta que casam AppProps, mais
// recentes primeiro. Pagina por pageToken com teto de segurança.
func (c *restDriveClient) List(ctx context.Context, in DriveListInput) ([]DriveFile, error) {
	q := fmt.Sprintf("'%s' in parents and trashed=false", in.FolderID) + appPropsQuery(in.AppProps)

	var out []DriveFile
	pageToken := ""
	for page := 0; page < driveListMaxPages; page++ {
		vals := url.Values{}
		vals.Set("q", q)
		vals.Set("orderBy", "createdTime desc")
		vals.Set("pageSize", fmt.Sprintf("%d", driveListPageSize))
		vals.Set("fields", "nextPageToken,files(id,name,createdTime)")
		if pageToken != "" {
			vals.Set("pageToken", pageToken)
		}

		var body struct {
			NextPageToken string `json:"nextPageToken"`
			Files         []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				CreatedTime string `json:"createdTime"`
			} `json:"files"`
		}
		if err := c.getJSON(ctx, c.filesURL+"?"+vals.Encode(), &body); err != nil {
			return nil, err
		}
		for _, f := range body.Files {
			out = append(out, DriveFile{ID: f.ID, Name: f.Name, CreatedTime: parseDriveTime(f.CreatedTime)})
		}
		if body.NextPageToken == "" {
			break
		}
		pageToken = body.NextPageToken
	}
	return out, nil
}

// Trash move o arquivo para a lixeira (PATCH files/{id} {"trashed":true}).
func (c *restDriveClient) Trash(ctx context.Context, fileID string) error {
	payload, _ := json.Marshal(map[string]any{"trashed": true})
	u := c.filesURL + "/" + url.PathEscape(fileID)
	return c.sendJSON(ctx, http.MethodPatch, u, bytes.NewReader(payload), nil)
}

// FindFolder busca uma pasta pelo nome + appProperties (só o que o app criou).
func (c *restDriveClient) FindFolder(ctx context.Context, name string, appProps map[string]string) (string, bool, error) {
	q := fmt.Sprintf("mimeType='%s' and trashed=false and name='%s'", folderMimeType, name) + appPropsQuery(appProps)
	vals := url.Values{}
	vals.Set("q", q)
	vals.Set("pageSize", "1")
	vals.Set("fields", "files(id)")

	var body struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	if err := c.getJSON(ctx, c.filesURL+"?"+vals.Encode(), &body); err != nil {
		return "", false, err
	}
	if len(body.Files) == 0 {
		return "", false, nil
	}
	return body.Files[0].ID, true, nil
}

// CreateFolder cria uma pasta (files.create com mimeType de pasta).
func (c *restDriveClient) CreateFolder(ctx context.Context, name string, appProps map[string]string) (string, error) {
	meta := map[string]any{"name": name, "mimeType": folderMimeType}
	if len(appProps) > 0 {
		meta["appProperties"] = appProps
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("drive: serializar pasta: %w", err)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.sendJSON(ctx, http.MethodPost, c.filesURL, bytes.NewReader(payload), &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// GetFolder revalida uma pasta por id. 404 → ErrDriveNotFound.
func (c *restDriveClient) GetFolder(ctx context.Context, id string) (bool, error) {
	vals := url.Values{}
	vals.Set("fields", "id,trashed")
	u := c.filesURL + "/" + url.PathEscape(id) + "?" + vals.Encode()
	var out struct {
		Trashed bool `json:"trashed"`
	}
	if err := c.getJSON(ctx, u, &out); err != nil {
		return false, err
	}
	return out.Trashed, nil
}

// getJSON faz um GET autenticado e decodifica o corpo em out. 404 → ErrDriveNotFound.
func (c *restDriveClient) getJSON(ctx context.Context, u string, out any) error {
	return c.sendJSON(ctx, http.MethodGet, u, nil, out)
}

// sendJSON faz uma requisição (GET/POST/PATCH) com corpo JSON opcional e
// decodifica a resposta em out (se != nil). 404 → ErrDriveNotFound; >= 400 vira
// erro sem vazar o corpo bruto.
func (c *restDriveClient) sendJSON(ctx context.Context, method, u string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return fmt.Errorf("drive: montar requisição: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("drive: chamada ao Google: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return ErrDriveNotFound
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("drive: Google respondeu %d", resp.StatusCode)
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("drive: decodificar resposta: %w", err)
		}
	}
	return nil
}

// parseDriveTime lê o createdTime (RFC3339) do Drive; zero em erro.
func parseDriveTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
