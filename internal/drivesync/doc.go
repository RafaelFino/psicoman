// Package drivesync espelha os backups do PocketBase para o Google Drive como
// fallback/DR.
//
// Dispara a partir de um hook no ciclo de backup do PocketBase (fora do caminho
// de request). Cada backup sobe um SNAPSHOT INTEIRO, cifrado (AES-256-GCM) antes
// do upload, para a pasta de DR (criada/descoberta automaticamente pelo
// FolderResolver; escopo OAuth mínimo drive.file). Não há backup incremental.
//
// Após um upload ok, a retenção mantém os N snapshots mais recentes e manda os
// excedentes para a LIXEIRA (nunca delete). Falha de espelhamento ou de retenção
// é logada, contada (Observer) e refletida no Status (/readyz), mas nunca
// derruba o backup local nem o processo. O Drive NUNCA é fonte da verdade nem
// está no caminho de request de nenhuma operação de negócio (R9, D9).
//
// O pacote não importa integration/google nem app: tudo entra por interfaces
// locais (SnapshotStore, FolderClient, FolderStore, Observer).
package drivesync
