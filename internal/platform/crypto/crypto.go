// Package crypto cifra e decifra segredos (tokens do Google, snapshots de
// backup) com AES-256-GCM. A chave vem de um KeyProvider, que no MVP lê do
// config mas é plugável para um vault futuro (docs/architecture.md §4.4).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// KeyProvider fornece a chave de cifragem. Abstrai a fonte (config, vault).
type KeyProvider interface {
	// Key devolve a chave de 32 bytes (AES-256).
	Key() ([]byte, error)
}

// StaticKeyProvider lê a chave base64 do config.
type StaticKeyProvider struct {
	B64 string
}

// Key decodifica e valida a chave base64 (deve ter 32 bytes).
func (p StaticKeyProvider) Key() ([]byte, error) {
	if p.B64 == "" {
		return nil, errors.New("crypto: chave de cifragem não configurada")
	}
	key, err := base64.StdEncoding.DecodeString(p.B64)
	if err != nil {
		return nil, fmt.Errorf("crypto: chave base64 inválida: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: chave deve ter 32 bytes (AES-256), tem %d", len(key))
	}
	return key, nil
}

// Cipher cifra/decifra com AES-256-GCM usando a chave do provider.
type Cipher struct {
	provider KeyProvider
}

// New cria um Cipher a partir de um KeyProvider.
func New(provider KeyProvider) *Cipher {
	return &Cipher{provider: provider}
}

// Encrypt cifra plaintext, devolvendo base64(nonce || ciphertext).
func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	gcm, err := c.gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: gerando nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverte Encrypt.
func (c *Cipher) Decrypt(encoded string) ([]byte, error) {
	gcm, err := c.gcm()
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("crypto: base64 inválido: %w", err)
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return nil, errors.New("crypto: dado cifrado muito curto")
	}
	nonce, ciphertext := data[:ns], data[ns:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: falha ao decifrar: %w", err)
	}
	return plaintext, nil
}

func (c *Cipher) gcm() (cipher.AEAD, error) {
	key, err := c.provider.Key()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: criando cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

// GenerateKey gera uma chave AES-256 aleatória em base64 (utilitário/setup).
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
