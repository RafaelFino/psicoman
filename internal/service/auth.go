package service

import (
	"errors"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type PatientClaims struct {
	PatientID string `json:"patient_id"`
	Email     string `json:"email"`
	jwt.RegisteredClaims
}

type AuthService struct {
	JWTSecret          string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

func (a *AuthService) PatientOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.GoogleClientID,
		ClientSecret: a.GoogleClientSecret,
		RedirectURL:  a.GoogleRedirectURL,
		Scopes:       []string{"email", "profile", "openid"},
		Endpoint:     google.Endpoint,
	}
}

func (a *AuthService) PatientAuthURL(state string) string {
	return a.PatientOAuthConfig().AuthCodeURL(state)
}

func (a *AuthService) IssuePatientToken(patientID, email string) (string, error) {
	claims := PatientClaims{
		PatientID: patientID,
		Email:     email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.JWTSecret))
}

func (a *AuthService) ParsePatientToken(tokenStr string) (*PatientClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &PatientClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(a.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*PatientClaims)
	if !ok || !token.Valid {
		return nil, errors.New("token inválido")
	}
	return claims, nil
}

func (a *AuthService) EnsureStaff(db *storage.DB, email, rolesHeader string) (*domain.StaffUser, error) {
	role := domain.RolePsychologist
	if rolesHeader == "admin" {
		role = domain.RoleAdmin
	}
	return db.UpsertStaffUser(email, role)
}
