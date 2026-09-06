// Package config carrega e valida a configuração de uma instância do Psicoman.
//
// Cada terapeuta roda uma instância isolada; toda a parametrização (paths,
// credenciais, segredos) vem de um único arquivo YAML, permitindo deploy seguro
// e segregação de ambientes (docs/architecture.md §8).
package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config é a configuração completa de uma instância.
type Config struct {
	// Server agrupa as portas/binds dos dois binários.
	Admin  ServerConfig `yaml:"admin"`
	Portal ServerConfig `yaml:"portal"`

	// Paths de dados locais.
	Paths PathsConfig `yaml:"paths"`

	// Auth do admin (validação própria, defense in depth sobre o Pangolin).
	AdminAuth AdminAuthConfig `yaml:"admin_auth"`

	// Google OAuth + integrações.
	Google GoogleConfig `yaml:"google"`

	// Crypto: chave de cifragem de tokens e backup.
	Crypto CryptoConfig `yaml:"crypto"`

	// Logging.
	Log LogConfig `yaml:"log"`

	// Reminders default do Google Calendar (acumulativos).
	Reminders RemindersConfig `yaml:"reminders"`

	// RateLimit das rotas públicas do portal.
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// ServerConfig descreve o bind HTTP de um binário.
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Addr devolve o endereço no formato host:port.
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// PathsConfig agrupa os caminhos de dados locais.
type PathsConfig struct {
	SQLite  string `yaml:"sqlite"`   // arquivo .db
	GEDRoot string `yaml:"ged_root"` // raiz do GED (segregado por paciente)
	LogDir  string `yaml:"log_dir"`  // diretório de logs (rotação diária)
}

// AdminAuthConfig são as credenciais que o admin valida por conta própria.
type AdminAuthConfig struct {
	Email  string `yaml:"email"`  // email do terapeuta (header via Pangolin)
	Secret string `yaml:"secret"` // secret compartilhado
	// Headers customizáveis (Pangolin pode reescrever).
	EmailHeader  string `yaml:"email_header"`
	SecretHeader string `yaml:"secret_header"`
}

// GoogleConfig são as credenciais OAuth e parâmetros de integração.
type GoogleConfig struct {
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`
	// CalendarID: geralmente "primary".
	CalendarID string `yaml:"calendar_id"`
	// DriveFolder: pasta lógica de backup no Drive.
	DriveFolder string `yaml:"drive_folder"`
}

// CryptoConfig carrega a chave de cifragem (backup + tokens).
// No MVP vem do config, atrás da interface KeyProvider (docs/architecture.md §4.4).
type CryptoConfig struct {
	// Key é a chave base64 (32 bytes → AES-256-GCM).
	Key string `yaml:"key"`
}

// LogConfig parametriza o logger.
type LogConfig struct {
	Level string `yaml:"level"` // debug|info|warn|error
}

// RemindersConfig: intervalos (em minutos) antes do evento. Default 1440 + 30.
type RemindersConfig struct {
	MinutesBefore []int `yaml:"minutes_before"`
}

// RateLimitConfig: limites das rotas públicas do portal.
type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	Burst             int `yaml:"burst"`
}

// Load lê e valida o arquivo de configuração no caminho informado.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("lendo config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parseando config %q: %w", path, err)
	}

	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// applyDefaults preenche valores default para campos não informados.
func (c *Config) applyDefaults() {
	if c.Admin.Host == "" {
		c.Admin.Host = "127.0.0.1"
	}
	if c.Admin.Port == 0 {
		c.Admin.Port = 8080
	}
	if c.Portal.Host == "" {
		c.Portal.Host = "127.0.0.1"
	}
	if c.Portal.Port == 0 {
		c.Portal.Port = 8081
	}
	if c.AdminAuth.EmailHeader == "" {
		c.AdminAuth.EmailHeader = "X-Pangolin-Email"
	}
	if c.AdminAuth.SecretHeader == "" {
		c.AdminAuth.SecretHeader = "X-Pangolin-Secret"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Google.CalendarID == "" {
		c.Google.CalendarID = "primary"
	}
	if len(c.Google.Scopes) == 0 {
		c.Google.Scopes = []string{
			"https://www.googleapis.com/auth/calendar",
			"https://www.googleapis.com/auth/gmail.send",
			"https://www.googleapis.com/auth/drive.file",
		}
	}
	if len(c.Reminders.MinutesBefore) == 0 {
		c.Reminders.MinutesBefore = []int{1440, 30} // 1 dia + 30 min
	}
	if c.RateLimit.RequestsPerMinute == 0 {
		c.RateLimit.RequestsPerMinute = 30
	}
	if c.RateLimit.Burst == 0 {
		c.RateLimit.Burst = 10
	}
}

// Validate garante que campos obrigatórios estão presentes e coerentes.
func (c *Config) Validate() error {
	var errs []error

	if c.Paths.SQLite == "" {
		errs = append(errs, errors.New("paths.sqlite é obrigatório"))
	}
	if c.Paths.GEDRoot == "" {
		errs = append(errs, errors.New("paths.ged_root é obrigatório"))
	}
	if c.AdminAuth.Email == "" {
		errs = append(errs, errors.New("admin_auth.email é obrigatório"))
	}
	if c.AdminAuth.Secret == "" {
		errs = append(errs, errors.New("admin_auth.secret é obrigatório"))
	}
	for _, m := range c.Reminders.MinutesBefore {
		if m < 0 {
			errs = append(errs, errors.New("reminders.minutes_before não pode ter valor negativo"))
			break
		}
	}

	return errors.Join(errs...)
}

// ReminderDurations converte os minutos configurados em []time.Duration.
func (c *Config) ReminderDurations() []time.Duration {
	out := make([]time.Duration, 0, len(c.Reminders.MinutesBefore))
	for _, m := range c.Reminders.MinutesBefore {
		out = append(out, time.Duration(m)*time.Minute)
	}
	return out
}
