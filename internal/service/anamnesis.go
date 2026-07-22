package service

import (
	"errors"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
)

type AnamnesisService struct{}

type CreateAnamnesisTemplateInput struct {
	Name           string                 `json:"name"`
	TargetAgeGroup domain.AgeGroup        `json:"target_age_group"`
	Fields         []domain.AnamnesisField `json:"fields"`
}

type UpdateAnamnesisTemplateInput struct {
	Name           string                 `json:"name"`
	TargetAgeGroup domain.AgeGroup        `json:"target_age_group"`
	Fields         []domain.AnamnesisField `json:"fields"`
	IsActive       bool                   `json:"is_active"`
}

type SubmitAnamnesisInput struct {
	TemplateID string            `json:"template_id"`
	Responses  map[string]string `json:"responses"`
}

func (s *AnamnesisService) ListTemplates(db *storage.DB) ([]domain.AnamnesisTemplate, error) {
	return db.ListAnamnesisTemplates()
}

func (s *AnamnesisService) CreateTemplate(db *storage.DB, in CreateAnamnesisTemplateInput) (*domain.AnamnesisTemplate, error) {
	if in.Name == "" {
		return nil, errors.New("nome é obrigatório")
	}
	if len(in.Fields) == 0 {
		return nil, errors.New("pelo menos um campo é obrigatório")
	}
	if in.TargetAgeGroup == "" {
		in.TargetAgeGroup = domain.AgeGroupAdult
	}

	t := domain.AnamnesisTemplate{
		Name:           in.Name,
		TargetAgeGroup: in.TargetAgeGroup,
		Fields:         in.Fields,
		IsActive:       true,
	}
	return db.CreateAnamnesisTemplate(t)
}

func (s *AnamnesisService) UpdateTemplate(db *storage.DB, id string, in UpdateAnamnesisTemplateInput) (*domain.AnamnesisTemplate, error) {
	existing, err := db.GetAnamnesisTemplate(id)
	if err != nil {
		return nil, errors.New("template não encontrado")
	}
	existing.Name = in.Name
	existing.TargetAgeGroup = in.TargetAgeGroup
	existing.Fields = in.Fields
	existing.IsActive = in.IsActive

	if err := db.UpdateAnamnesisTemplate(*existing); err != nil {
		return nil, err
	}
	return db.GetAnamnesisTemplate(id)
}

func (s *AnamnesisService) DeleteTemplate(db *storage.DB, id string) error {
	return db.DeleteAnamnesisTemplate(id)
}

func (s *AnamnesisService) ListResponses(db *storage.DB, patientID string) ([]domain.AnamnesisResponse, error) {
	return db.ListAnamnesisResponses(patientID)
}

func (s *AnamnesisService) GetResponse(db *storage.DB, id string) (*domain.AnamnesisResponse, error) {
	return db.GetAnamnesisResponse(id)
}

func (s *AnamnesisService) Submit(db *storage.DB, patientID string, in SubmitAnamnesisInput) (*domain.AnamnesisResponse, error) {
	if in.TemplateID == "" {
		return nil, errors.New("template_id é obrigatório")
	}

	// Verify template exists
	tmpl, err := db.GetAnamnesisTemplate(in.TemplateID)
	if err != nil {
		return nil, errors.New("template não encontrado")
	}

	// Validate required fields
	for _, field := range tmpl.Fields {
		if field.Required {
			val, ok := in.Responses[field.Key]
			if !ok || val == "" {
				return nil, errors.New("campo obrigatório: " + field.Label)
			}
		}
	}

	now := time.Now().UTC()
	r := domain.AnamnesisResponse{
		PatientID:   patientID,
		TemplateID:  in.TemplateID,
		Responses:   in.Responses,
		CompletedAt: &now,
	}
	return db.CreateAnamnesisResponse(r)
}
