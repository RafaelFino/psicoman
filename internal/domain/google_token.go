package domain

import "time"

// GoogleToken guarda o refresh token OAuth 3-legged do terapeuta (R2). Há no
// máximo um por instância (single-tenant). O refresh token é dado sensível:
// usa SealedText (claro só em memória; cifrado na borda de persistência, como
// o prontuário — psicoman-google-api.md "refresh token persistido cifrado").
// Os escopos e a expiry do access token ficam em CLARO (não são segredo e são
// úteis para diagnóstico/observabilidade).
//
// Transversal aos bounded contexts: vive no pacote raiz domain.
type GoogleToken struct {
	ID           string
	RefreshToken SealedText // cifrado em repouso; NUNCA logado nem serializado
	Scopes       []string
	Expiry       time.Time // expiry do access token (informativo; o refresh renova)
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// HasRefreshToken indica se há um refresh token utilizável (não vazio).
func (t *GoogleToken) HasRefreshToken() bool {
	return t != nil && !t.RefreshToken.Empty()
}
