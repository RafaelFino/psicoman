// Package portal contém handlers e middleware exclusivos do binário
// psicoman-portal (login social do paciente, rotas self-service). Nunca expõe
// dado clínico (psicoman-seguranca-lgpd.md).
package portal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

const sessionCookie = "psicoman_portal"

// sessionData é o conteúdo assinado do cookie de sessão do paciente.
type sessionData struct {
	Email     string `json:"email"`
	ExpiresAt int64  `json:"exp"`
}

// SessionManager assina e valida cookies de sessão com HMAC-SHA256.
type SessionManager struct {
	secret []byte
	ttl    time.Duration
	secure bool
}

// NewSessionManager cria o gerenciador de sessão do portal.
//
// secure controla o atributo Secure do cookie. Em produção (TLS terminado no
// Pangolin) deve ser true; em desenvolvimento local sobre http://localhost deve
// ser false, senão o navegador não reenvia o cookie e toda rota autenticada
// responde 401.
func NewSessionManager(secret string, ttl time.Duration, secure bool) *SessionManager {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	return &SessionManager{secret: []byte(secret), ttl: ttl, secure: secure}
}

// Issue cria um cookie de sessão assinado para o email verificado.
func (m *SessionManager) Issue(w http.ResponseWriter, email string) {
	data := sessionData{Email: email, ExpiresAt: clock.Now().Add(m.ttl).Unix()}
	payload, _ := json.Marshal(data)
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	sig := m.sign(b64)
	value := b64 + "." + sig

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure, // TLS terminado no Pangolin (true em prod; false em dev local)
		SameSite: http.SameSiteLaxMode,
		Expires:  clock.Now().Add(m.ttl),
	})
}

// Clear remove o cookie de sessão.
func (m *SessionManager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
}

// Verify valida o cookie da requisição e devolve o email autenticado.
func (m *SessionManager) Verify(r *http.Request) (string, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", errors.New("sessão ausente")
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return "", errors.New("sessão malformada")
	}
	if !hmac.Equal([]byte(m.sign(parts[0])), []byte(parts[1])) {
		return "", errors.New("assinatura inválida")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	var data sessionData
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}
	if clock.Now().Unix() > data.ExpiresAt {
		return "", errors.New("sessão expirada")
	}
	return data.Email, nil
}

func (m *SessionManager) sign(msg string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
