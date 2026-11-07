package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// 0050_drive_dr cria a coleção singleton drive_dr, que persiste o id da pasta
// de DR do Google Drive criada/descoberta automaticamente (onda
// auditoria-profunda, R3). Ciclo de vida independente do google_token: a
// reautorização do OAuth (que faz replace do token) não pode apagar o id da
// pasta. O id da pasta NÃO é segredo — fica em claro.
//
// Regras de acesso nulas (só superuser via REST), como as demais coleções
// internas: o drive_dr é consumido pelo adapter em platform/pb, não exposto
// por contrato /v1.
func init() {
	m.Register(upDriveDR, downDriveDR, "0050_drive_dr")
}

func upDriveDR(app core.App) error {
	col := newBaseCollection("drive_dr")
	col.Fields.Add(textField("folder_id", true)) // obrigatório; id da pasta no Drive
	col.Fields.Add(textField("folder_name", false))
	timestamps(col)
	return app.Save(col)
}

func downDriveDR(app core.App) error {
	return dropCollections(app, "drive_dr")
}
