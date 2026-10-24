package crypto

import (
	"encoding/base64"
	"fmt"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// MapKeyProvider é um KeyProvider em memória, alimentado pela configuração.
// Guarda um conjunto de chaves por id e qual é a corrente — o suficiente para
// rotação: a chave corrente cifra o que é novo; chaves antigas continuam
// resolvíveis para decifrar envelopes gravados antes da troca (D6.4).
//
// Evolução futura (fora do escopo desta onda): trocar por um provider com vault
// externo, mantendo a mesma interface domain.KeyProvider.
type MapKeyProvider struct {
	keys      map[string][]byte
	currentID string
}

// KeySpec descreve uma chave de cifra na configuração: um id estável e a chave
// de 32 bytes codificada em base64 (padrão, com padding).
type KeySpec struct {
	ID     string
	B64Key string
}

// NewMapKeyProvider monta o provider a partir das chaves da config e do id
// corrente. Valida que todas as chaves decodificam para 32 bytes e que o id
// corrente existe. Erra (em vez de silenciar) para falhar o boot cedo se a
// configuração de cifra estiver inconsistente.
func NewMapKeyProvider(currentID string, specs []KeySpec) (*MapKeyProvider, error) {
	if currentID == "" {
		return nil, fmt.Errorf("id da chave corrente vazio")
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("nenhuma chave de cifra configurada")
	}
	keys := make(map[string][]byte, len(specs))
	for _, s := range specs {
		if s.ID == "" {
			return nil, fmt.Errorf("chave de cifra sem id")
		}
		raw, err := base64.StdEncoding.DecodeString(s.B64Key)
		if err != nil {
			return nil, fmt.Errorf("decodificando chave %q (base64): %w", s.ID, err)
		}
		if len(raw) != keySize {
			return nil, fmt.Errorf("chave %q tem %d bytes; esperado %d (AES-256)", s.ID, len(raw), keySize)
		}
		keys[s.ID] = raw
	}
	if _, ok := keys[currentID]; !ok {
		return nil, fmt.Errorf("id da chave corrente %q não está entre as chaves configuradas", currentID)
	}
	return &MapKeyProvider{keys: keys, currentID: currentID}, nil
}

// Key devolve a chave associada ao id, ou erro se desconhecida.
func (p *MapKeyProvider) Key(id string) ([]byte, error) {
	k, ok := p.keys[id]
	if !ok {
		return nil, fmt.Errorf("chave de cifra desconhecida: %q", id)
	}
	return k, nil
}

// CurrentID devolve o id da chave corrente (usada ao cifrar).
func (p *MapKeyProvider) CurrentID() string { return p.currentID }

// Garante, em tempo de compilação, que MapKeyProvider satisfaz a porta.
var _ domain.KeyProvider = (*MapKeyProvider)(nil)
