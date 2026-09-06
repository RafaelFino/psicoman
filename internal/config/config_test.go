package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("escrevendo config temp: %v", err)
	}
	return p
}

func TestLoadAppliesDefaults(t *testing.T) {
	p := writeTemp(t, `
paths:
  sqlite: "./data/psicoman.db"
  ged_root: "./data/ged"
admin_auth:
  email: "t@example.com"
  secret: "s"
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Admin.Port != 8080 {
		t.Errorf("admin port default = %d, quer 8080", cfg.Admin.Port)
	}
	if cfg.Portal.Port != 8081 {
		t.Errorf("portal port default = %d, quer 8081", cfg.Portal.Port)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("log level default = %q, quer info", cfg.Log.Level)
	}
	if cfg.Google.CalendarID != "primary" {
		t.Errorf("calendar id default = %q, quer primary", cfg.Google.CalendarID)
	}
	if len(cfg.Google.Scopes) != 3 {
		t.Errorf("scopes default = %d, quer 3", len(cfg.Google.Scopes))
	}
	if got := cfg.Reminders.MinutesBefore; len(got) != 2 || got[0] != 1440 || got[1] != 30 {
		t.Errorf("reminders default = %v, quer [1440 30]", got)
	}
	if cfg.AdminAuth.EmailHeader != "X-Pangolin-Email" {
		t.Errorf("email header default = %q", cfg.AdminAuth.EmailHeader)
	}
}

func TestLoadValidationErrors(t *testing.T) {
	cases := map[string]string{
		"sem sqlite": "admin_auth:\n  email: t@x.com\n  secret: s\npaths:\n  ged_root: ./g\n",
		"sem ged":    "admin_auth:\n  email: t@x.com\n  secret: s\npaths:\n  sqlite: ./x.db\n",
		"sem email":  "admin_auth:\n  secret: s\npaths:\n  sqlite: ./x.db\n  ged_root: ./g\n",
		"sem secret": "admin_auth:\n  email: t@x.com\npaths:\n  sqlite: ./x.db\n  ged_root: ./g\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			p := writeTemp(t, content)
			if _, err := Load(p); err == nil {
				t.Fatalf("esperava erro de validação para %q", name)
			}
		})
	}
}

func TestReminderDurations(t *testing.T) {
	c := &Config{}
	c.Reminders.MinutesBefore = []int{1440, 30}
	ds := c.ReminderDurations()
	if len(ds) != 2 {
		t.Fatalf("len = %d, quer 2", len(ds))
	}
	if ds[0].Hours() != 24 {
		t.Errorf("primeiro reminder = %v, quer 24h", ds[0])
	}
	if ds[1].Minutes() != 30 {
		t.Errorf("segundo reminder = %v, quer 30m", ds[1])
	}
}

func TestAddr(t *testing.T) {
	s := ServerConfig{Host: "0.0.0.0", Port: 9000}
	if s.Addr() != "0.0.0.0:9000" {
		t.Errorf("Addr = %q", s.Addr())
	}
}
