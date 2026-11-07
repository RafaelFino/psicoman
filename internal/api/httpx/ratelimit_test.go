package httpx

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// fakeClock é um relógio controlável.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter(c *fakeClock, limit, maxKeys int, onReject func(string), log *slog.Logger) *Limiter {
	return NewLimiter(LimiterOptions{
		Limit: limit, Window: time.Hour, MaxKeys: maxKeys, Scope: ScopeCadastro,
		Clock: c.now, OnReject: onReject, Log: log,
	})
}

func TestLimiter_DentroDoLimiteEEstouro(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	l := newTestLimiter(c, 3, 100, nil, nil)

	for i := 1; i <= 3; i++ {
		if ok, _ := l.Allow("k"); !ok {
			t.Fatalf("tentativa %d dentro do limite deveria passar", i)
		}
	}
	ok, retry := l.Allow("k")
	if ok {
		t.Fatal("4ª tentativa deveria estourar")
	}
	if retry <= 0 || retry > time.Hour {
		t.Errorf("retryAfter deveria estar em (0, 1h], veio %v", retry)
	}
	// Outra chave não é afetada.
	if ok, _ := l.Allow("outra"); !ok {
		t.Error("outra chave não deveria ser afetada")
	}
}

func TestLimiter_ExpiracaoDaJanela(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	l := newTestLimiter(c, 1, 100, nil, nil)

	l.Allow("k")
	if ok, _ := l.Allow("k"); ok {
		t.Fatal("2ª tentativa deveria estourar")
	}
	c.advance(time.Hour)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("após a janela a chave deveria voltar a ser aceita")
	}
}

func TestLimiter_LimpezaRemoveExpiradas(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	l := newTestLimiter(c, 5, 1000, nil, nil)

	for i := 0; i < 10; i++ {
		l.Allow(string(rune('a' + i)))
	}
	c.advance(2 * time.Hour)
	// sweepEvery chamadas forçam a varredura.
	for i := 0; i < sweepEvery; i++ {
		l.Allow("ativa")
	}
	l.mu.Lock()
	n := len(l.entries)
	l.mu.Unlock()
	if n != 1 {
		t.Errorf("só a chave ativa deveria restar após a limpeza, restaram %d", n)
	}
}

func TestLimiter_SaturacaoChaveNovaRecusadaExistenteAtendida(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	var rejects []string
	l := newTestLimiter(c, 5, 3, func(s string) { rejects = append(rejects, s) }, nil)

	for _, k := range []string{"a", "b", "c"} {
		if ok, _ := l.Allow(k); !ok {
			t.Fatalf("chave %q deveria entrar (tabela ainda tem espaço)", k)
		}
	}
	// Tabela cheia: chave NOVA é recusada...
	if ok, retry := l.Allow("nova"); ok || retry <= 0 {
		t.Fatalf("chave nova com tabela cheia deveria ser recusada com retryAfter, ok=%v retry=%v", ok, retry)
	}
	// ...mas chave EXISTENTE dentro do limite segue atendida.
	if ok, _ := l.Allow("a"); !ok {
		t.Error("chave existente dentro do limite deveria ser atendida com a tabela cheia")
	}
	// Expirada a janela, a chave nova volta a ser aceita.
	c.advance(time.Hour)
	if ok, _ := l.Allow("nova"); !ok {
		t.Error("expirada a janela, a chave nova deveria ser aceita")
	}
	if len(rejects) != 1 || rejects[0] != ScopeCadastro {
		t.Errorf("OnReject deveria ter sido chamado 1x com o escopo, veio %v", rejects)
	}
}

func TestLimiter_OnRejectUmaVezPorBloqueioEZeroEmOk(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	n := 0
	l := newTestLimiter(c, 2, 100, func(string) { n++ }, nil)

	l.Allow("k")
	l.Allow("k")
	if n != 0 {
		t.Fatalf("OnReject não pode ser chamado em ok, veio %d", n)
	}
	l.Allow("k") // estouro
	l.Allow("k") // estouro
	if n != 2 {
		t.Errorf("OnReject deveria ter sido chamado 1x por 429, veio %d", n)
	}
}

func TestLimiter_WarnDeSaturacaoUmaVezPorMinuto(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	l := newTestLimiter(c, 5, 1, nil, log)

	l.Allow("a")
	for i := 0; i < 5; i++ {
		l.Allow("chave-secreta-xyz") // recusada por saturação, 5x no mesmo instante
	}
	if got := strings.Count(buf.String(), "rate limit saturado"); got != 1 {
		t.Fatalf("Warn deveria sair 1x no mesmo minuto, saiu %d", got)
	}
	c.advance(2 * time.Minute)
	l.Allow("chave-secreta-xyz")
	if got := strings.Count(buf.String(), "rate limit saturado"); got != 2 {
		t.Errorf("após 1 minuto o Warn deveria sair de novo (total 2), total %d", got)
	}
	// O Warn nunca carrega a chave.
	if strings.Contains(buf.String(), "chave-secreta-xyz") {
		t.Error("o Warn não pode vazar a chave")
	}
}

func TestEmailKey(t *testing.T) {
	a := EmailKey(ScopeCadastro, "Fulano@Exemplo.com")
	b := EmailKey(ScopeCadastro, "  fulano@exemplo.com ")
	if a != b {
		t.Error("caixa e espaços devem normalizar para a mesma chave")
	}
	if EmailKey(ScopeAgendamento, "fulano@exemplo.com") == a {
		t.Error("escopos diferentes devem gerar chaves diferentes")
	}
	if len(a) != 64 || strings.Contains(a, "fulano") {
		t.Errorf("a chave deve ser um sha256 hex sem o valor original: %q", a)
	}
}
