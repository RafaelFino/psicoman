package service

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// PatientRepository persiste pacientes.
type PatientRepository interface {
	Create(ctx context.Context, p *domain.Patient) error
	Update(ctx context.Context, p *domain.Patient) error
	Get(ctx context.Context, id string) (*domain.Patient, error)
	GetByEmail(ctx context.Context, email string) (*domain.Patient, error)
	List(ctx context.Context) ([]*domain.Patient, error)
	ListByApproval(ctx context.Context, status string) ([]*domain.Patient, error)
	SoftDelete(ctx context.Context, id string) error
	// EmailExists / CPFExists checam unicidade ignorando um id (para update).
	EmailExists(ctx context.Context, email, exceptID string) (bool, error)
	CPFExists(ctx context.Context, cpf, exceptID string) (bool, error)
}

// PatientService orquestra o cadastro de pacientes com as regras de negócio.
type PatientService struct {
	repo  PatientRepository
	clock clock.Clock
}

// NewPatientService cria o serviço.
func NewPatientService(repo PatientRepository) *PatientService {
	return &PatientService{repo: repo, clock: clock.System{}}
}

// CreateInput são os dados de criação de um paciente.
type CreateInput struct {
	Name     string
	Phone    string
	Email    string
	CPF      string
	OriginID string
}

// Create valida e cria um paciente, garantindo unicidade de email e CPF.
func (s *PatientService) Create(ctx context.Context, in CreateInput) (*domain.Patient, error) {
	now := s.clock.Now()
	p := &domain.Patient{
		ID:             ulid.New(),
		Name:           in.Name,
		Phone:          in.Phone,
		Email:          in.Email,
		CPF:            domain.NormalizeCPF(in.CPF),
		OriginID:       in.OriginID,
		ApprovalStatus: domain.PatientApproved, // cadastro pelo admin: já aprovado (R1.1)
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := p.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.checkUnique(ctx, p, ""); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, s.mapErr(err)
	}
	return p, nil
}

// UpdateInput são os dados de atualização (id + campos).
type UpdateInput struct {
	ID       string
	Name     string
	Phone    string
	Email    string
	CPF      string
	OriginID string
}

// Update valida e atualiza um paciente existente.
func (s *PatientService) Update(ctx context.Context, in UpdateInput) (*domain.Patient, error) {
	existing, err := s.repo.Get(ctx, in.ID)
	if err != nil {
		return nil, s.mapErr(err)
	}
	existing.Name = in.Name
	existing.Phone = in.Phone
	existing.Email = in.Email
	existing.CPF = domain.NormalizeCPF(in.CPF)
	existing.OriginID = in.OriginID
	existing.UpdatedAt = s.clock.Now()

	if err := existing.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.checkUnique(ctx, existing, existing.ID); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, s.mapErr(err)
	}
	return existing, nil
}

// PortalRegisterInput são os dados de cadastro básico feito pelo próprio
// paciente no portal.
type PortalRegisterInput struct {
	Name  string
	Phone string
	Email string
	CPF   string
}

