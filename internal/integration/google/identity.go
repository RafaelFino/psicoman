package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// IdentityVerifier valida um access token do login social Google e devolve o
// email verificado (via endpoint userinfo). Implementa portal.IdentityVerifier.
type IdentityVerifier struct {
	http *http.Client
}

// NewIdentityVerifier cria o verificador de identidade.
func NewIdentityVerifier() *IdentityVerifier {
	return &IdentityVerifier{http: &http.Client{Timeout: 10 * time.Second}}
}

// Verify troca o access token pelo email verificado do usuário Google.
func (v *IdentityVerifier) Verify(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v3/userinfo?"+url.Values{}.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := v.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google: userinfo → %d", resp.StatusCode)
	}
	var info struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	if !info.EmailVerified || info.Email == "" {
		return "", fmt.Errorf("google: email não verificado")
	}
	return info.Email, nil
}

// FakeIdentityVerifier trata a credencial como o próprio email (testes).
type FakeIdentityVerifier struct{}

// Verify devolve a credencial como email (sem rede).
func (FakeIdentityVerifier) Verify(_ context.Context, credential string) (string, error) {
	if credential == "" {
		return "", fmt.Errorf("credencial vazia")
	}
	return credential, nil
}
