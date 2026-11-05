package clock

import (
	"testing"
	"time"
)

func TestLocationIsSaoPauloOrFixedMinus3(t *testing.T) {
	loc := Location()
	if loc == nil {
		t.Fatal("Location() devolveu nil")
	}
	// -03:00 (padrão sem horário de verão, abolido no Brasil desde 2019).
	ref := time.Date(2026, 6, 1, 12, 0, 0, 0, loc)
	_, offset := ref.Zone()
	if offset != -3*60*60 {
		t.Errorf("offset esperado -10800s, veio %d (zone %s)", offset, ref.Location())
	}
	// Idempotência da sync.Once: segunda chamada devolve a mesma location.
	if Location() != loc {
		t.Error("Location() não é estável entre chamadas")
	}
}

func TestNowNoFusoDoProjeto(t *testing.T) {
	n := Now()
	if n.Location() != Location() {
		t.Errorf("Now() não está no fuso do projeto: %s", n.Location())
	}
	// System.Now e o atalho Now devolvem o mesmo fuso.
	if (System{}).Now().Location() != Location() {
		t.Error("System.Now() fora do fuso do projeto")
	}
	// Sanidade temporal: Now está perto do agora real.
	if d := time.Since(n); d < -2*time.Second || d > 2*time.Second {
		t.Errorf("Now() distante do tempo real: %v", d)
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	orig := time.Date(2026, 3, 10, 14, 30, 0, 0, Location())
	s := Format(orig)
	if s != "2026-03-10T14:30:00-03:00" {
		t.Errorf("Format inesperado: %q", s)
	}
	back, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !back.Equal(orig) {
		t.Errorf("round-trip divergente: %v != %v", back, orig)
	}
	if back.Location() != Location() {
		t.Errorf("Parse não normalizou o fuso: %s", back.Location())
	}
}

func TestParseNormalizaFusoDiferente(t *testing.T) {
	// Entrada em UTC é normalizada para o fuso do projeto, mesmo instante.
	back, err := Parse("2026-03-10T17:30:00Z")
	if err != nil {
		t.Fatalf("Parse UTC: %v", err)
	}
	if back.Location() != Location() {
		t.Errorf("fuso não normalizado: %s", back.Location())
	}
	// 17:30Z == 14:30-03:00.
	if h, m, _ := back.Clock(); h != 14 || m != 30 {
		t.Errorf("instante convertido errado: %02d:%02d", h, m)
	}
}

func TestParseInvalido(t *testing.T) {
	if _, err := Parse("não é data"); err == nil {
		t.Error("Parse de string inválida deveria falhar")
	}
	if _, err := Parse("10/03/2026"); err == nil {
		t.Error("Parse de formato não-RFC3339 deveria falhar")
	}
}

func TestFixedClock(t *testing.T) {
	want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := Fixed{T: want}
	n := f.Now()
	if !n.Equal(want) {
		t.Errorf("Fixed.Now instante divergente: %v", n)
	}
	if n.Location() != Location() {
		t.Errorf("Fixed.Now não normalizou o fuso: %s", n.Location())
	}
	// Determinístico: duas chamadas iguais.
	if !f.Now().Equal(f.Now()) {
		t.Error("Fixed.Now não é determinístico")
	}
}
