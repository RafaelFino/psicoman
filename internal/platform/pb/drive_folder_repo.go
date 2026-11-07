package pb

import (
	"context"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// driveFolderRepo implementa app.DriveFolderRepository sobre a coleção drive_dr
// (migração 0050). Singleton lógico: Load pega o único registro; Save reusa o
// existente ou cria o primeiro. O id da pasta não é segredo — fica em claro.
type driveFolderRepo struct {
	app core.App
}

var _ app.DriveFolderRepository = (*driveFolderRepo)(nil)

// NewDriveFolderRepository cria o repositório da pasta de DR.
func NewDriveFolderRepository(a core.App) app.DriveFolderRepository {
	return &driveFolderRepo{app: a}
}

// Load devolve o id da pasta persistida. ErrNotFound quando ainda não resolvida.
func (r *driveFolderRepo) Load(_ context.Context) (string, error) {
	recs, err := r.app.FindRecordsByFilter("drive_dr", "", "created_at", 1, 0)
	if err != nil {
		return "", mapError(err)
	}
	if len(recs) == 0 {
		return "", app.ErrNotFound
	}
	return recs[0].GetString("folder_id"), nil
}

// Save grava o id (cria-ou-atualiza o singleton).
func (r *driveFolderRepo) Save(ctx context.Context, folderID string) error {
	recs, err := r.app.FindRecordsByFilter("drive_dr", "", "created_at", 1, 0)
	if err != nil {
		return mapError(err)
	}
	now := formatTime(clock.System{}.Now())
	var rec *core.Record
	if len(recs) > 0 {
		rec = recs[0]
	} else {
		col, cerr := r.app.FindCollectionByNameOrId("drive_dr")
		if cerr != nil {
			return mapError(cerr)
		}
		rec = core.NewRecord(col)
		rec.Set("id", ulid.New())
		rec.Set("created_at", now)
	}
	rec.Set("folder_id", folderID)
	rec.Set("updated_at", now)
	return mapError(r.app.SaveWithContext(ctx, rec))
}
