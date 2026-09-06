package service

import (
	"bytes"
	"context"
	"io"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// GEDMetaRepository persiste os metadados dos arquivos do GED.
type GEDMetaRepository interface {
	Insert(ctx context.Context, f *domain.GEDFile) error
	Get(ctx context.Context, id string) (*domain.GEDFile, error)
	// FindByHash busca dedup dentro do escopo do paciente ("" = perfil terapeuta).
	FindByHash(ctx context.Context, patientID, sha string) (*domain.GEDFile, error)
	ListByPatient(ctx context.Context, patientID string) ([]*domain.GEDFile, error)
	Delete(ctx context.Context, id string) error
}

// GEDStorage é o armazenamento físico (repository/ged.Store).
type GEDStorage interface {
	Write(scope, fileID string, content io.Reader) (relPath, sha string, size int64, err error)
	Read(relPath, expectedSHA string) ([]byte, error)
	Delete(relPath string) error
}

// GEDService coordena a escrita/leitura de arquivos com integridade e dedup.
type GEDService struct {
	meta    GEDMetaRepository
	storage GEDStorage
	clock   clock.Clock
}

// NewGEDService cria o serviço do GED.
func NewGEDService(meta GEDMetaRepository, storage GEDStorage) *GEDService {
	return &GEDService{meta: meta, storage: storage, clock: clock.System{}}
}

// GEDLink vincula o arquivo a uma entidade (opcional).
type GEDLink struct {
	PatientID string
	SessionID string
	DebtID    string
	PaymentID string
}

// Store grava um arquivo no escopo do paciente (PatientID vazio = perfil do
// terapeuta), com dedup por hash: se já existe um arquivo idêntico no escopo,
// devolve o metadado existente sem regravar.
func (s *GEDService) Store(ctx context.Context, link GEDLink, mime string, content io.Reader) (*domain.GEDFile, error) {
	// Lê tudo em memória para permitir dedup por hash antes de persistir.
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, err
	}

	fileID := ulid.New()
	relPath, sha, size, err := s.storage.Write(link.PatientID, fileID, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// Dedup: se já há arquivo com o mesmo hash no escopo, remove o recém-escrito
	// e devolve o existente.
	if existing, err := s.meta.FindByHash(ctx, link.PatientID, sha); err == nil && existing != nil {
		_ = s.storage.Delete(relPath)
		return existing, nil
	}

	f := &domain.GEDFile{
		ID:        fileID,
		PatientID: link.PatientID,
		SessionID: link.SessionID,
		DebtID:    link.DebtID,
		PaymentID: link.PaymentID,
		RelPath:   relPath,
		MIME:      mime,
		Size:      size,
		SHA256:    sha,
		CreatedAt: s.clock.Now(),
	}
	if err := s.meta.Insert(ctx, f); err != nil {
		_ = s.storage.Delete(relPath)
		return nil, err
	}
	return f, nil
}

// Read devolve o conteúdo de um arquivo, validando integridade.
func (s *GEDService) Read(ctx context.Context, id string) (*domain.GEDFile, []byte, error) {
	f, err := s.meta.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	data, err := s.storage.Read(f.RelPath, f.SHA256)
	if err != nil {
		return nil, nil, err
	}
	return f, data, nil
}

// Get devolve apenas o metadado.
func (s *GEDService) Get(ctx context.Context, id string) (*domain.GEDFile, error) {
	return s.meta.Get(ctx, id)
}

// ListByPatient devolve os arquivos de um paciente.
func (s *GEDService) ListByPatient(ctx context.Context, patientID string) ([]*domain.GEDFile, error) {
	return s.meta.ListByPatient(ctx, patientID)
}
