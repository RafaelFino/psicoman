package httpx

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestRespondCreatedEAccepted: 201 e 202 saem no envelope com data e error null.
func TestRespondCreatedEAccepted(t *testing.T) {
	e, rec := newEvent(t)
	if err := RespondCreated(e, "criado", map[string]any{"id": "1"}); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("RespondCreated: quero 201, veio %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error":null`) || !strings.Contains(rec.Body.String(), `"id":"1"`) {
		t.Errorf("envelope 201 inesperado: %s", rec.Body.String())
	}

	e2, rec2 := newEvent(t)
	if err := RespondAccepted(e2, "aceito", map[string]any{"status": "ok"}); err != nil {
		t.Fatal(err)
	}
	if rec2.Code != http.StatusAccepted {
		t.Errorf("RespondAccepted: quero 202, veio %d", rec2.Code)
	}
}

// TestRespondErrorMessageTruncada: mensagem acima de 300 runes é truncada no
// corpo (guarda defensiva), preservando o code.
func TestRespondErrorMessageTruncada(t *testing.T) {
	e, rec := newEvent(t)
	long := strings.Repeat("á", 500) // runes multibyte: exercita a truncagem por rune
	if err := RespondError(e, http.StatusBadRequest, "requisicao_invalida", long, nil); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: quero 400, veio %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"requisicao_invalida"`) {
		t.Errorf("code ausente: %s", rec.Body.String())
	}
	// A mensagem no corpo não pode conter as 500 runes inteiras.
	if strings.Contains(rec.Body.String(), long) {
		t.Error("mensagem longa não foi truncada no corpo")
	}
}

// TestActorIdentitySemStore: sem o middleware do Pangolin, ActorIdentity é "".
func TestActorIdentitySemStore(t *testing.T) {
	e, _ := newEvent(t)
	if got := ActorIdentity(e); got != "" {
		t.Errorf("ActorIdentity sem store deveria ser \"\", veio %q", got)
	}
}

// TestFmtTimeFormata: instante não-zero vira ISO-8601 no fuso do projeto.
func TestFmtTimeFormata(t *testing.T) {
	got := FmtTime(time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC))
	// 14:30Z == 11:30-03:00.
	if got != "2026-03-10T11:30:00-03:00" {
		t.Errorf("FmtTime inesperado: %q", got)
	}
}

// TestParseDateRange: período date-only vira [de 00:00, ate 23:59:59.999...]
// no fuso do projeto (ate inclusivo); entrada inválida devolve erro.
func TestParseDateRange(t *testing.T) {
	de, ate, err := ParseDateRange("2026-03-01", "2026-03-31")
	if err != nil {
		t.Fatalf("ParseDateRange: %v", err)
	}
	if h, m, s := de.Clock(); h != 0 || m != 0 || s != 0 {
		t.Errorf("de deveria ser meia-noite, veio %02d:%02d:%02d", h, m, s)
	}
	// ate é o fim do dia 31 (inclusivo): deve ser depois de 31 23:59:00.
	limite := time.Date(2026, 3, 31, 23, 59, 0, 0, de.Location())
	if !ate.After(limite) {
		t.Errorf("ate deveria cobrir o dia inteiro, veio %v", ate)
	}
	// Mesmo mês/ano.
	if de.Month() != 3 || ate.Month() != 3 {
		t.Errorf("mês inesperado: de=%v ate=%v", de.Month(), ate.Month())
	}

	if _, _, err := ParseDateRange("2026-13-99", "2026-03-31"); err == nil {
		t.Error("data inicial inválida deveria falhar")
	}
	if _, _, err := ParseDateRange("2026-03-01", "xx"); err == nil {
		t.Error("data final inválida deveria falhar")
	}
}
