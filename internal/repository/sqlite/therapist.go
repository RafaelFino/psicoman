package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// TherapistRepo implementa service.TherapistRepository sobre SQLite.
type TherapistRepo struct{ db *sql.DB }

// NewTherapistRepo cria o repositório do perfil do terapeuta.
func NewTherapistRepo(db *DB) *TherapistRepo { return &TherapistRepo{db: db.DB} }

var _ service.TherapistRepository = (*TherapistRepo)(nil)

// GetProfile devolve o perfil único (o primeiro registro).
func (r *TherapistRepo) GetProfile(ctx context.Context) (*domain.TherapistProfile, error) {
	var (
		p                    domain.TherapistProfile
		crp, email, contacts sql.NullString
		bio, photo           sql.NullString
		createdAt, updated   string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, crp, email, contacts, bio, photo_ged_file_id, created_at, updated_at
		 FROM therapist_profile LIMIT 1`).
		Scan(&p.ID, &p.Name, &crp, &email, &contacts, &bio, &photo, &createdAt, &updated)
	if err != nil {
		return nil, mapError(err)
	}
	p.CRP = crp.String
	p.Email = email.String
	p.Bio = bio.String
	p.PhotoFileID = photo.String
	if contacts.Valid && contacts.String != "" {
		_ = json.Unmarshal([]byte(contacts.String), &p.Contacts)
	}
	if t, err := clock.Parse(createdAt); err == nil {
		p.CreatedAt = t
	}
	if t, err := clock.Parse(updated); err == nil {
		p.UpdatedAt = t
	}
	p.LocationIDs, _ = r.locationIDs(ctx, p.ID)
	return &p, nil
}

func (r *TherapistRepo) locationIDs(ctx context.Context, profileID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT location_id FROM therapist_location WHERE profile_id=?`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpsertProfile cria ou atualiza o perfil.
func (r *TherapistRepo) UpsertProfile(ctx context.Context, p *domain.TherapistProfile) error {
	var contacts any
	if len(p.Contacts) > 0 {
		b, _ := json.Marshal(p.Contacts)
		contacts = string(b)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO therapist_profile (id, name, crp, email, contacts, bio, photo_ged_file_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name=excluded.name, crp=excluded.crp, email=excluded.email,
		   contacts=excluded.contacts, bio=excluded.bio,
		   photo_ged_file_id=excluded.photo_ged_file_id, updated_at=excluded.updated_at`,
		p.ID, p.Name, nullStr(p.CRP), nullStr(p.Email), contacts, nullStr(p.Bio),
		nullStr(p.PhotoFileID), clock.Format(p.CreatedAt), clock.Format(p.UpdatedAt))
	return mapError(err)
}

// SetLocations substitui as associações de locais do perfil.
func (r *TherapistRepo) SetLocations(ctx context.Context, profileID string, locationIDs []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM therapist_location WHERE profile_id=?`, profileID); err != nil {
		return mapError(err)
	}
	for _, lid := range locationIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO therapist_location (profile_id, location_id) VALUES (?, ?)`,
			profileID, lid); err != nil {
			return mapError(err)
		}
	}
	return tx.Commit()
}

// AddLink insere um link de plataforma.
func (r *TherapistRepo) AddLink(ctx context.Context, l *domain.TherapistPlatformLink) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO therapist_platform_link (id, profile_id, label, url, origin_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.ProfileID, l.Label, l.URL, nullStr(l.OriginID),
		clock.Format(l.CreatedAt), clock.Format(l.UpdatedAt))
	return mapError(err)
}

// ListLinks devolve os links do perfil.
func (r *TherapistRepo) ListLinks(ctx context.Context, profileID string) ([]*domain.TherapistPlatformLink, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, profile_id, label, url, origin_id, created_at, updated_at
		 FROM therapist_platform_link WHERE profile_id=? ORDER BY label`, profileID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.TherapistPlatformLink
	for rows.Next() {
		var (
			l                  domain.TherapistPlatformLink
			originID           sql.NullString
			createdAt, updated string
		)
		if err := rows.Scan(&l.ID, &l.ProfileID, &l.Label, &l.URL, &originID, &createdAt, &updated); err != nil {
			return nil, err
		}
		l.OriginID = originID.String
		if t, err := clock.Parse(createdAt); err == nil {
			l.CreatedAt = t
		}
		if t, err := clock.Parse(updated); err == nil {
			l.UpdatedAt = t
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

// DeleteLink remove um link de plataforma.
func (r *TherapistRepo) DeleteLink(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM therapist_platform_link WHERE id=?`, id)
	return mapError(err)
}
