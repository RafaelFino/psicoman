// Package config é o loader de configuração do binário único do Psicoman.
//
// É um pacote de BORDA PURO: usa apenas a stdlib + gopkg.in/yaml.v3. NÃO importa
// o PocketBase nem internal/platform/pb (garantido pelo archtest). Quem liga a
// config ao framework é o wiring (internal/api) e o main (cmd/psicoman).
//
// Modelo (decisões do Rafael, Fase 6, Opção B):
//   - A config vem de um arquivo YAML (dev: config.dev.yaml, git-ignored).
//   - Variáveis de ambiente SOBRESCREVEM o arquivo (precedência env > arquivo).
//   - A origem de cada chave (arquivo | env | default) é reportada ao chamador
//     via []OriginEvent para ser LOGADA, sem que este pacote faça I/O de log
//     acoplado — mantendo config livre de dependência de logger do framework.
//   - Gemini é opt-in pela PRESENÇA de llm.gemini.api_key (R10.3): com chave, a
//     IA liga; sem chave, desliga. Não há flag on/off separada.
//   - NENHUM valor de segredo (crypto.keys, gemini.api_key, google.client_secret)
//     é exposto nos OriginEvent: só a chave lógica e a origem (psicoman-seguranca-lgpd).
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config é o struct tipado que espelha config.dev.yaml (formato atual).
type Config struct {
	DataDir  string
	Log      LogConfig
	Crypto   CryptoConfig
	Pangolin PangolinConfig
	Google   GoogleConfig
	Gemini   GeminiConfig
}

// LogConfig espelha o bloco `log`.
type LogConfig struct {
	Level string
}

// CryptoConfig espelha o bloco `crypto` (cifra por campo).
type CryptoConfig struct {
	KeyID string            // crypto.key_id — id da chave corrente.
	Keys  map[string]string // crypto.keys — id -> base64(32 bytes).
}

// PangolinConfig espelha o bloco `pangolin`.
type PangolinConfig struct {
	IdentityHeader string
}

// GoogleConfig espelha o bloco `google`.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	CalendarID   string
	DriveFolder  string
}

// GeminiConfig espelha o bloco `llm.gemini`.
type GeminiConfig struct {
	APIKey string
}

// Enabled reporta o opt-in da IA pela PRESENÇA da api_key (R10.3; decisão
// Rafael #1): com chave não-vazia a IA liga; sem chave desliga (default off).
func (g GeminiConfig) Enabled() bool { return strings.TrimSpace(g.APIKey) != "" }

// OriginEvent descreve de onde cada chave de config foi lida, para o chamador
// logar. NUNCA carrega o VALOR de segredos — só a chave lógica e a origem.
type OriginEvent struct {
	Key    string // chave lógica, ex.: "crypto.key_id", "llm.gemini.api_key"
	Source string // SourceFile | SourceEnv | SourceDefault
}

// Origens possíveis de uma config.
const (
	SourceFile    = "arquivo"
	SourceEnv     = "env"
	SourceDefault = "default"
)

// Variáveis de ambiente que sobrescrevem o arquivo (precedência env > arquivo).
const (
	EnvConfig         = "PSICOMAN_CONFIG"
	EnvCipherKeyID    = "PSICOMAN_CIPHER_KEY_ID"
	EnvCipherKeys     = "PSICOMAN_CIPHER_KEYS"
	EnvPangolinHeader = "PSICOMAN_PANGOLIN_IDENTITY_HEADER"
	EnvGeminiAPIKey   = "PSICOMAN_GEMINI_API_KEY"
)

// Options parametriza a descoberta do arquivo de config.
type Options struct {
	// Path é o caminho explícito do arquivo (ex.: do flag --config). Se vazio,
	// cai para PSICOMAN_CONFIG e, depois, para a busca padrão no cwd.
	Path string
}

// Candidatos padrão de arquivo no cwd, em ordem de preferência.
var defaultCandidates = []string{"config.dev.yaml", "config.yaml"}

