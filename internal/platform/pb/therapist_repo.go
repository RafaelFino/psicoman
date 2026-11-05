package pb

import (
	"context"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain/therapist"
)

// therapistRepo implementa app.TherapistRepository sobre as coleções
// therapist_profile e therapist_platform_link (migração 0030). O perfil é um
// singleton lógico 1:1 por instância (R3.1): GetProfile devolve o único registro
// e SaveProfile cria-ou-atualiza pelo id do agregado. Nenhum campo é cifrado —
// o perfil do terapeuta é dado operacional, não clínico de paciente.
type therapistRepo struct{ app core.App }

var _ app.TherapistRepository = (*therapistRepo)(nil)

// NewTherapistRepository cria o repositório de perfil do terapeuta.
func NewTherapistRepository(a core.App) app.TherapistRepository { return &therapistRepo{app: a} }

func (r *therapistRepo) toProfileRecord(rec *core.Record, p *therapist.TherapistProfile) {
	rec.Set("id", p.ID)
	rec.Set("name", p.Name)
	rec.Set("crp", p.CRP)
	rec.Set("email", p.Email)
	rec.Set("bio", p.Bio)
	rec.Set("contacts", p.Contacts)
	rec.Set("photo_file_id", p.PhotoFileID)
	rec.Set("location_ids", p.LocationIDs)
	rec.Set("created_at", formatTime(p.CreatedAt))
	rec.Set("updated_at", formatTime(p.UpdatedAt))
}

func profileFromRecord(rec *core.Record) *therapist.TherapistProfile {
	var contacts map[string]string
	_ = rec.UnmarshalJSONField("contacts", &contacts)
	var locationIDs []string
	_ = rec.UnmarshalJSONField("location_ids", &locationIDs)
	return &therapist.TherapistProfile{
		ID:          rec.Id,
		Name:        rec.GetString("name"),
		CRP:         rec.GetString("crp"),
		Email:       rec.GetString("email"),
		Bio:         rec.GetString("bio"),
		Contacts:    contacts,
		PhotoFileID: rec.GetString("photo_file_id"),
		LocationIDs: locationIDs,
		CreatedAt:   parseTime(rec.GetString("created_at")),
		UpdatedAt:   parseTime(rec.GetString("updated_at")),
	}
}

// GetProfile devolve o único perfil da instância, ordenando por created_at para
// pegar o primeiro de forma determinística caso (indevidamente) exista mais de
// um. ErrNotFound quando ainda não há perfil configurado.
func (r *therapistRepo) GetProfile(ctx context.Context) (*therapist.TherapistProfile, error) {
	recs, err := r.app.FindRecordsByFilter("therapist_profile", "", "created_at", 1, 0)
	if err != nil {
		return nil, mapError(err)
	}
	if len(recs) == 0 {
		return nil, app.ErrNotFound
	}
	return profileFromRecord(recs[0]), nil
}

// SaveProfile cria o perfil se o id ainda não existir, senão atualiza. O id do
// agregado é a fonte de verdade (o serviço garante o singleton reusando o id
// existente num upsert).
func (r *therapistRepo) SaveProfile(ctx context.Context, p *therapist.TherapistProfile) error {
	rec, err := r.app.FindRecordById("therapist_profile", p.ID)
	if err != nil {
		if !isNoRows(err) {
			return mapError(err)
		}
		col, cerr := r.app.FindCollectionByNameOrId("therapist_profile")
		if cerr != nil {
			return mapError(cerr)
		}
		rec = core.NewRecord(col)
	}
	r.toProfileRecord(rec, p)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *therapistRepo) toLinkRecord(rec *core.Record, l *therapist.TherapistPlatformLink) {
	rec.Set("id", l.ID)
	rec.Set("profile_id", l.ProfileID)
	rec.Set("label", l.Label)
	rec.Set("url", l.URL)
	rec.Set("origin_id", l.OriginID)
	rec.Set("created_at", formatTime(l.CreatedAt))
	rec.Set("updated_at", formatTime(l.UpdatedAt))
}

func linkFromRecord(rec *core.Record) *therapist.TherapistPlatformLink {
	return &therapist.TherapistPlatformLink{
		ID:        rec.Id,
		ProfileID: rec.GetString("profile_id"),
		Label:     rec.GetString("label"),
		URL:       rec.GetString("url"),
		OriginID:  rec.GetString("origin_id"),
		CreatedAt: parseTime(rec.GetString("created_at")),
		UpdatedAt: parseTime(rec.GetString("updated_at")),
	}
}

func (r *therapistRepo) ListLinks(ctx context.Context, profileID string) ([]*therapist.TherapistPlatformLink, error) {
	recs, err := r.app.FindRecordsByFilter("therapist_platform_link",
		"profile_id = {:p}", "created_at", 0, 0, dbxParams("p", profileID))
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*therapist.TherapistPlatformLink, 0, len(recs))
	for _, rec := range recs {
		out = append(out, linkFromRecord(rec))
	}
	return out, nil
}

func (r *therapistRepo) CreateLink(ctx context.Context, l *therapist.TherapistPlatformLink) error {
	col, err := r.app.FindCollectionByNameOrId("therapist_platform_link")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toLinkRecord(rec, l)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *therapistRepo) DeleteLink(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "therapist_platform_link", id)
	if err != nil {
		return err
	}
	return mapError(r.app.Delete(rec))
}
