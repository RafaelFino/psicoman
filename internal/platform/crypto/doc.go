// Package crypto é o adapter de cifra do Psicoman.
//
// Implementa a interface Cipher que o domínio conhece (Seal/Open), encapsulando
// o primitivo AES-256-GCM e o KeyProvider. Monta e parseia o envelope versionado
// versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext (base64), com o
// id_da_chave habilitando rotação (D6, D11).
//
// O domínio NÃO importa este pacote diretamente além da interface Cipher; a
// cifra/decifra de campo clínico acontece na borda de persistência (platform/pb).
package crypto
