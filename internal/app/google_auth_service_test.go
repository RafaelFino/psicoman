package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// fakeGoogleTokenRepo é um repositório em memória do token OAuth (singleton).
type fakeGoogleTokenRepo struct {
	token   *domain.GoogleToken
	saves   int
	loadErr error // força um erro de I/O no Load (não-ErrNotFound)
}

func (f *fakeGoogleTokenRepo) Load(context.Context) (*domain.GoogleToken, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	if f.token == nil {
		return nil, ErrNotFound
	}
	cp := *f.token
	return &cp, nil
}

func (f *fakeGoogleTokenRepo) Save(_ context.Context, t *domain.GoogleToken) error {
	f.saves++
	cp := *t
	f.token = &cp
	return nil
}

// TestGoogleAuthSaveTokenEStatus: salvar um refresh token válido limpa o estado
// e deixa Status=ok; antes de salvar, Status=degraded (sem token).
func TestGoogleAuthSaveTokenEStatus(t *testing.T) {
	ctx := context.Background()
	repo := &fakeGoogleTokenRepo{}
	svc := NewGoogleAuthService(repo)

	// Sem token: degraded.
	if state, _ := svc.Status(ctx); state != GoogleAuthDegraded {
		t.Errorf("sem token: quero degraded, veio %s", state)
	}

	err := svc.SaveToken(ctx, SaveTokenInput{
		RefreshToken: "1//refresh-abc",
		Scopes:       []string{"https://www.googleapis.com/auth/calendar"},
		Expiry:       time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	if repo.saves != 1 {
		t.Errorf("SaveToken deveria gravar 1 vez, gravou %d", repo.saves)
	}
	if state, reason := svc.Status(ctx); state != GoogleAuthOK || reason != "" {
		t.Errorf("com token: quero ok sem motivo, veio %s (%q)", state, reason)
	}

	// RefreshToken devolve o claro.
	tok, ok, err := svc.RefreshToken(ctx)
	if err != nil || !ok || tok != "1//refresh-abc" {
		t.Errorf("RefreshToken: %q ok=%v err=%v", tok, ok, err)
	}
}

// TestGoogleAuthSaveTokenVazio rejeita refresh token vazio (o Google só devolve
// com access_type=offline + prompt=consent).
func TestGoogleAuthSaveTokenVazio(t *testing.T) {
	svc := NewGoogleAuthService(&fakeGoogleTokenRepo{})
	err := svc.SaveToken(context.Background(), SaveTokenInput{RefreshToken: ""})
	if err == nil || !errors.Is(err, ErrValidation) {
		t.Errorf("SaveToken vazio deveria ser ErrValidation, veio: %v", err)
	}
}

// TestGoogleAuthSaveTokenReusaID: um 2º SaveToken reusa o id/created_at do
// token existente (singleton), não cria outro.
func TestGoogleAuthSaveTokenReusaID(t *testing.T) {
	ctx := context.Background()
	repo := &fakeGoogleTokenRepo{}
	svc := NewGoogleAuthService(repo)

	if err := svc.SaveToken(ctx, SaveTokenInput{RefreshToken: "t1"}); err != nil {
		t.Fatal(err)
	}
	id1, created1 := repo.token.ID, repo.token.CreatedAt
	if err := svc.SaveToken(ctx, SaveTokenInput{RefreshToken: "t2"}); err != nil {
		t.Fatal(err)
	}
	if repo.token.ID != id1 {
		t.Errorf("2º SaveToken trocou o id: %s != %s", repo.token.ID, id1)
	}
	if !repo.token.CreatedAt.Equal(created1) {
		t.Errorf("2º SaveToken trocou created_at")
	}
	if repo.token.RefreshToken.Reveal() != "t2" {
		t.Errorf("2º SaveToken não atualizou o token: %q", repo.token.RefreshToken.Reveal())
	}
}

// TestGoogleAuthFlagReauthorization: a flag de reautorização força degraded
// mesmo com token presente, até um novo SaveToken limpar.
func TestGoogleAuthFlagReauthorization(t *testing.T) {
	ctx := context.Background()
	repo := &fakeGoogleTokenRepo{}
	svc := NewGoogleAuthService(repo)
	_ = svc.SaveToken(ctx, SaveTokenInput{RefreshToken: "t1"})

	svc.FlagReauthorization("renovação falhou")
	if state, reason := svc.Status(ctx); state != GoogleAuthDegraded || reason != "renovação falhou" {
		t.Errorf("após flag: quero degraded com motivo, veio %s (%q)", state, reason)
	}

	// Novo SaveToken limpa a flag.
	_ = svc.SaveToken(ctx, SaveTokenInput{RefreshToken: "t2"})
	if state, _ := svc.Status(ctx); state != GoogleAuthOK {
		t.Errorf("após re-salvar: quero ok, veio %s", state)
	}
}

// TestGoogleAuthRefreshTokenAusente: sem autorização, RefreshToken devolve
// ("", false, nil) — não é erro, é estado.
func TestGoogleAuthRefreshTokenAusente(t *testing.T) {
	svc := NewGoogleAuthService(&fakeGoogleTokenRepo{})
	tok, ok, err := svc.RefreshToken(context.Background())
	if tok != "" || ok || err != nil {
		t.Errorf("sem token: quero (\"\", false, nil), veio (%q, %v, %v)", tok, ok, err)
	}
}
