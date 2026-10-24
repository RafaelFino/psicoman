package crypto

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

// genKeyB64 gera uma chave AES-256 aleatória em base64 (padrão) para os testes.
func genKeyB64(t *testing.T) string {
	t.Helper()
	raw := make([]byte, keySize)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("gerando chave: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func newProvider(t *testing.T, currentID string, specs ...KeySpec) *MapKeyProvider {
	t.Helper()
	p, err := NewMapKeyProvider(currentID, specs)
	if err != nil {
		t.Fatalf("NewMapKeyProvider: %v", err)
	}
	return p
}

func TestSealOpenRoundTrip(t *testing.T) {
	kp := newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)})
	c := New(kp)
	ctx := context.Background()

	casos := []string{
		"",
		"anamnese: paciente relata ansiedade",
		"linha 1\nlinha 2\nacentuação: ção, ã, é",
		strings.Repeat("x", 10_000),
	}
	for _, claro := range casos {
		env, err := c.Seal(ctx, claro)
		if err != nil {
			t.Fatalf("Seal(%q): %v", claro, err)
		}
		got, err := c.Open(ctx, env)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if got != claro {
			t.Fatalf("round-trip divergiu: quero %q, obtive %q", claro, got)
		}
	}
}

func TestEnvelopeFormato(t *testing.T) {
	kp := newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)})
	c := New(kp)

	env, err := c.Seal(context.Background(), "segredo clínico")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	parts := strings.Split(env, ".")
	if len(parts) != envelopeParts {
		t.Fatalf("envelope deve ter %d partes, tem %d: %q", envelopeParts, len(parts), env)
	}
	if parts[0] != schemaVersion {
		t.Errorf("versao_do_esquema: quero %q, obtive %q", schemaVersion, parts[0])
	}
	if parts[1] != algorithm {
		t.Errorf("algoritmo: quero %q, obtive %q", algorithm, parts[1])
	}
	// O claro nunca aparece no envelope.
	if strings.Contains(env, "segredo") || strings.Contains(env, "clínico") {
		t.Errorf("envelope vazou o claro: %q", env)
	}
}

func TestNonceUnicoPorOperacao(t *testing.T) {
	kp := newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)})
	c := New(kp)
	ctx := context.Background()

	const n = 50
	vistos := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		env, err := c.Seal(ctx, "mesmo texto")
		if err != nil {
			t.Fatalf("Seal: %v", err)
		}
		if vistos[env] {
			t.Fatalf("envelope repetido na iteração %d — nonce não é único", i)
		}
		vistos[env] = true
	}
}

func TestRotacaoDeChave(t *testing.T) {
	// Duas chaves distintas; k1 cifra primeiro, depois a corrente vira k2.
	k1 := KeySpec{ID: "k1", B64Key: genKeyB64(t)}
	k2 := KeySpec{ID: "k2", B64Key: genKeyB64(t)}
	ctx := context.Background()

	// Provider com k1 corrente cifra um dado antigo.
	antigo := New(newProvider(t, "k1", k1))
	envAntigo, err := antigo.Seal(ctx, "registro antigo")
	if err != nil {
		t.Fatalf("Seal antigo: %v", err)
	}

	// Rotaciona: k2 passa a ser a corrente, mas k1 continua conhecida.
	rotacionado := New(newProvider(t, "k2", k1, k2))

	// Dado antigo (cifrado com k1) ainda decifra após a rotação.
	got, err := rotacionado.Open(ctx, envAntigo)
	if err != nil {
		t.Fatalf("Open do envelope antigo após rotação: %v", err)
	}
	if got != "registro antigo" {
		t.Fatalf("quero %q, obtive %q", "registro antigo", got)
	}

	// Dado novo é cifrado com a chave corrente (k2).
	envNovo, err := rotacionado.Seal(ctx, "registro novo")
	if err != nil {
		t.Fatalf("Seal novo: %v", err)
	}
	if id := strings.Split(envNovo, ".")[2]; decodeID(t, id) != "k2" {
		t.Errorf("dado novo deveria usar a chave corrente k2, usou %q", decodeID(t, id))
	}
}

func decodeID(t *testing.T, b64ID string) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(b64ID)
	if err != nil {
		t.Fatalf("decodificando id: %v", err)
	}
	return string(raw)
}

func TestOpenComChaveErradaFalha(t *testing.T) {
	ctx := context.Background()
	// Cifra com k1.
	origem := New(newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)}))
	env, err := origem.Seal(ctx, "dado")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Outro provider onde o id "k1" existe mas com OUTRA chave: a autenticação
	// GCM deve falhar.
	impostor := New(newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)}))
	if _, err := impostor.Open(ctx, env); err == nil {
		t.Fatal("Open deveria falhar com chave errada, mas não falhou")
	}
}

func TestOpenEnvelopeAdulteradoFalha(t *testing.T) {
	ctx := context.Background()
	c := New(newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)}))
	env, err := c.Seal(ctx, "dado íntegro")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Adultera 1 byte do ciphertext.
	parts := strings.Split(env, ".")
	ct, _ := base64.RawURLEncoding.DecodeString(parts[4])
	ct[0] ^= 0xFF
	parts[4] = base64.RawURLEncoding.EncodeToString(ct)
	adulterado := strings.Join(parts, ".")

	if _, err := c.Open(ctx, adulterado); err == nil {
		t.Fatal("Open deveria falhar em envelope adulterado (GCM), mas não falhou")
	}
}

func TestOpenEnvelopeMalformado(t *testing.T) {
	c := New(newProvider(t, "k1", KeySpec{ID: "k1", B64Key: genKeyB64(t)}))
	ctx := context.Background()
	casos := []string{
		"",
		"sem-pontos",
		"a.b.c.d",              // 4 partes
		"v2.aes-256-gcm.a.b.c", // esquema desconhecido
		"v1.chacha.a.b.c",      // algoritmo desconhecido
	}
	for _, env := range casos {
		if _, err := c.Open(ctx, env); err == nil {
			t.Errorf("Open(%q) deveria falhar, mas não falhou", env)
		}
	}
}

func TestNewMapKeyProviderValida(t *testing.T) {
	ok := genKeyB64(t)
	casos := []struct {
		nome    string
		current string
		specs   []KeySpec
		querErr bool
	}{
		{"ok", "k1", []KeySpec{{ID: "k1", B64Key: ok}}, false},
		{"sem corrente", "", []KeySpec{{ID: "k1", B64Key: ok}}, true},
		{"sem chaves", "k1", nil, true},
		{"corrente ausente", "k2", []KeySpec{{ID: "k1", B64Key: ok}}, true},
		{"id vazio", "k1", []KeySpec{{ID: "", B64Key: ok}}, true},
		{"base64 inválido", "k1", []KeySpec{{ID: "k1", B64Key: "não-base64!!"}}, true},
		{"tamanho errado", "k1", []KeySpec{{ID: "k1", B64Key: base64.StdEncoding.EncodeToString([]byte("curta"))}}, true},
	}
	for _, c := range casos {
		_, err := NewMapKeyProvider(c.current, c.specs)
		if c.querErr && err == nil {
			t.Errorf("%s: esperava erro, não veio", c.nome)
		}
		if !c.querErr && err != nil {
			t.Errorf("%s: erro inesperado: %v", c.nome, err)
		}
	}
}
