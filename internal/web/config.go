package web

import (
	"os"
	"strconv"
)

type Config struct {
	Addr              string
	DataDir           string
	JWTSecret         string
	GoogleClientID    string
	GoogleClientSecret string
	GoogleRedirectURL      string
	GooglePsychRedirectURL string
	GoogleCalendarID       string
	PangolinUserHeader  string
	PangolinEmailHeader string
	PangolinRoleHeader  string
	DefaultTenantID   string

	// DevMode enables local development helpers:
	//   - X-Dev-Auth header bypasses Pangolin on /api/psych/* (value must match DevSecret)
	//   - GET /api/dev/patient-token issues a patient JWT without Google OAuth
	// NEVER enable in production (set DEV_MODE=true only locally or in .env.dev).
	DevMode   bool
	DevSecret string
}

func LoadConfig() Config {
	return Config{
		Addr:                env("ADDR", ":8080"),
		DataDir:             env("DATA_DIR", "./data"),
		JWTSecret:           env("JWT_SECRET", "change-me-in-production"),
		GoogleClientID:      env("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  env("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:      env("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/auth/patient/callback"),
		GooglePsychRedirectURL: env("GOOGLE_PSYCH_REDIRECT_URL", "http://localhost:8080/api/psych/google/callback"),
		GoogleCalendarID:       env("GOOGLE_CALENDAR_ID", "primary"),
		PangolinUserHeader:  env("PANGOLIN_USER_HEADER", "X-User-Id"),
		PangolinEmailHeader: env("PANGOLIN_EMAIL_HEADER", "X-User-Email"),
		PangolinRoleHeader:  env("PANGOLIN_ROLE_HEADER", "X-User-Role"),
		DefaultTenantID:     env("DEFAULT_TENANT_ID", "default"),
		DevMode:             env("DEV_MODE", "") == "true",
		DevSecret:           env("DEV_SECRET", "dev-secret-local"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
