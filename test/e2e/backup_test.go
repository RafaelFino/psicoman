package e2e

import (
	"net/http"
	"testing"
)

func TestBackupEndpoint(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Cria um paciente + anexo no GED para o backup ter conteúdo.
	pid := env.createPatient(t, "backup@example.com")
	env.uploadFile(t, "/v1/admin/patients/"+pid+"/files", "doc.txt", []byte("conteúdo")).Body.Close()

	// Backup → 200, com contadores do GED.
	resp := env.AdminPOST(t, "/v1/admin/backup", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("backup status = %d, quer 200", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var res struct {
		SnapshotFileID string `json:"snapshot_file_id"`
		GEDUploaded    int    `json:"ged_uploaded"`
		GEDSkipped     int    `json:"ged_skipped"`
	}
	e.DataAs(t, &res)
	if res.SnapshotFileID == "" {
		t.Error("backup sem snapshot no Drive")
	}
	if res.GEDUploaded != 1 {
		t.Errorf("ged_uploaded = %d, quer 1", res.GEDUploaded)
	}

	// Segundo backup: GED incremental não reenvia (skip por hash).
	resp = env.AdminPOST(t, "/v1/admin/backup", map[string]any{})
	e2 := DecodeEnvelope(t, resp)
	var res2 struct {
		GEDUploaded int `json:"ged_uploaded"`
		GEDSkipped  int `json:"ged_skipped"`
	}
	e2.DataAs(t, &res2)
	if res2.GEDUploaded != 0 || res2.GEDSkipped != 1 {
		t.Errorf("2º backup: uploaded=%d skipped=%d, quer 0/1", res2.GEDUploaded, res2.GEDSkipped)
	}
}

func TestRestoreWithoutBackup(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Sem backup prévio no Drive → 422.
	resp := env.AdminPOST(t, "/v1/admin/restore", map[string]any{})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("restore sem backup status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}