// RegisterFromPortal cria ou vincula um paciente pelo email verificado do login
// social. Se já existe um paciente com esse email (cadastrado pelo terapeuta ou
// pelo próprio paciente), ATUALIZA/VINCULA ao registro existente — nunca
// duplica (requirements §3.1; upsert por email).
func (s *PatientService) RegisterFromPortal(ctx context.Context, in PortalRegisterInput) (*domain.Patient, error) {
	existing, err := s.repo.GetByEmail(ctx, in.Email)
	if err == nil && existing != nil {
		// Vincula: completa dados básicos sem sobrescrever o que o terapeuta já tem.
		if in.Name != "" {
			existing.Name = in.Name
		}
		if in.Phone != "" {
			existing.Phone = in.Phone
		}
		if cpf := domain.NormalizeCPF(in.CPF); cpf != "" {
			existing.CPF = cpf
		}
		// Vínculo por email não rebaixa o estado: se já era aprovado (cadastrado
		// pelo terapeuta), mantém aprovado (R1.1). Registros legados sem estado
		// definido caem em pendente por segurança.
		if !domain.ValidApprovalStatus(existing.ApprovalStatus) {
			existing.ApprovalStatus = domain.PatientPending
		}
		existing.UpdatedAt = s.clock.Now()
		if err := existing.Validate(); err != nil {
			return nil, NewValidation(err.Error())
		}
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}
	// Novo auto-cadastro pelo portal: nasce pendente de aprovação (R1.1).
	now := s.clock.Now()
	p := &domain.Patient{
		ID:             ulid.New(),
		Name:           in.Name,
		Phone:          in.Phone,
		Email:          in.Email,
		CPF:            domain.NormalizeCPF(in.CPF),
		ApprovalStatus: domain.PatientPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := p.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.checkUnique(ctx, p, ""); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, s.mapErr(err)
	}
	return p, nil
}

// Get devolve um paciente por id.
func (s *PatientService) Get(ctx context.Context, id string) (*domain.Patient, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, s.mapErr(err)
	}
	return p, nil
}

// GetByEmail devolve um paciente pelo email (usado pelo portal, isolamento).
func (s *PatientService) GetByEmail(ctx context.Context, email string) (*domain.Patient, error) {
	return s.repo.GetByEmail(ctx, email)
}

// List devolve todos os pacientes ativos.
func (s *PatientService) List(ctx context.Context) ([]*domain.Patient, error) {
	return s.repo.List(ctx)
}

// Delete faz soft-delete de um paciente.
func (s *PatientService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return s.mapErr(err)
	}
	return s.repo.SoftDelete(ctx, id)
}

// ListPending devolve os pacientes com cadastro pendente de aprovação (fila).
func (s *PatientService) ListPending(ctx context.Context) ([]*domain.Patient, error) {
	return s.repo.ListByApproval(ctx, domain.PatientPending)
}

// Approve libera o acesso do paciente ao portal (transição pendente→aprovado).
func (s *PatientService) Approve(ctx context.Context, id string) (*domain.Patient, error) {
	return s.transitionApproval(ctx, id, domain.PatientApproved)
}

// Reject nega o acesso do paciente ao portal (transição pendente→rejeitado).
func (s *PatientService) Reject(ctx context.Context, id string) (*domain.Patient, error) {
	return s.transitionApproval(ctx, id, domain.PatientRejected)
}

// transitionApproval aplica uma transição de estado de aprovação válida.
func (s *PatientService) transitionApproval(ctx context.Context, id, next string) (*domain.Patient, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, s.mapErr(err)
	}
	if p.ApprovalStatus == next {
		return p, nil // idempotente: já está no estado desejado
	}
	if !p.CanTransitionApproval(next) {
		return nil, NewValidation("Este cadastro não pode mais ter o estado de aprovação alterado.")
	}
	p.ApprovalStatus = next
	p.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, s.mapErr(err)
	}
	return p, nil
}

// checkUnique valida unicidade de email e CPF (exceto o próprio id no update).
func (s *PatientService) checkUnique(ctx context.Context, p *domain.Patient, exceptID string) error {
	emailDup, err := s.repo.EmailExists(ctx, p.Email, exceptID)
	if err != nil {
		return s.mapErr(err)
	}
	if emailDup {
		return NewConflict("Já existe um paciente com este email.")
	}
	if p.CPF != "" {
		cpfDup, err := s.repo.CPFExists(ctx, p.CPF, exceptID)
		if err != nil {
			return s.mapErr(err)
		}
		if cpfDup {
			return NewConflict("Já existe um paciente com este CPF.")
		}
	}
	return nil
}

// mapErr repassa erros do repositório; erros sentinela (ErrNotFound/ErrConflict)
// são preservados para a API mapear ao HTTP correto.
func (s *PatientService) mapErr(err error) error { return err }
