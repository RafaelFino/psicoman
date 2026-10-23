package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// Formato do envelope (D6.4): cinco campos separados por ponto.
//
//	versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext
//
// Os três últimos campos (id_da_chave, nonce, ciphertext) são base64 (raw URL,
// sem padding, para não colidir com o separador "."). versao_do_esquema e
// algoritmo são tokens fixos que habilitam evolução sem quebrar dados antigos.
const (
	schemaVersion = "v1"          // versao_do_esquema
	algorithm     = "aes-256-gcm" // algoritmo
	envelopeParts = 5
	keySize       = 32 // AES-256
)

var b64 = base64.RawURLEncoding

// Erros expostos (mensagens técnicas; a tradução PT-BR para o usuário é
// responsabilidade da camada de API ao montar o envelope de resposta).
var (
	ErrEnvelopeInvalido  = errors.New("envelope de cifra inválido")
	ErrEsquemaDesconheci = errors.New("versão de esquema de cifra desconhecida")
	ErrAlgoritmoDesconh  = errors.New("algoritmo de cifra desconhecido")
	ErrChaveInvalida     = errors.New("chave de cifra inválida (esperado 32 bytes para AES-256)")
)

// aesCipher implementa domain.Cipher com AES-256-GCM, resolvendo chaves via
// KeyProvider. É o único lugar que conhece o primitivo de cripto.
type aesCipher struct {
	keys domain.KeyProvider
}

// New cria um domain.Cipher AES-256-GCM sobre o KeyProvider informado.
func New(keys domain.KeyProvider) domain.Cipher {
	return &aesCipher{keys: keys}
}

// Seal cifra o claro com a chave CORRENTE e devolve o envelope versionado.
// Gera um nonce aleatório único por operação (requisito do GCM).
func (c *aesCipher) Seal(ctx context.Context, plaintext string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	keyID := c.keys.CurrentID()
	gcm, err := c.gcmFor(keyID)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("gerando nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	return strings.Join([]string{
		schemaVersion,
		algorithm,
		b64.EncodeToString([]byte(keyID)),
		b64.EncodeToString(nonce),
		b64.EncodeToString(ciphertext),
	}, "."), nil
}

// Open decifra o envelope, resolvendo a chave pelo id_da_chave nele embutido
// (habilita rotação: um envelope antigo é lido com a chave antiga mesmo após a
// corrente ter mudado).
func (c *aesCipher) Open(ctx context.Context, envelope string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	parts := strings.Split(envelope, ".")
	if len(parts) != envelopeParts {
		return "", ErrEnvelopeInvalido
	}
	schema, algo, keyIDb64, nonceB64, ctB64 := parts[0], parts[1], parts[2], parts[3], parts[4]

	if schema != schemaVersion {
		return "", fmt.Errorf("%w: %q", ErrEsquemaDesconheci, schema)
	}
	if algo != algorithm {
		return "", fmt.Errorf("%w: %q", ErrAlgoritmoDesconh, algo)
	}

	keyIDBytes, err := b64.DecodeString(keyIDb64)
	if err != nil {
		return "", fmt.Errorf("%w: id_da_chave", ErrEnvelopeInvalido)
	}
	nonce, err := b64.DecodeString(nonceB64)
	if err != nil {
		return "", fmt.Errorf("%w: nonce", ErrEnvelopeInvalido)
	}
	ciphertext, err := b64.DecodeString(ctB64)
	if err != nil {
		return "", fmt.Errorf("%w: ciphertext", ErrEnvelopeInvalido)
	}

	gcm, err := c.gcmFor(string(keyIDBytes))
	if err != nil {
		return "", err
	}
	if len(nonce) != gcm.NonceSize() {
		return "", fmt.Errorf("%w: tamanho de nonce", ErrEnvelopeInvalido)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Chave errada, dado corrompido ou adulterado: o GCM falha a
		// autenticação. Não distinguir para não dar pista a um atacante.
		return "", fmt.Errorf("falha ao decifrar: %w", err)
	}
	return string(plaintext), nil
}

// gcmFor resolve a chave pelo id e monta o AEAD AES-256-GCM.
func (c *aesCipher) gcmFor(keyID string) (cipher.AEAD, error) {
	key, err := c.keys.Key(keyID)
	if err != nil {
		return nil, fmt.Errorf("resolvendo chave %q: %w", keyID, err)
	}
	if len(key) != keySize {
		return nil, ErrChaveInvalida
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("montando cifra AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("montando GCM: %w", err)
	}
	return gcm, nil
}
