package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// yamlFixture espelha config.dev.yaml (formato atual) com valores de teste.
const yamlFixture = `
data_dir: "./data-dev"
log:
  level: "debug"
crypto:
  key_id: "k1"
  keys:
    k1: "D5XFpp6YiebdlbZ5Zi6madpJh38YnhPEUkGho9JHBjA="
pangolin:
  identity_header: "X-Pangolin-User"
google:
  client_id: "cid"
  client_secret: "csecret"
  calendar_id: "cal@group"
  drive_folder: "folder123"
llm:
  gemini:
    api_key: "SEGREDO_GEMINI_DO_ARQUIVO"
`

// writeFixture grava o YAML num arquivo temporário e devolve seu caminho.
func writeFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.dev.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("escrever fixture: %v", err)
	}
	return p
}

// originOf devolve a origem reportada para uma chave lógica.
func originOf(origins []OriginEvent, key string) string {
	for _, o := range origins {
		if o.Key == key {
			return o.Source
		}
	}
	return ""
}

// TestLoadDecodeTodosCampos: (a) decode mapeando todos os campos, incluindo o
// aninhado llm.gemini.api_key -> Gemini.APIKey.
func TestLoadDecodeTodosCampos(t *testing.T) {
	p := writeFixture(t, yamlFixture)
	cfg, _, err := Load(Options{Path: p})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataDir != "./data-dev" {
		t.Errorf("DataDir: veio %q", cfg.DataDir)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level: veio %q", cfg.Log.Level)
	}
	if cfg.Crypto.KeyID != "k1" {
		t.Errorf("Crypto.KeyID: veio %q", cfg.Crypto.KeyID)
	}
	if cfg.Crypto.Keys["k1"] != "D5XFpp6YiebdlbZ5Zi6madpJh38YnhPEUkGho9JHBjA=" {
		t.Errorf("Crypto.Keys[k1]: veio %q", cfg.Crypto.Keys["k1"])
	}
	if cfg.Pangolin.IdentityHeader != "X-Pangolin-User" {
		t.Errorf("Pangolin.IdentityHeader: veio %q", cfg.Pangolin.IdentityHeader)
	}
	if cfg.Google.ClientID != "cid" || cfg.Google.ClientSecret != "csecret" ||
		cfg.Google.CalendarID != "cal@group" || cfg.Google.DriveFolder != "folder123" {
		t.Errorf("Google: veio %+v", cfg.Google)
	}
	if cfg.Gemini.APIKey != "SEGREDO_GEMINI_DO_ARQUIVO" {
		t.Errorf("Gemini.APIKey: veio %q", cfg.Gemini.APIKey)
	}
}

// TestLoadEnvSobrescreveArquivo: (b) env sobrescreve o arquivo e a origem
// reportada é "env".
func TestLoadEnvSobrescreveArquivo(t *testing.T) {
	p := writeFixture(t, yamlFixture)
	t.Setenv(EnvCipherKeyID, "k2")
	t.Setenv(EnvCipherKeys, "k2:QkFTRTY0LURPLUVOVi1OQU8tVkFMSURBRE8tQUtJ")
	t.Setenv(EnvPangolinHeader, "X-Custom-Header")
	t.Setenv(EnvGeminiAPIKey, "SEGREDO_GEMINI_DO_ENV")

	cfg, origins, err := Load(Options{Path: p})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Crypto.KeyID != "k2" {
		t.Errorf("env deveria sobrescrever crypto.key_id: %q", cfg.Crypto.KeyID)
	}
	if cfg.Crypto.Keys["k2"] == "" || cfg.Crypto.Keys["k1"] != "" {
		t.Errorf("env deveria substituir o mapa de chaves: %+v", cfg.Crypto.Keys)
	}
	if cfg.Pangolin.IdentityHeader != "X-Custom-Header" {
		t.Errorf("env deveria sobrescrever pangolin.identity_header: %q", cfg.Pangolin.IdentityHeader)
	}
	if cfg.Gemini.APIKey != "SEGREDO_GEMINI_DO_ENV" {
		t.Errorf("env deveria sobrescrever llm.gemini.api_key: %q", cfg.Gemini.APIKey)
	}
	for _, k := range []string{"crypto.key_id", "crypto.keys", "pangolin.identity_header", "llm.gemini.api_key"} {
		if got := originOf(origins, k); got != SourceEnv {
			t.Errorf("origem de %s: quero %q, veio %q", k, SourceEnv, got)
		}
	}
	// Uma chave sem env deve reportar "arquivo".
	if got := originOf(origins, "google.calendar_id"); got != SourceFile {
		t.Errorf("origem de google.calendar_id: quero %q, veio %q", SourceFile, got)
	}
}

