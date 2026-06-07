package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
	"github.com/google/uuid"
)

type GEDService struct {
	BaseDir string
}

func (s *GEDService) dir(tenantID, patientID string) string {
	return filepath.Join(s.BaseDir, tenantID, patientID)
}

func (s *GEDService) Save(db *storage.DB, tenantID string, doc domain.Document, reader io.Reader) (*domain.Document, error) {
	dir := s.dir(tenantID, doc.PatientID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	doc.ID = uuid.New().String()
	doc.Path = filepath.Join(dir, doc.ID+"_"+doc.Filename)

	f, err := os.Create(doc.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		os.Remove(doc.Path)
		return nil, err
	}

	return db.CreateDocument(doc)
}

func (s *GEDService) Open(doc domain.Document) (*os.File, error) {
	if _, err := os.Stat(doc.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("arquivo não encontrado")
	}
	return os.Open(doc.Path)
}

func (s *GEDService) List(db *storage.DB, patientID string) ([]domain.Document, error) {
	return db.ListDocuments(patientID)
}
