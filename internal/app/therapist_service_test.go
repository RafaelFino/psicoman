package app

import (
	"context"
	"errors"
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain/therapist"
)

// fakeTherapistRepo é um repositório em memória do perfil (singleton) + links.
type fakeTherapistRepo struct {
	profile *therapist.TherapistProfile
	links   map[string]*therapist.TherapistPlatformLink
	saves   int
}

func newTherapistRepo() *fakeTherapistRepo {
	return &fakeTherapistRepo{links: map[string]*therapist.TherapistPlatformLink{}}
}

func (f *fakeTherapistRepo) GetProfile(context.Context) (*therapist.TherapistProfile, error) {
	if f.profile == nil {
		return nil, ErrNotFound
	}
	cp := *f.profile
	return &cp, nil
}

func (f *fakeTherapistRepo) SaveProfile(_ context.Context, p *therapist.TherapistProfile) error {
	f.saves++
	cp := *p
	f.profile = &cp
	return nil
}

func (f *fakeTherapistRepo) ListLinks(_ context.Context, profileID string) ([]*therapist.TherapistPlatformLink, error) {
	var out []*therapist.TherapistPlatformLink
	for _, l := range f.links {
		if l.ProfileID == profileID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *fakeTherapistRepo) CreateLink(_ context.Context, l *therapist.TherapistPlatformLink) error {
	cp := *l
	f.links[l.ID] = &cp
	return nil
}

func (f *fakeTherapistRepo) DeleteLink(_ context.Context, id string) error {
	delete(f.links, id)
	return nil
}

// TestTherapistUpsertIdempotente1a1 garante o singleton lógico: gravar duas
// vezes NÃO cria um segundo perfil — o id e o created_at do primeiro são
// preservados, e o repo guarda sempre um único registro (R3.1).
func TestTherapistUpsertIdempotente1a1(t *testing.T) {
	ctx := context.Background()
	repo := newTherapistRepo()
	svc := NewTherapistService(repo)

	p1, err := svc.Upsert(ctx, UpsertProfileInput{Name: "Dra. Ana", CRP: "06/12345"})
	if err != nil {
		t.Fatalf("1º Upsert: %v", err)
	}
	if p1.ID == "" {
		t.Fatal("perfil sem id após o 1º Upsert")
	}
	createdAt := p1.CreatedAt

	// Segundo upsert altera campos: deve reusar o MESMO id e preservar created_at.
	p2, err := svc.Upsert(ctx, UpsertProfileInput{Name: "Dra. Ana Paula", CRP: "06/12345", Bio: "Atende adultos."})
	if err != nil {
		t.Fatalf("2º Upsert: %v", err)
	}
	if p2.ID != p1.ID {
		t.Errorf("Upsert criou um 2º perfil: id1=%s id2=%s (esperado o mesmo)", p1.ID, p2.ID)
	}
	if !p2.CreatedAt.Equal(createdAt) {
		t.Errorf("created_at não preservado no update: %v != %v", p2.CreatedAt, createdAt)
	}
	if p2.Name != "Dra. Ana Paula" || p2.Bio != "Atende adultos." {
		t.Errorf("campos não atualizados: %+v", p2)
	}

	// Releitura confirma 1:1 (um único registro no repo).
	got, _, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != p1.ID || got.Name != "Dra. Ana Paula" {
		t.Errorf("Get devolveu perfil inesperado: %+v", got)
	}
}

// TestTherapistUpsertValida rejeita perfil sem nome (invariante do agregado).
func TestTherapistUpsertValida(t *testing.T) {
	svc := NewTherapistService(newTherapistRepo())
	_, err := svc.Upsert(context.Background(), UpsertProfileInput{Name: "   "})
	if err == nil || !errors.Is(err, ErrValidation) {
		t.Errorf("Upsert sem nome deveria ser ErrValidation, veio: %v", err)
	}
}

// TestTherapistGetSemPerfil devolve ErrNotFound quando ainda não há perfil.
func TestTherapistGetSemPerfil(t *testing.T) {
	svc := NewTherapistService(newTherapistRepo())
	if _, _, err := svc.Get(context.Background()); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get sem perfil deveria ser ErrNotFound, veio: %v", err)
	}
}

// TestTherapistLinks cobre add/list/remove de links de plataforma, incluindo a
// exigência de um perfil-pai e a validação do link.
func TestTherapistLinks(t *testing.T) {
	ctx := context.Background()
	repo := newTherapistRepo()
	svc := NewTherapistService(repo)

	// Sem perfil ainda: AddLink falha com ErrNotFound.
	if _, err := svc.AddLink(ctx, AddLinkInput{Label: "Doctoralia", URL: "https://x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("AddLink sem perfil deveria ser ErrNotFound, veio: %v", err)
	}

	if _, err := svc.Upsert(ctx, UpsertProfileInput{Name: "Dra. Ana"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Link inválido (sem URL) é rejeitado.
	if _, err := svc.AddLink(ctx, AddLinkInput{Label: "X"}); !errors.Is(err, ErrValidation) {
		t.Errorf("AddLink sem URL deveria ser ErrValidation, veio: %v", err)
	}

	l, err := svc.AddLink(ctx, AddLinkInput{Label: "Doctoralia", URL: "https://doctoralia/ana", OriginID: "o1"})
	if err != nil {
		t.Fatalf("AddLink: %v", err)
	}
	_, links, err := svc.Get(ctx)
	if err != nil || len(links) != 1 || links[0].ID != l.ID {
		t.Fatalf("Get links: %d (%v)", len(links), err)
	}

	if err := svc.RemoveLink(ctx, l.ID); err != nil {
		t.Fatalf("RemoveLink: %v", err)
	}
	_, links, _ = svc.Get(ctx)
	if len(links) != 0 {
		t.Errorf("link não removido: %d restantes", len(links))
	}
}
