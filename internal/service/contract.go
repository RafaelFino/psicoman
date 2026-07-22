package service

import (
	"errors"
	"strings"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
)

type ContractService struct{}

type CreateContractTemplateInput struct {
	Name        string `json:"name"`
	ContentHTML string `json:"content_html"`
}

type CreateContractInput struct {
	PatientID  string `json:"patient_id"`
	TemplateID string `json:"template_id"`
}

func (s *ContractService) ListTemplates(db *storage.DB) ([]domain.ContractTemplate, error) {
	return db.ListContractTemplates()
}

func (s *ContractService) CreateTemplate(db *storage.DB, in CreateContractTemplateInput) (*domain.ContractTemplate, error) {
	if in.Name == "" {
		return nil, errors.New("nome é obrigatório")
	}
	if in.ContentHTML == "" {
		return nil, errors.New("conteúdo HTML é obrigatório")
	}
	t := domain.ContractTemplate{
		Name:        in.Name,
		ContentHTML: in.ContentHTML,
		IsActive:    true,
	}
	return db.CreateContractTemplate(t)
}

func (s *ContractService) UpdateTemplate(db *storage.DB, id string, in CreateContractTemplateInput) error {
	existing, err := db.GetContractTemplate(id)
	if err != nil {
		return errors.New("template não encontrado")
	}
	existing.Name = in.Name
	existing.ContentHTML = in.ContentHTML
	return db.UpdateContractTemplate(*existing)
}

func (s *ContractService) ListContracts(db *storage.DB, patientID string) ([]domain.Contract, error) {
	return db.ListContracts(patientID)
}

func (s *ContractService) Create(db *storage.DB, in CreateContractInput) (*domain.Contract, error) {
	if in.PatientID == "" || in.TemplateID == "" {
		return nil, errors.New("patient_id e template_id são obrigatórios")
	}

	patient, err := db.GetPatient(in.PatientID)
	if err != nil {
		return nil, errors.New("paciente não encontrado")
	}

	tmpl, err := db.GetContractTemplate(in.TemplateID)
	if err != nil {
		return nil, errors.New("template não encontrado")
	}

	// Replace placeholders in template
	html := tmpl.ContentHTML
	html = strings.ReplaceAll(html, "{{PATIENT_NAME}}", patient.Name)
	html = strings.ReplaceAll(html, "{{PATIENT_EMAIL}}", patient.Email)
	html = strings.ReplaceAll(html, "{{PATIENT_PHONE}}", patient.Phone)
	html = strings.ReplaceAll(html, "{{DATE}}", time.Now().Format("02/01/2006"))

	ct := domain.Contract{
		PatientID:     in.PatientID,
		TemplateID:    in.TemplateID,
		Status:        domain.ContractPending,
		GeneratedHTML: html,
	}
	return db.CreateContract(ct)
}

func (s *ContractService) Sign(db *storage.DB, contractID, patientID, ip, userAgent string) error {
	ct, err := db.GetContractForPatient(contractID, patientID)
	if err != nil {
		return errors.New("contrato não encontrado")
	}
	if ct.Status != domain.ContractPending {
		return errors.New("contrato não está pendente de assinatura")
	}

	now := time.Now().UTC()
	return db.UpdateContractStatus(contractID, domain.ContractSigned, &now, ip, userAgent)
}

func (s *ContractService) Revoke(db *storage.DB, contractID string) error {
	ct, err := db.GetContract(contractID)
	if err != nil {
		return errors.New("contrato não encontrado")
	}
	if ct.Status == domain.ContractRevoked {
		return errors.New("contrato já revogado")
	}
	return db.UpdateContractStatus(contractID, domain.ContractRevoked, nil, "", "")
}
