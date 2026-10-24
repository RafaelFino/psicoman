// Package pb é o adapter de persistência do Psicoman sobre o PocketBase.
//
// É o ÚNICO pacote que importa o PocketBase (Ports & Adapters, R5/D5): implementa
// as interfaces de porta (repositórios) definidas no domínio/app, mapeando
// agregado de domínio ↔ core.Record. Ao gravar um campo clínico, cifra via a
// interface Cipher e persiste o ENVELOPE como string; ao ler, decifra para
// SealedText em memória (D6.3). Também define as coleções/migrations do PocketBase
// derivadas do MER canônico (D4b).
//
// Trocar o PocketBase por outra base = reescrever só este pacote (e as migrations).
package pb