// TestGeminiEnabled: (c) Enabled() true com chave, false sem.
func TestGeminiEnabled(t *testing.T) {
	if !(GeminiConfig{APIKey: "x"}).Enabled() {
		t.Error("Enabled deveria ser true com chave")
	}
	if (GeminiConfig{APIKey: "   "}).Enabled() {
		t.Error("Enabled deveria ser false com chave em branco")
	}
	if (GeminiConfig{}).Enabled() {
		t.Error("Enabled deveria ser false sem chave")
	}
}

// TestLoadAusenciaDeArquivoNaoEhFatal: (d) sem arquivo, cai para defaults + env.
func TestLoadAusenciaDeArquivoNaoEhFatal(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nao-existe.yaml")
	t.Setenv(EnvGeminiAPIKey, "SO_DO_ENV")

	cfg, origins, err := Load(Options{Path: missing})
	if err != nil {
		t.Fatalf("ausência de arquivo não deveria ser fatal: %v", err)
	}
	if cfg.Gemini.APIKey != "SO_DO_ENV" || !cfg.Gemini.Enabled() {
		t.Errorf("env deveria valer sem arquivo: %+v", cfg.Gemini)
	}
	if got := originOf(origins, "llm.gemini.api_key"); got != SourceEnv {
		t.Errorf("origem gemini: quero %q, veio %q", SourceEnv, got)
	}
	// Sem arquivo e sem env, a origem é "default".
	if got := originOf(origins, "crypto.key_id"); got != SourceDefault {
		t.Errorf("origem crypto.key_id sem arquivo/env: quero %q, veio %q", SourceDefault, got)
	}
}

// TestOriginEventsNaoVazamSegredo: (e) nenhum valor de segredo aparece nos
// OriginEvent — só a chave lógica e a origem.
func TestOriginEventsNaoVazamSegredo(t *testing.T) {
	p := writeFixture(t, yamlFixture)
	t.Setenv(EnvCipherKeys, "k9:SEGREDO_CIFRA_DO_ENV")
	t.Setenv(EnvGeminiAPIKey, "SEGREDO_GEMINI_DO_ENV")

	_, origins, err := Load(Options{Path: p})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	segredos := []string{
		"SEGREDO_GEMINI_DO_ARQUIVO",
		"SEGREDO_GEMINI_DO_ENV",
		"SEGREDO_CIFRA_DO_ENV",
		"D5XFpp6YiebdlbZ5Zi6madpJh38YnhPEUkGho9JHBjA=",
		"csecret",
	}
	for _, o := range origins {
		for _, s := range segredos {
			if strings.Contains(o.Key, s) || strings.Contains(o.Source, s) {
				t.Errorf("segredo %q vazou no OriginEvent %+v", s, o)
			}
		}
	}
}

// TestResolvePathDescobreCandidatoNoCwd: a busca padrão encontra config.dev.yaml
// no cwd quando não há Path explícito nem PSICOMAN_CONFIG.
func TestResolvePathDescobreCandidatoNoCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.dev.yaml"), []byte(yamlFixture), 0o600); err != nil {
		t.Fatalf("escrever fixture: %v", err)
	}
	t.Chdir(dir)

	cfg, _, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Crypto.KeyID != "k1" {
		t.Errorf("deveria ter carregado config.dev.yaml do cwd: %+v", cfg.Crypto)
	}
}
