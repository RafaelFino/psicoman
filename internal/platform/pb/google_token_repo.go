package pb

import (
	"context"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain"
)

// googleTokenRepo implementa app.GoogleTokenRepository sobre a coleção
// google_token (migração 0030). É um dos pontos de cifra por campo na borda
// (D6.3, R2.1): o campo token guarda o ENVELOPE cifrado do refresh token
// (domain.SealFor ao gravar, domain.OpenInto ao ler). scopes (JSON) e expiry
// (ISO) ficam em claro. Singleton lógico: Load pega o único registro.
type googleTokenRepo struct {
	app    core.App
	cipher domain.Cipher
}

var _ app.GoogleTokenRepository = (*googleTokenRepo)(nil)

// NewGoogleTokenRepository cria o repositório do token OAuth com cifra na borda.
func NewGoogleTokenRepository(a core.App, cipher domain.Cipher) app.GoogleTokenRepository {
	return &googleTokenRepo{app: a, cipher: cipher}
}

// Load devolve o token persistido, decifrando o refresh token na borda.
// ErrNotFound quando a instância ainda não foi autorizada (sem registro).
func (r *googleTokenRepo) Load(ctx context.Context) (*domain.GoogleToken, error) {
	recs, err := r.app.FindRecordsByFilter("google_token", "", "created_at", 1, 0)
	if err != nil {
		return nil, mapError(err)
	}
	if len(recs) == 0 {
		return nil, app.ErrNotFound
	}
	rec := recs[0]
	refresh, err := domain.OpenInto(ctx, r.cipher, rec.GetString("token"))
	if err != nil {
		return nil, err
	}
	var scopes []string
	_ = rec.UnmarshalJSONField("scopes", &scopes)
	return &domain.GoogleToken{
		ID:           rec.Id,
		RefreshToken: refresh,
		Scopes:       scopes,
		Expiry:       parseTime(rec.GetString("expiry")),
		CreatedAt:    parseTime(rec.GetString("created_at")),
		UpdatedAt:    parseTime(rec.GetString("updated_at")),
	}, nil
}

// Save grava o token (cria-ou-atualiza), cifrando o refresh token na borda. O
// banco guarda SOMENTE o envelope — o claro nunca é escrito.
func (r *googleTokenRepo) Save(ctx context.Context, t *domain.GoogleToken) error {
	// Singleton: reusa o registro existente se houver.
	recs, err := r.app.FindRecordsByFilter("google_token", "", "created_at", 1, 0)
	if err != nil {
		return mapError(err)
	}
	var rec *core.Record
	if len(recs) > 0 {
		rec = recs[0]
	} else {
		col, cerr := r.app.FindCollectionByNameOrId("google_token")
		if cerr != nil {
			return mapError(cerr)
		}
		rec = core.NewRecord(col)
		rec.Set("id", t.ID)
		rec.Set("created_at", formatTime(t.CreatedAt))
	}
	envelope, err := domain.SealFor(ctx, r.cipher, t.RefreshToken)
	if err != nil {
		return err
	}
	rec.Set("token", envelope)
	rec.Set("scopes", t.Scopes)
	rec.Set("expiry", formatTime(t.Expiry))
	rec.Set("updated_at", formatTime(t.UpdatedAt))
	return mapError(r.app.SaveWithContext(ctx, rec))
}