// yamlDoc é o struct intermediário SÓ para decode do YAML aninhado. Isola as
// tags de serialização (incl. o agrupador `llm`) do struct público Config.
type yamlDoc struct {
	DataDir string `yaml:"data_dir"`
	Log     struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
	Crypto struct {
		KeyID string            `yaml:"key_id"`
		Keys  map[string]string `yaml:"keys"`
	} `yaml:"crypto"`
	Pangolin struct {
		IdentityHeader string `yaml:"identity_header"`
	} `yaml:"pangolin"`
	Google struct {
		ClientID     string `yaml:"client_id"`
		ClientSecret string `yaml:"client_secret"`
		CalendarID   string `yaml:"calendar_id"`
		DriveFolder  string `yaml:"drive_folder"`
	} `yaml:"google"`
	LLM struct {
		Gemini struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"gemini"`
	} `yaml:"llm"`
}

// Load lê a config do arquivo descoberto e aplica a precedência env > arquivo.
//
// Ordem de descoberta do arquivo:
//  1. opts.Path (flag --config), se informado;
//  2. env PSICOMAN_CONFIG, se setada;
//  3. ./config.dev.yaml no cwd;
//  4. ./config.yaml no cwd.
//
// A AUSÊNCIA de arquivo NÃO é fatal: cai para defaults + env (a cifra, porém,
// continua fail-closed no boot — responsabilidade do wiring, não deste pacote).
//
// Devolve o Config resolvido e os OriginEvent (origem de cada chave) para o
// chamador logar. Não loga nada nem faz I/O de log aqui (sem acoplar a logger).
func Load(opts Options) (Config, []OriginEvent, error) {
	path := resolvePath(opts)

	var doc yamlDoc
	fileLoaded := false
	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(data, &doc); err != nil {
				return Config{}, nil, fmt.Errorf("config: decode de %s: %w", path, err)
			}
			fileLoaded = true
		case os.IsNotExist(err):
			// Ausência não é fatal: segue com defaults + env.
		default:
			return Config{}, nil, fmt.Errorf("config: ler %s: %w", path, err)
		}
	}

	var origins []OriginEvent
	// resolve escolhe entre env e o valor do arquivo, registrando a origem.
	resolve := func(key, envName, fileVal string) string {
		if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
			origins = append(origins, OriginEvent{Key: key, Source: SourceEnv})
			return v
		}
		if strings.TrimSpace(fileVal) != "" && fileLoaded {
			origins = append(origins, OriginEvent{Key: key, Source: SourceFile})
			return fileVal
		}
		origins = append(origins, OriginEvent{Key: key, Source: SourceDefault})
		return fileVal
	}

	cfg := Config{
		DataDir: resolve("data_dir", "", doc.DataDir), // informativo: --dir do PB vence.
		Log:     LogConfig{Level: resolve("log.level", "", doc.Log.Level)},
		Crypto: CryptoConfig{
			KeyID: resolve("crypto.key_id", EnvCipherKeyID, doc.Crypto.KeyID),
			Keys:  resolveKeys(&origins, doc.Crypto.Keys, fileLoaded),
		},
		Pangolin: PangolinConfig{
			IdentityHeader: resolve("pangolin.identity_header", EnvPangolinHeader, doc.Pangolin.IdentityHeader),
		},
		Google: GoogleConfig{
			// google.* sem env por ora (decisão Fase 6): origem é sempre arquivo/default.
			ClientID:     resolve("google.client_id", "", doc.Google.ClientID),
			ClientSecret: resolve("google.client_secret", "", doc.Google.ClientSecret),
			CalendarID:   resolve("google.calendar_id", "", doc.Google.CalendarID),
			DriveFolder:  resolve("google.drive_folder", "", doc.Google.DriveFolder),
		},
		Gemini: GeminiConfig{
			APIKey: resolve("llm.gemini.api_key", EnvGeminiAPIKey, doc.LLM.Gemini.APIKey),
		},
	}
	return cfg, origins, nil
}

// resolvePath aplica a ordem de descoberta do arquivo e devolve o caminho a
// carregar ("" se nenhum candidato existe no cwd).
func resolvePath(opts Options) string {
	if p := strings.TrimSpace(opts.Path); p != "" {
		return p
	}
	if p := strings.TrimSpace(os.Getenv(EnvConfig)); p != "" {
		return p
	}
	for _, c := range defaultCandidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// resolveKeys aplica a precedência env > arquivo às chaves de cifra. O env
// PSICOMAN_CIPHER_KEYS usa o formato "id:base64,id2:base64" (mesmo parse
// histórico do loadCipherConfig do wiring). Registra só a origem, nunca o valor.
func resolveKeys(origins *[]OriginEvent, fileKeys map[string]string, fileLoaded bool) map[string]string {
	const key = "crypto.keys"
	if raw := strings.TrimSpace(os.Getenv(EnvCipherKeys)); raw != "" {
		*origins = append(*origins, OriginEvent{Key: key, Source: SourceEnv})
		return parseCipherKeys(raw)
	}
	if len(fileKeys) > 0 && fileLoaded {
		*origins = append(*origins, OriginEvent{Key: key, Source: SourceFile})
		return fileKeys
	}
	*origins = append(*origins, OriginEvent{Key: key, Source: SourceDefault})
	return fileKeys
}

// parseCipherKeys converte a lista "id:base64,id2:base64" em map[id]base64.
// Entradas vazias são ignoradas; a validação do base64/tamanho fica a cargo do
// KeyProvider no boot (fail-closed), fora deste pacote de borda.
func parseCipherKeys(raw string) map[string]string {
	keys := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, b64, ok := strings.Cut(part, ":")
		id = strings.TrimSpace(id)
		b64 = strings.TrimSpace(b64)
		if !ok || id == "" || b64 == "" {
			continue
		}
		keys[id] = b64
	}
	return keys
}
