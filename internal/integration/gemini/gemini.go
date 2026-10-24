package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// apiBaseURL é a base da API do Gemini (generativelanguage). Variável de pacote
// para apontar ao servidor fake nos testes (sem tocar a rede real).
var apiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// defaultModel é o modelo usado para resumo/tagueamento clínico.
const defaultModel = "gemini-1.5-flash"

// SummaryInput é o MÍNIMO enviado à IA para resumir/taguear uma sessão
// (minimização — psicoman-seguranca-lgpd.md): apenas o texto clínico já
// decifrado em memória pelo chamador. NÃO carrega identificadores de paciente,
// nome, nem metadados — só o conteúdo a resumir.
type SummaryInput struct {
	Text string
}

// SummaryResult é o retorno da IA: um resumo em texto e uma lista de tags (em
// claro, para o índice invertido). O chamador cifra o resumo (SealedText) antes
// de persistir; as tags ficam em claro.
type SummaryResult struct {
	Summary string
	Tags    []string
}

// Summarizer é a interface LOCAL da IA (mesma disciplina de Ports & Adapters do
// Calendar/Drive): resumo clínico + tags atrás de interface, com fake nos
// testes — nenhum teste toca a API real do Gemini. Respeita o ctx recebido.
type Summarizer interface {
	Summarize(ctx context.Context, in SummaryInput) (*SummaryResult, error)
}

// httpDoer é o transporte HTTP injetável (satisfeito por *http.Client).
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// restSummarizer fala com a API do Gemini sobre um httpDoer. A api_key é lida
// da config (NUNCA hard-coded, NUNCA logada — psicoman-seguranca-lgpd.md) e vai
// como query param na requisição. Pede explicitamente uma resposta JSON com
// {summary, tags} para parse determinístico.
type restSummarizer struct {
	http    httpDoer
	baseURL string
	model   string
	apiKey  string
}

var _ Summarizer = (*restSummarizer)(nil)

// NewRESTSummarizer cria o cliente REST do Gemini. apiKey vazia não é aceita
// pelo wiring (a IA degrada antes — opt-in pela presença da chave).
func NewRESTSummarizer(doer httpDoer, apiKey string) Summarizer {
	return &restSummarizer{http: doer, baseURL: apiBaseURL, model: defaultModel, apiKey: apiKey}
}

// promptInstrucao orienta a IA a devolver JSON {summary, tags}. O texto clínico
// vai numa parte separada; a instrução não embute dados do paciente.
const promptInstrucao = `Você é um assistente de registro clínico. Resuma objetivamente o conteúdo da sessão a seguir e proponha de 3 a 8 tags temáticas curtas (minúsculas, sem acento). Responda SOMENTE um JSON no formato {"summary": "...", "tags": ["...", "..."]}.`

// Summarize envia o texto à API do Gemini e parseia o JSON {summary, tags}.
func (c *restSummarizer) Summarize(ctx context.Context, in SummaryInput) (*SummaryResult, error) {
	reqBody := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{
				{"text": promptInstrucao},
				{"text": in.Text},
			},
		}},
		"generationConfig": map[string]any{"responseMimeType": "application/json"},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gemini: serializar requisição: %w", err)
	}
	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("gemini: montar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: chamada à API: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("gemini: API respondeu %d", resp.StatusCode)
	}

	// Extrai o texto da 1ª parte da 1ª candidata e parseia o JSON embutido.
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("gemini: decodificar resposta: %w", err)
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: resposta sem conteúdo")
	}
	return parseSummaryJSON(out.Candidates[0].Content.Parts[0].Text)
}

// parseSummaryJSON interpreta o JSON {summary, tags} devolvido pela IA,
// normalizando as tags (trim, minúsculas, sem vazias/duplicadas).
func parseSummaryJSON(s string) (*SummaryResult, error) {
	var parsed struct {
		Summary string   `json:"summary"`
		Tags    []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &parsed); err != nil {
		return nil, fmt.Errorf("gemini: JSON de resumo inválido: %w", err)
	}
	return &SummaryResult{Summary: parsed.Summary, Tags: NormalizeTags(parsed.Tags)}, nil
}

// NormalizeTags limpa a lista de tags: trim, minúsculas, remove vazias e
// duplicadas preservando a ordem. Exportada para reuso no serviço/testes.
func NormalizeTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
