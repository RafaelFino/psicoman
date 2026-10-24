package domain

import "context"

// SealedText é o valor de domínio para texto clínico sensível (corpo de sessão,
// anamnese, nota, resumo de IA). Carrega o texto em CLARO em memória durante a
// vida do agregado; a cifra/decifra é EXPLÍCITA, via Cipher, na borda de
// persistência (platform/pb) — não há serialização mágica (D6.2).
//
// Não-vazamento (R6.3, R4.3):
//   - SealedText NÃO implementa MarshalJSON — a saída para a API passa por DTO
//     explícito (o terapeuta lê via Reveal(); nenhum DTO do portal inclui clínico).
//   - String() (usado por fmt/%v/%s e logs) devolve um marcador REDIGIDO, nunca
//     o claro, para que o conteúdo clínico não vaze em log por acidente.
//   - O único caminho para o claro é Reveal(), chamado deliberadamente ao montar
//     um DTO de leitura do terapeuta.
type SealedText struct {
	plaintext string // claro em memória; nunca serializado nem logado direto
}

// NewSealedText cria um SealedText a partir do texto em claro (em memória).
func NewSealedText(plaintext string) SealedText {
	return SealedText{plaintext: plaintext}
}

// Reveal devolve o texto em claro. É o ÚNICO caminho explícito para o claro,
// usado ao montar o DTO de leitura do terapeuta (D6.3). Nunca deve ser chamado
// para montar resposta do portal do paciente.
func (t SealedText) Reveal() string { return t.plaintext }

// Empty indica se o SealedText não tem conteúdo.
func (t SealedText) Empty() bool { return t.plaintext == "" }

// String devolve um marcador REDIGIDO (nunca o claro), protegendo contra
// vazamento acidental via fmt/%v/%s e logs. Para o claro, use Reveal().
func (t SealedText) String() string {
	if t.plaintext == "" {
		return "SealedText(vazio)"
	}
	return "SealedText(redigido)"
}

// GoString garante que %#v (usado por alguns loggers/dumps) também não vaze o
// claro.
func (t SealedText) GoString() string { return t.String() }

// SealFor cifra o SealedText e devolve o envelope versionado para persistência.
// É uma conversão de BORDA: deve ser usada apenas pelo adapter platform/pb ao
// gravar um campo clínico (D6.3). Um SealedText vazio produz envelope vazio
// (coluna nula/vazia no banco), sem chamar a cifra.
func SealFor(ctx context.Context, c Cipher, t SealedText) (string, error) {
	if t.plaintext == "" {
		return "", nil
	}
	return c.Seal(ctx, t.plaintext)
}

// OpenInto decifra o envelope e devolve um SealedText com o claro em memória.
// É uma conversão de BORDA: deve ser usada apenas pelo adapter platform/pb ao
// ler um campo clínico (D6.3). Um envelope vazio produz um SealedText vazio.
func OpenInto(ctx context.Context, c Cipher, envelope string) (SealedText, error) {
	if envelope == "" {
		return SealedText{}, nil
	}
	plaintext, err := c.Open(ctx, envelope)
	if err != nil {
		return SealedText{}, err
	}
	return SealedText{plaintext: plaintext}, nil
}
