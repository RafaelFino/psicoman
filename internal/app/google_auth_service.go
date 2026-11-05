package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// GoogleAuthState é o estado de autorização do Google exposto à observabilidade
// (/readyz) e à operação (R2.3). "ok" quando há refresh token utilizável;
// "degraded" quando falta token ou uma renovação falhou (reautorização
// necessária) — NUNCA derruba o processo (psicoman-google-api.md).
type GoogleAuthState string

const (
	// GoogleAuthOK: há refresh token persistido e nenhuma falha de renovação.
	GoogleAuthOK GoogleAuthState = "ok"
	// GoogleAuthDegraded: sem token ou renovação falhou — reautorização necessária.
	GoogleAuthDegraded GoogleAuthState = "degraded"
)

// GoogleAuthService orquestra a autorização OAuth 3-legged do Google (R2): salva
// o refresh token (cifrado pelo adapter na borda), carrega-o no boot e mantém o
// estado de "reautorização necessária" em memória. A troca code→token (que toca
// o SDK oauth2) fica fora deste serviço (no handler admin, em internal/api), por
// disciplina de dependência: o app não importa integration/google.
type GoogleAuthService struct {
	repo  GoogleTokenRepository
	clock clock.Clock

	mu            sync.RWMutex
	reauthReason  string // motivo da última falha de renovação ("" = sem falha pendente)
	reauthFlagged bool
}

// NewGoogleAuthService cria o serviço de autorização do Google.
func NewGoogleAuthService(repo GoogleTokenRepository) *GoogleAuthService {
	return &GoogleAuthService{repo: repo, clock: clock.System{}}
}

// SaveTokenInput são os dados do token recebidos no callback OAuth.
type SaveTokenInput struct {
	RefreshToken string
	Scopes       []string
	Expiry       time.Time
}

// SaveToken persiste o refresh token (cifrado na borda pelo adapter) e limpa o
// estado de reautorização. Erro se o refresh token vier vazio (o Google só o
// devolve com access_type=offline + prompt=consent — sem ele não há o que salvar).
func (s *GoogleAuthService) SaveToken(ctx context.Context, in SaveTokenInput) error {
	if in.RefreshToken == "" {
		return NewValidation("O Google não retornou um refresh token. Refaça a autorização concedendo o consentimento.")
	}
	now := s.clock.Now()
	t := &domain.GoogleToken{
		RefreshToken: domain.NewSealedText(in.RefreshToken),
		Scopes:       in.Scopes,
		Expiry:       in.Expiry,
		UpdatedAt:    now,
	}
	// Reusa o id existente (singleton) ou gera um novo.
	if existing, err := s.repo.Load(ctx); err == nil {
		t.ID = existing.ID
		t.CreatedAt = existing.CreatedAt
	} else if errors.Is(err, ErrNotFound) {
		t.ID = ulid.New()
		t.CreatedAt = now
	} else {
		return err
	}
	if err := s.repo.Save(ctx, t); err != nil {
		return err
	}
	s.clearReauth()
	return nil
}

// RefreshToken carrega o refresh token persistido (claro em memória). Devolve
// ("", false) quando ainda não há autorização (ErrNotFound). Usado pelo wiring
// do Calendar/Drive no boot para preferir o repo ao env (precedência do repo).
func (s *GoogleAuthService) RefreshToken(ctx context.Context) (string, bool, error) {
	t, err := s.repo.Load(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if !t.HasRefreshToken() {
		return "", false, nil
	}
	return t.RefreshToken.Reveal(), true, nil
}

// FlagReauthorization marca que uma renovação falhou (reautorização necessária).
// É best-effort e idempotente — não derruba nada; só altera o estado observável.
func (s *GoogleAuthService) FlagReauthorization(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reauthFlagged = true
	s.reauthReason = reason
}

func (s *GoogleAuthService) clearReauth() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reauthFlagged = false
	s.reauthReason = ""
}

// Status devolve o estado de autorização do Google para a observabilidade
// (/readyz) e um motivo legível quando degradado. degraded quando não há token
// persistido OU uma renovação falhou (flag em memória). Nunca retorna erro: a
// ausência de token é um estado, não uma falha.
func (s *GoogleAuthService) Status(ctx context.Context) (GoogleAuthState, string) {
	s.mu.RLock()
	flagged, reason := s.reauthFlagged, s.reauthReason
	s.mu.RUnlock()
	if flagged {
		return GoogleAuthDegraded, reason
	}
	t, err := s.repo.Load(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return GoogleAuthDegraded, "Instância ainda não autorizada no Google."
		}
		return GoogleAuthDegraded, "Falha ao ler o token do Google."
	}
	if !t.HasRefreshToken() {
		return GoogleAuthDegraded, "Sem refresh token do Google (reautorização necessária)."
	}
	return GoogleAuthOK, ""
}
