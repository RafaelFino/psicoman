package domain

import "context"

// Cipher é a interface de cifra que o DOMÍNIO conhece (porta). A implementação
// concreta vive em internal/platform/crypto e encapsula o primitivo AES-256-GCM
// e o KeyProvider — o domínio NÃO importa a lib de cripto nem o PocketBase (D6.1).
//
// Seal recebe o texto em claro e devolve o ENVELOPE versionado (string);
// Open recebe o envelope e devolve o claro, resolvendo a chave pelo id embutido
// no envelope (habilita rotação, D6.4).
type Cipher interface {
	Seal(ctx context.Context, plaintext string) (envelope string, err error)
	Open(ctx context.Context, envelope string) (plaintext string, err error)
}

// KeyProvider resolve a chave de cifra por id (habilita rotação) e informa o id
// corrente. É a porta que desacopla a origem da chave (config hoje; vault no
// futuro) da interface Cipher (D6.4). Definida no domínio como porta; a
// implementação concreta vive em internal/platform/crypto.
type KeyProvider interface {
	// Key devolve a chave de 32 bytes (AES-256) associada ao id.
	Key(id string) ([]byte, error)
	// CurrentID devolve o id da chave corrente (usada ao cifrar).
	CurrentID() string
}
