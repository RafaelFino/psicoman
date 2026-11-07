package drivesync

import "strings"

// snapshotSuffix é a extensão dos snapshots cifrados. A retenção só toca
// arquivos com este sufixo (trava de segurança em duas camadas: o List do
// adapter já filtra por pasta + appProperties; aqui filtra por nome).
const snapshotSuffix = ".enc"

// selectToTrash decide quais snapshots vão para a lixeira. files vem do mais
// recente para o mais antigo. Mantém `keep` snapshots GARANTINDO que o recém
// enviado (justUploadedID) esteja entre eles, mesmo que a listagem ainda não o
// mostre (consistência eventual). Arquivos sem o sufixo .enc nunca são tocados.
// keep <= 0 não seleciona nada (retenção desligada é decidida pelo chamador).
func selectToTrash(files []File, keep int, justUploadedID string) []string {
	if keep <= 0 {
		return nil
	}
	// O recém-enviado ocupa um dos slots de `keep`.
	others := keep - 1
	var trash []string
	seen := 0
	for _, f := range files {
		if !strings.HasSuffix(f.Name, snapshotSuffix) || f.ID == justUploadedID {
			continue
		}
		if seen < others {
			seen++
			continue
		}
		trash = append(trash, f.ID)
	}
	return trash
}

// countSnapshots conta os snapshots .enc de files (para a métrica de mantidos).
func countSnapshots(files []File) int {
	n := 0
	for _, f := range files {
		if strings.HasSuffix(f.Name, snapshotSuffix) {
			n++
		}
	}
	return n
}
