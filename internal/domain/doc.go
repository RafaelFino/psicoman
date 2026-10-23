// Package domain agrupa os bounded contexts do Psicoman (domínio rico).
//
// Cada subpacote (patient, clinical, billing, scheduling, costs, therapist) é a
// autoridade sobre suas entidades e invariantes; os agregados carregam o próprio
// comportamento, validação, serialização e formatação de log (OO bem feita, R4).
//
// Regra de dependência (não-negociável, psicoman-golang.md / D5):
// o domínio é a AUTORIDADE e NÃO importa o PocketBase, o adapter platform/pb,
// a camada app nem a camada api. Toda lib externa (persistência, cifra, Google,
// IA) é conhecida apenas por uma interface (porta); a implementação concreta vive
// em platform/* ou integration/*. A única dependência de plataforma permitida ao
// domínio é platform/crypto (para o SealedText) — e mesmo essa via a interface
// Cipher, sem importar a lib de cripto.
package domain
