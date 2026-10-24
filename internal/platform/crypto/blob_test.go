package crypto

import (
	"bytes"
	"testing"
)

func TestBlobSeal_RoundTrip(t *testing.T) {
	kp := newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)})
	b := NewBlobSealer(kp)

	plain := bytes.Repeat([]byte("snapshot-zip-conteúdo-binário\x00\x01\x02"), 500)
	env, err := b.SealBytes(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(env, plain) {
		t.Fatal("envelope não deveria conter o claro (vazamento)")
	}
	if !bytes.HasPrefix(env, []byte(blobMagic)) {
		t.Fatalf("envelope sem magic: %q", env[:4])
	}
	got, err := b.OpenBytes(env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("round-trip não recuperou o conteúdo original")
	}
}

func TestBlobSeal_NonceUnico(t *testing.T) {
	kp := newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)})
	b := NewBlobSealer(kp)
	plain := []byte("mesmo conteúdo")
	e1, _ := b.SealBytes(plain)
	e2, _ := b.SealBytes(plain)
	if bytes.Equal(e1, e2) {
		t.Fatal("dois selos do mesmo conteúdo não deveriam ser idênticos (nonce único)")
	}
}

func TestBlobSeal_Rotacao(t *testing.T) {
	// Cifra com k1; depois k2 vira corrente mas k1 segue resolvível.
	k1, k2 := genKeyB64(t), genKeyB64(t)
	bK1 := NewBlobSealer(newProvider(t, "k1", KeySpec{ID: "k1", B64Key: k1}))
	env, err := bK1.SealBytes([]byte("antigo"))
	if err != nil {
		t.Fatal(err)
	}
	// Provider com k2 corrente + k1 ainda presente.
	bRot := NewBlobSealer(newProvider(t, "k2",
		KeySpec{ID: "k2", B64Key: k2}, KeySpec{ID: "k1", B64Key: k1}))
	got, err := bRot.OpenBytes(env)
	if err != nil {
		t.Fatalf("deveria decifrar com a chave antiga (rotação): %v", err)
	}
	if string(got) != "antigo" {
		t.Fatalf("conteúdo errado após rotação: %q", got)
	}
}

func TestBlobSeal_ChaveErradaFalha(t *testing.T) {
	bK1 := NewBlobSealer(newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)}))
	env, _ := bK1.SealBytes([]byte("segredo"))
	// Outro provider com o MESMO id k1 mas chave diferente.
	bOutra := NewBlobSealer(newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)}))
	if _, err := bOutra.OpenBytes(env); err == nil {
		t.Fatal("decifrar com chave errada deveria falhar")
	}
}

func TestBlobSeal_HeaderAdulteradoFalha(t *testing.T) {
	kp := newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)})
	b := NewBlobSealer(kp)
	env, _ := b.SealBytes([]byte("conteúdo"))
	// Corrompe um byte do keyID (dentro do header, que é AAD): GCM deve falhar.
	tampered := make([]byte, len(env))
	copy(tampered, env)
	tampered[len(blobMagic)+1] ^= 0xFF
	if _, err := b.OpenBytes(tampered); err == nil {
		t.Fatal("header adulterado deveria falhar a autenticação")
	}
}

func TestBlobSeal_EnvelopeMalformado(t *testing.T) {
	kp := newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)})
	b := NewBlobSealer(kp)
	casos := [][]byte{
		nil,
		[]byte("XX"),
		[]byte("PSB1"),                    // sem keyIDLen
		append([]byte(blobMagic), 5, 'k'), // keyIDLen=5 mas só 1 byte de keyID
	}
	for i, c := range casos {
		if _, err := b.OpenBytes(c); err == nil {
			t.Fatalf("caso %d: envelope malformado deveria falhar", i)
		}
	}
}
