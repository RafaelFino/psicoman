package app

import (
	"testing"
	"time"
)

// mutableClock é um clock.Clock avançável nos testes.
type mutableClock struct{ t time.Time }

func (c *mutableClock) Now() time.Time { return c.t }

func newStateStore(limite int) (*MemoryOAuthStateStore, *mutableClock) {
	clk := &mutableClock{t: time.Unix(1_700_000_000, 0)}
	return NewMemoryOAuthStateStore(clk, OAuthStateTTL, limite), clk
}

func TestOAuthState_EmiteEConsomeUmaVez(t *testing.T) {
	s, _ := newStateStore(OAuthStateMaxPending)
	st := s.Issue()
	if st == "" || len(st) < 20 {
		t.Fatalf("state deveria ser um valor imprevisível de 128 bits, veio %q", st)
	}
	if !s.Consume(st) {
		t.Fatal("o state emitido deveria ser consumível uma vez")
	}
	if s.Consume(st) {
		t.Fatal("reuso do state deve falhar (uso único)")
	}
}

func TestOAuthState_DesconhecidoEVazio(t *testing.T) {
	s, _ := newStateStore(OAuthStateMaxPending)
	if s.Consume("nunca-emitido") {
		t.Error("state desconhecido não pode ser aceito")
	}
	if s.Consume("") {
		t.Error("state vazio não pode ser aceito")
	}
}

func TestOAuthState_Expira(t *testing.T) {
	s, clk := newStateStore(OAuthStateMaxPending)
	st := s.Issue()
	clk.t = clk.t.Add(OAuthStateTTL + time.Second)
	if s.Consume(st) {
		t.Fatal("state expirado não pode ser aceito")
	}
}

func TestOAuthState_ValidoDentroDoTTL(t *testing.T) {
	s, clk := newStateStore(OAuthStateMaxPending)
	st := s.Issue()
	clk.t = clk.t.Add(OAuthStateTTL - time.Second)
	if !s.Consume(st) {
		t.Fatal("state dentro do TTL deveria ser aceito")
	}
}

func TestOAuthState_CheioRemoveOMaisAntigo(t *testing.T) {
	s, clk := newStateStore(3)
	var emitidos []string
	for i := 0; i < 3; i++ {
		emitidos = append(emitidos, s.Issue())
		clk.t = clk.t.Add(time.Second) // expirações crescentes
	}
	novo := s.Issue() // cheio: o mais antigo (emitidos[0]) sai
	if s.Consume(emitidos[0]) {
		t.Error("o mais antigo deveria ter sido removido ao encher")
	}
	if !s.Consume(emitidos[1]) || !s.Consume(emitidos[2]) || !s.Consume(novo) {
		t.Error("os demais states devem continuar válidos")
	}
}

func TestOAuthState_CheioDescartaExpiradosPrimeiro(t *testing.T) {
	s, clk := newStateStore(2)
	velho1, velho2 := s.Issue(), s.Issue()
	clk.t = clk.t.Add(OAuthStateTTL + time.Minute) // ambos expiram
	novo := s.Issue()
	if s.Consume(velho1) || s.Consume(velho2) {
		t.Error("os expirados devem ter sido descartados")
	}
	if !s.Consume(novo) {
		t.Error("o state novo deve valer")
	}
}

func TestOAuthState_UnicosEntreEmissoes(t *testing.T) {
	s, _ := newStateStore(OAuthStateMaxPending)
	a, b := s.Issue(), s.Issue()
	if a == b {
		t.Fatal("dois states emitidos não podem ser iguais")
	}
}
