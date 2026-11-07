package pb

import (
	"context"
	"errors"
	"testing"

	"github.com/RafaelFino/psicoman/internal/app"
)

func TestDriveFolderRepo_LoadVazioEhNotFound(t *testing.T) {
	a := newTestApp(t)
	repo := NewDriveFolderRepository(a)

	_, err := repo.Load(context.Background())
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("Load sem registro deveria ser ErrNotFound, veio %v", err)
	}
}

func TestDriveFolderRepo_SaveLoadRoundTrip(t *testing.T) {
	a := newTestApp(t)
	repo := NewDriveFolderRepository(a)

	if err := repo.Save(context.Background(), "folder-abc"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "folder-abc" {
		t.Fatalf("esperava folder-abc, veio %q", got)
	}
}

func TestDriveFolderRepo_SaveSobrescreveSingleton(t *testing.T) {
	a := newTestApp(t)
	repo := NewDriveFolderRepository(a)

	if err := repo.Save(context.Background(), "folder-1"); err != nil {
		t.Fatalf("Save 1: %v", err)
	}
	if err := repo.Save(context.Background(), "folder-2"); err != nil {
		t.Fatalf("Save 2: %v", err)
	}
	got, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "folder-2" {
		t.Fatalf("singleton deveria ter o id atualizado, veio %q", got)
	}
	// Garante que há um único registro (não duplicou).
	recs, err := a.FindRecordsByFilter("drive_dr", "", "created_at", 10, 0)
	if err != nil {
		t.Fatalf("FindRecordsByFilter: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("drive_dr deveria ter 1 registro (singleton), veio %d", len(recs))
	}
}
