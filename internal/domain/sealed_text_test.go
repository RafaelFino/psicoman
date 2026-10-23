package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// fakeCipher é um Cipher de teste: "cifra" prefixando um marcador e invertendo a
// string, só para provar o round-trip pela borda sem depender do AES real.
type fakeCipher struct{}

func (fakeCipher) Seal(_ context.Context, plaintext string) (string, error) {
	return "FAKE:" + reverse(plaintext), nil
}

func (fakeCipher) Open(_ context.Context, envelope string) (string, error) {
	if !strings.HasPrefix(envelope, "FAKE:") {
		return "", fmt.Errorf("envelope fake inválido")
	}
	return reverse(strings.TrimPrefix(envelope, "FAKE:")), nil
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func TestSealedTextRevealEmpty(t *testing.T) {
	st := NewSealedText("conteúdo clínico")
	if st.Reveal() != "conteúdo clínico" {
		t.Errorf("Reveal: quero %q, obtive %q", "conteúdo clínico", st.Reveal())
	}
	if st.Empty() {
		t.Error("não deveria estar vazio")
	}
	if !NewSealedText("").Empty() {
		t.Error("deveria estar vazio")
	}
}

// TestSealedTextNaoVazaNoJSON garante que serializar um struct com SealedText
// NÃO emite o claro: como o tipo não tem MarshalJSON e o campo plaintext é não
// exportado, o json.Marshal produz um objeto vazio para o SealedText.
func TestSealedTextNaoVazaNoJSON(t *testing.T) {
	const segredo = "paciente-relata-episodio-sensivel"
	st := NewSealedText(segredo)

	// Marshal direto do valor.
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("json.Marshal(SealedText): %v", err)
	}
	if strings.Contains(string(b), segredo) {
		t.Fatalf("json.Marshal vazou o claro: %s", b)
	}

	// Marshal dentro de um struct, como aconteceria por acidente num handler.
	type wrapper struct {
		Nota SealedText `json:"nota"`
	}
	b2, err := json.Marshal(wrapper{Nota: st})
	if err != nil {
		t.Fatalf("json.Marshal(wrapper): %v", err)
	}
	if strings.Contains(string(b2), segredo) {
		t.Fatalf("json.Marshal(wrapper) vazou o claro: %s", b2)
	}
}

// TestSealedTextNaoVazaNoLog garante que fmt (%v, %s, %+v, %#v) — caminho comum
// de log — nunca imprime o claro.
func TestSealedTextNaoVazaNoLog(t *testing.T) {
	const segredo = "conteudo-super-sensivel-123"
	st := NewSealedText(segredo)

	for _, formato := range []string{"%v", "%s", "%+v", "%#v", "%q"} {
		out := fmt.Sprintf(formato, st)
		if strings.Contains(out, segredo) {
			t.Errorf("fmt %q vazou o claro: %s", formato, out)
		}
	}

	// Também dentro de um struct maior.
	type agregado struct {
		ID   string
		Nota SealedText
	}
	out := fmt.Sprintf("%+v", agregado{ID: "01ABC", Nota: st})
	if strings.Contains(out, segredo) {
		t.Errorf("log do agregado vazou o claro: %s", out)
	}
}

func TestSealForOpenIntoRoundTrip(t *testing.T) {
	ctx := context.Background()
	c := fakeCipher{}
	original := NewSealedText("nota de sessão")

	env, err := SealFor(ctx, c, original)
	if err != nil {
		t.Fatalf("SealFor: %v", err)
	}
	if env == "" {
		t.Fatal("envelope vazio para conteúdo não vazio")
	}
	if strings.Contains(env, "nota de sessão") {
		t.Fatalf("envelope não deveria conter o claro legível: %q", env)
	}

	back, err := OpenInto(ctx, c, env)
	if err != nil {
		t.Fatalf("OpenInto: %v", err)
	}
	if back.Reveal() != original.Reveal() {
		t.Fatalf("round-trip divergiu: quero %q, obtive %q", original.Reveal(), back.Reveal())
	}
}

func TestSealForOpenIntoVazio(t *testing.T) {
	ctx := context.Background()
	c := fakeCipher{}

	// SealFor de vazio não chama a cifra e produz envelope vazio.
	env, err := SealFor(ctx, c, NewSealedText(""))
	if err != nil {
		t.Fatalf("SealFor vazio: %v", err)
	}
	if env != "" {
		t.Fatalf("envelope de vazio deveria ser vazio, obtive %q", env)
	}

	// OpenInto de vazio devolve SealedText vazio.
	st, err := OpenInto(ctx, c, "")
	if err != nil {
		t.Fatalf("OpenInto vazio: %v", err)
	}
	if !st.Empty() {
		t.Error("OpenInto de envelope vazio deveria devolver SealedText vazio")
	}
}
