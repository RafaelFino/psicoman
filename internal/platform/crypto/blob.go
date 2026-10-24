package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// Cifra de BLOB (arquivo), distinta da cifra de CAMPO (aesCipher/domain.Cipher).
//
// A cifra de campo produz um envelope TEXTO base64 (ideal para uma nota clínica
// curta persistida numa coluna). Um snapshot de backup é um zip binário grande:
// aplicar o envelope-texto nele desperdiçaria memória e misturaria a semântica
// de "campo" com "arquivo". Este helper cifra BYTES com o MESMO primitivo
// (AES-256-GCM) e as MESMAS chaves (domain.KeyProvider) — sem chave nova, sem
// config nova —, num envelope BINÁRIO compacto (D9, psicoman-google-api:
// "snapshot cifrado AES-256-GCM antes do upload"; psicoman-seguranca-lgpd).
//
// Envelope binário (big-endian onde aplicável):
//
//	magic[4]="PSB1" | keyIDLen[1] | keyID[keyIDLen] | nonce[gcm.NonceSize] | ciphertext[...]
//
// O keyID embutido habilita rotação na restauração (decifra com a chave da
// época), igual ao envelope de campo.
const (
	blobMagic      = "PSB1" // Psicoman Snapshot Blob v1
	blobMaxKeyID   = 255    // keyIDLen cabe em 1 byte
	blobMinKeySize = 32     // AES-256
)

// ErrBlobInvalido indica um envelope de blob malformado (magic/estrutura).
var ErrBlobInvalido = errors.New("envelope de blob cifrado inválido")

// BlobSealer cifra/decifra BYTES (snapshots de backup) com AES-256-GCM sobre um
// domain.KeyProvider. Reusa as chaves da cifra de campo (mesmo KeyProvider),
// mantendo rotação pelo keyID embutido no envelope binário.
type BlobSealer struct {
	keys domain.KeyProvider
}

// NewBlobSealer cria o sealer de blobs sobre o KeyProvider informado.
func NewBlobSealer(keys domain.KeyProvider) *BlobSealer {
	return &BlobSealer{keys: keys}
}

// SealBytes cifra plaintext com a chave CORRENTE e devolve o envelope binário.
// Gera um nonce aleatório único por operação (requisito do GCM).
func (b *BlobSealer) SealBytes(plaintext []byte) ([]byte, error) {
	keyID := b.keys.CurrentID()
	if len(keyID) == 0 || len(keyID) > blobMaxKeyID {
		return nil, fmt.Errorf("%w: id de chave com tamanho inválido", ErrBlobInvalido)
	}
	gcm, err := b.gcmFor(keyID)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("gerando nonce: %w", err)
	}
	// header: magic + keyIDLen + keyID. O ciphertext é anexado depois (Seal
	// usa o header como additional data para autenticar a integridade do
	// envelope, não só do conteúdo).
	header := make([]byte, 0, len(blobMagic)+1+len(keyID))
	header = append(header, blobMagic...)
	header = append(header, byte(len(keyID)))
	header = append(header, keyID...)

	out := make([]byte, 0, len(header)+len(nonce)+len(plaintext)+gcm.Overhead())
	out = append(out, header...)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plaintext, header)
	return out, nil
}

// OpenBytes decifra um envelope binário produzido por SealBytes, resolvendo a
// chave pelo keyID embutido (rotação). Autentica também o header (AAD).
func (b *BlobSealer) OpenBytes(envelope []byte) ([]byte, error) {
	magicLen := len(blobMagic)
	if len(envelope) < magicLen+1 || string(envelope[:magicLen]) != blobMagic {
		return nil, ErrBlobInvalido
	}
	keyIDLen := int(envelope[magicLen])
	off := magicLen + 1
	if keyIDLen == 0 || len(envelope) < off+keyIDLen {
		return nil, ErrBlobInvalido
	}
	keyID := string(envelope[off : off+keyIDLen])
	off += keyIDLen

	gcm, err := b.gcmFor(keyID)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(envelope) < off+ns {
		return nil, ErrBlobInvalido
	}
	nonce := envelope[off : off+ns]
	ciphertext := envelope[off+ns:]
	header := envelope[:off] // magic + keyIDLen + keyID (AAD)

	plaintext, err := gcm.Open(nil, nonce, ciphertext, header)
	if err != nil {
		return nil, fmt.Errorf("falha ao decifrar snapshot: %w", err)
	}
	return plaintext, nil
}

// gcmFor resolve a chave pelo id e monta o AEAD AES-256-GCM (espelha aesCipher).
func (b *BlobSealer) gcmFor(keyID string) (cipher.AEAD, error) {
	key, err := b.keys.Key(keyID)
	if err != nil {
		return nil, fmt.Errorf("resolvendo chave %q: %w", keyID, err)
	}
	if len(key) != blobMinKeySize {
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
