package service_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/migration"
	"github.com/RafaelFino/psicoman/internal/platform/crypto"
	"github.com/RafaelFino/psicoman/internal/repository/sqlite"
	"github.com/RafaelFino/psicoman/internal/service"
)

// fakeDrive é um Drive em memória para o teste de backup.
type fakeDrive struct {
	content map[string][]byte
	names   map[string]string
	seq     int
}

func newFakeDrive() *fakeDrive {
	return &fakeDrive{content: map[string][]byte{}, names: map[string]string{}}
}

func (f *fakeDrive) Upload(_ context.Context, _ string, file service.DriveFile) (string, error) {
	f.seq++
	id := "d" + itoaBackup(f.seq)
	f.content[id] = append([]byte(nil), file.Content...)
	f.names[file.Name] = id
	return id, nil
}
func (f *fakeDrive) Download(_ context.Context, id string) ([]byte, error) { return f.content[id], nil }
func (f *fakeDrive) List(_ context.Context, _ string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range f.names {
		out[k] = v
	}
	return out, nil
}

func itoaBackup(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// fakeAudit descarta tudo (evita depender de repositório).
type fakeAudit struct{}

func (fakeAudit) Insert(context.Context, *domain.AuditLog) error        { return nil }
func (fakeAudit) List(context.Context, int) ([]*domain.AuditLog, error) { return nil, nil }

func TestBackupRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "psicoman.db")
	db, err := sqlite.Open(sqlite.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := migration.Run(db.DB); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	patientRepo := sqlite.NewPatientRepo(db)
	svc := service.NewPatientService(patientRepo)
	if _, err := svc.Create(context.Background(), service.CreateInput{
		Name: "Backup Teste", Phone: "1199", Email: "backup@example.com",
	}); err != nil {
		t.Fatalf("criar paciente: %v", err)
	}

	key, _ := crypto.GenerateKey()
	cipher := crypto.New(crypto.StaticKeyProvider{B64: key})
	drive := newFakeDrive()
	audit := service.NewAuditService(fakeAudit{})
	backup := service.NewBackupService(db.DB, dbPath, cipher, drive, "folder", nil, audit)

	if _, err := backup.Backup(context.Background()); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// Simula perda: remove o paciente.
	list, _ := patientRepo.List(context.Background())
	if len(list) != 1 {
		t.Fatalf("esperava 1 paciente, tem %d", len(list))
	}
	if err := patientRepo.SoftDelete(context.Background(), list[0].ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	db.Close() // libera o arquivo para substituição

	if err := backup.Restore(context.Background()); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	db2, err := sqlite.Open(sqlite.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()
	list2, err := sqlite.NewPatientRepo(db2).List(context.Background())
	if err != nil {
		t.Fatalf("list pós-restore: %v", err)
	}
	if len(list2) != 1 || list2[0].Email != "backup@example.com" {
		t.Errorf("restore não recuperou o paciente: %+v", list2)
	}
}

func TestBackupGEDIncremental(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "psicoman.db")
	db, _ := sqlite.Open(sqlite.Options{Path: dbPath})
	_ = migration.Run(db.DB)
	defer db.Close()

	key, _ := crypto.GenerateKey()
	cipher := crypto.New(crypto.StaticKeyProvider{B64: key})
	drive := newFakeDrive()
	audit := service.NewAuditService(fakeAudit{})

	// Lister mutável para adicionar arquivo entre backups.
	lister := &fakeLister{files: []service.GEDFileRef{
		{RelPath: "p1/a", SHA256: "hashA"},
		{RelPath: "p1/b", SHA256: "hashB"},
	}}
	src := service.NewGEDManifestSource(lister, &fakeReader{data: map[string][]byte{
		"p1/a": []byte("A"), "p1/b": []byte("B"), "p1/c": []byte("C"),
	}})
	backup := service.NewBackupService(db.DB, dbPath, cipher, drive, "folder", src, audit)

	res, err := backup.Backup(context.Background())
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if res.GEDUploaded != 2 || res.GEDSkipped != 0 {
		t.Errorf("1º: uploaded=%d skipped=%d, quer 2/0", res.GEDUploaded, res.GEDSkipped)
	}

	res2, _ := backup.Backup(context.Background())
	if res2.GEDUploaded != 0 || res2.GEDSkipped != 2 {
		t.Errorf("2º: uploaded=%d skipped=%d, quer 0/2", res2.GEDUploaded, res2.GEDSkipped)
	}

	lister.files = append(lister.files, service.GEDFileRef{RelPath: "p1/c", SHA256: "hashC"})
	res3, _ := backup.Backup(context.Background())
	if res3.GEDUploaded != 1 || res3.GEDSkipped != 2 {
		t.Errorf("3º: uploaded=%d skipped=%d, quer 1/2", res3.GEDUploaded, res3.GEDSkipped)
	}
}

type fakeLister struct{ files []service.GEDFileRef }

func (f *fakeLister) AllFiles(context.Context) ([]service.GEDFileRef, error) { return f.files, nil }

type fakeReader struct{ data map[string][]byte }

func (f *fakeReader) ReadRaw(relPath string) ([]byte, error) { return f.data[relPath], nil }
