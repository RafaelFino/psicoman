package gemini

import (
	"context"
	"sync"
)

// FakeSummarizer é um Summarizer em memória para testes — NENHUM teste toca a
// API real do Gemini. Registra as chamadas (para asserção de minimização: o que
// foi enviado) e devolve um resultado determinístico.
type FakeSummarizer struct {
	mu sync.Mutex

	// Result é o retorno de Summarize (resumo + tags).
	Result SummaryResult
	// Err, se não-nil, é devolvido por Summarize.
	Err error

	// Calls registra os inputs recebidos (asserção de minimização: só Text).
	Calls []SummaryInput
}

var _ Summarizer = (*FakeSummarizer)(nil)

// Summarize registra o input e devolve o resultado configurado.
func (f *FakeSummarizer) Summarize(_ context.Context, in SummaryInput) (*SummaryResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, in)
	if f.Err != nil {
		return nil, f.Err
	}
	r := f.Result
	return &r, nil
}
