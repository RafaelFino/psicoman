// Package drivesync espelha os backups do PocketBase para o Google Drive como
// fallback/DR.
//
// Dispara a partir de um hook no ciclo de backup do PocketBase (fora do caminho
// de request); faz upload do snapshot cifrado para uma pasta fria no Drive, com
// escopo OAuth mínimo (drive.file). O Drive NUNCA é fonte da verdade nem está no
// caminho de request de nenhuma operação de negócio (R9, D9).
package drivesync
