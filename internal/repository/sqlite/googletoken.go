package sqlite

import (
	"context"
	"database/sql"
	"strings"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// Cipher abstrai a cifragem do refresh token (platform/crypto.Cipher).
type Cipher interface {
	Encrypt(plaintext []byte) (string, error)
	Decrypt(encoded string) ([]byte, error)
}

// GoogleTokenRepo persiste o refresh token do Google CIFRADO (um por instância).
type GoogleTokenRepo struct {
	db     *sql.DB
	cipher Cipher
}

// NewGoogleTokenRepo cria o repositório do token Google.
func NewGoogleTokenRepo(db *DB, cipher Cipher) *GoogleTokenRepo {
	return &GoogleTokenRepo{db: db.DB, cipher: cipher}
}

// SaveRefreshToken cifra e persiste o refresh token e os escopos.
func (r *GoogleTokenRepo) SaveRefreshToken(ctx context.Context, refreshToken string, scopes []string) error {
	enc, err := r.cipher.Encrypt([]byte(refreshToken))
	if err != nil {
		return err
	}
	now := clock.Format(clock.Now())
	// Um registro por instância: usa id fixo via upsert.
	id := r.singletonID(ctx)
	if id == "" {
		id = ulid.New()
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO google_token (id, refresh_token_enc, scopes, reauth_required, created_at, updated_at)
		 VALUES (?, ?, ?, 0, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   refresh_token_enc=excluded.refresh_token_enc, scopes=excluded.scopes,
		   reauth_required=0, updated_at=excluded.updated_at`,
		id, enc, strings.Join(scopes, " "), now, now)
	return mapError(err)
}

// LoadRefreshToken decifra e devolve o refresh token ("" se não autorizado).
func (r *GoogleTokenRepo) LoadRefreshToken(ctx context.Context) (string, error) {
	var enc string
	err := r.db.QueryRowContext(ctx,
		`SELECT refresh_token_enc FROM google_token LIMIT 1`).Scan(&enc)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", mapError(err)
	}
	plain, err := r.cipher.Decrypt(enc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// SetReauthRequired sinaliza que a reautorização é necessária.
func (r *GoogleTokenRepo) SetReauthRequired(ctx context.Context, required bool) error {
	v := 0
	if required {
		v = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE google_token SET reauth_required=?, updated_at=?`, v, clock.Format(clock.Now()))
	return mapError(err)
}

// ReauthRequired devolve true somente quando já houve autorização e o refresh
// falhou (flag reauth_required=1). Uma instância nunca autorizada NÃO é
// considerada degradada — apenas ainda não conectada ao Google.
func (r *GoogleTokenRepo) ReauthRequired(ctx context.Context) (bool, error) {
	var v int
	err := r.db.QueryRowContext(ctx, `SELECT reauth_required FROM google_token LIMIT 1`).Scan(&v)
	if err == sql.ErrNoRows {
		return false, nil // sem token = ainda não conectado, não degradado
	}
	if err != nil {
		return false, mapError(err)
	}
	return v == 1, nil
}

// Authorized indica se há um refresh token persistido.
func (r *GoogleTokenRepo) Authorized(ctx context.Context) (bool, error) {
	var enc string
	err := r.db.QueryRowContext(ctx, `SELECT refresh_token_enc FROM google_token LIMIT 1`).Scan(&enc)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, mapError(err)
	}
	return enc != "", nil
}

func (r *GoogleTokenRepo) singletonID(ctx context.Context) string {
	var id string
	_ = r.db.QueryRowContext(ctx, `SELECT id FROM google_token LIMIT 1`).Scan(&id)
	return id
}
