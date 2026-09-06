package crypto

import (
	"bytes"
	"testing"
)

func testCipher(t *testing.T) *Cipher {
	t.Helper()
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return New(StaticKeyProvider{B64: key})
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c := testCipher(t)
	plain := []byte("refresh-token-super-secreto")
	enc, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == string(plain) {
		t.Error("texto cifrado igual ao claro")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Errorf("decifrado = %q, quer %q", dec, plain)
	}
}

func TestEncryptProducesDistinctCiphertexts(t *testing.T) {
	c := testCipher(t)
	a, _ := c.Encrypt([]byte("mesmo texto"))
	b, _ := c.Encrypt([]byte("mesmo texto"))
	if a == b {
		t.Error("nonce não aleatório: ciphertexts idênticos")
	}
}

func TestInvalidKey(t *testing.T) {
	c := New(StaticKeyProvider{B64: "chave-curta"})
	if _, err := c.Encrypt([]byte("x")); err == nil {
		t.Error("esperava erro com chave inválida")
	}
}

func TestDecryptTamperedFails(t *testing.T) {
	c := testCipher(t)
	enc, _ := c.Encrypt([]byte("dados"))
	// Adultera o último caractere.
	tampered := enc[:len(enc)-2] + "AA"
	if _, err := c.Decrypt(tampered); err == nil {
		t.Error("esperava falha ao decifrar dado adulterado")
	}
}
