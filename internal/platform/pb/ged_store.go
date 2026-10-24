package pb

import (
	"context"
	"fmt"
	"io"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
)

// gedBlobStore implementa app.GEDBlobStore sobre o file storage nativo do
// PocketBase (app.NewFilesystem() — local ou S3 conforme a config do PB, D9b).
// É o ÚNICO ponto que toca o storage de binários do GED; o metadado fica no
// gedMetaRepo. O relPath do domínio é usado como fileKey do bucket.
//
// Cada operação abre um System próprio e aplica o ctx recebido (SetContext),
// respeitando o timeout do chamador (psicoman-golang.md: todo I/O com timeout).
type gedBlobStore struct{ app core.App }

var _ app.GEDBlobStore = (*gedBlobStore)(nil)

// NewGEDBlobStore cria o adapter de storage de binários do GED.
func NewGEDBlobStore(a core.App) app.GEDBlobStore { return &gedBlobStore{app: a} }

// Put grava os bytes sob relPath (fileKey). O Upload do PocketBase detecta o
// MIME a partir do conteúdo; mime é aceito na assinatura para fins de contrato
// e auditoria, mas o storage usa a detecção nativa (evita divergência).
func (s *gedBlobStore) Put(ctx context.Context, relPath, _ string, content []byte) error {
	fs, err := s.app.NewFilesystem()
	if err != nil {
		return fmt.Errorf("ged: abrir file storage: %w", err)
	}
	defer fs.Close()
	fs.SetContext(ctx)
	if err := fs.Upload(content, relPath); err != nil {
		return fmt.Errorf("ged: gravar arquivo: %w", err)
	}
	return nil
}

// Get devolve os bytes gravados sob relPath (fileKey).
func (s *gedBlobStore) Get(ctx context.Context, relPath string) ([]byte, error) {
	fs, err := s.app.NewFilesystem()
	if err != nil {
		return nil, fmt.Errorf("ged: abrir file storage: %w", err)
	}
	defer fs.Close()
	fs.SetContext(ctx)
	r, err := fs.GetReader(relPath)
	if err != nil {
		return nil, fmt.Errorf("ged: ler arquivo: %w", err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("ged: ler conteúdo: %w", err)
	}
	return data, nil
}
