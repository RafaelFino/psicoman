package gemini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newFakeServer(t *testing.T, h http.HandlerFunc) *restSummarizer {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &restSummarizer{http: srv.Client(), baseURL: srv.URL, model: "test-model", apiKey: "k-test"}
}

// geminiResponse monta a resposta da API embutindo o JSON {summary,tags} como
// texto da 1ª parte (formato real do generateContent).
func geminiResponse(inner string) string {
	resp := map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"parts": []map[string]any{{"text": inner}},
			},
		}},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func TestRESTSummarize_ParseEMinimizacao(t *testing.T) {
	var sentBody string
	var sentKey string
	c := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		sentKey = r.URL.Query().Get("key")
		raw, _ := io.ReadAll(r.Body)
		sentBody = string(raw)
		_, _ = io.WriteString(w, geminiResponse(`{"summary":"paciente relatou melhora","tags":["Ansiedade "," SONO ","ansiedade"]}`))
	})

	out, err := c.Summarize(context.Background(), SummaryInput{Text: "conteúdo clínico da sessão"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Summary != "paciente relatou melhora" {
		t.Fatalf("resumo inesperado: %q", out.Summary)
	}
	// Tags normalizadas: trim, minúsculas, sem duplicata.
	if len(out.Tags) != 2 || out.Tags[0] != "ansiedade" || out.Tags[1] != "sono" {
		t.Fatalf("tags não normalizadas: %v", out.Tags)
	}
	// Minimização: o corpo enviado contém o texto clínico mas NÃO identificador
	// de paciente (o input só tem Text — garantido pela struct SummaryInput).
	if !strings.Contains(sentBody, "conteúdo clínico da sessão") {
		t.Error("texto clínico deveria ir no corpo")
	}
	if sentKey != "k-test" {
		t.Errorf("api_key deveria ir como query key, veio %q", sentKey)
	}
}

func TestRESTSummarize_ErroHTTP(t *testing.T) {
	c := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	if _, err := c.Summarize(context.Background(), SummaryInput{Text: "x"}); err == nil {
		t.Fatal("status 429 deveria virar erro")
	}
}

func TestRESTSummarize_RespostaVazia(t *testing.T) {
	c := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"candidates":[]}`)
	})
	if _, err := c.Summarize(context.Background(), SummaryInput{Text: "x"}); err == nil {
		t.Fatal("resposta sem candidatas deveria falhar")
	}
}

func TestRESTSummarize_RespeitaContext(t *testing.T) {
	c := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Summarize(ctx, SummaryInput{Text: "x"}); err == nil {
		t.Fatal("ctx expirado deveria abortar a chamada")
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" Ansiedade", "ANSIEDADE", "", "  ", "Sono"})
	if len(got) != 2 || got[0] != "ansiedade" || got[1] != "sono" {
		t.Fatalf("normalização inesperada: %v", got)
	}
}
