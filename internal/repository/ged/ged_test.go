package ged

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadIntegrity(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	content := []byte("conteúdo do prontuário")
	rel, sha, size, err := store.Write("paciente-1", "file-1", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("size = %d, quer %d", size, len(content))
	}
	want := sha256.Sum256(content)
	if sha != hex.EncodeToString(want[:]) {
		t.Errorf("sha divergente")
	}

	data, err := store.Read(rel, sha)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Error("conteúdo lido difere do escrito")
	}
}

func TestReadDetectsCorruption(t *testing.T) {
	root := t.TempDir()
	store, _ := NewStore(root)
	rel, sha, _, _ := store.Write("p1", "f1", bytes.NewReader([]byte("original")))

	// Corrompe o arquivo em disco.
	abs := filepath.Join(root, rel)
	if err := os.WriteFile(abs, []byte("adulterado"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(rel, sha); err != ErrIntegrity {
		t.Errorf("esperava ErrIntegrity, veio: %v", err)
	}
}

func TestScopeSegregation(t *testing.T) {
	root := t.TempDir()
	store, _ := NewStore(root)
	rel, _, _, _ := store.Write("paciente-abc", "f1", bytes.NewReader([]byte("x")))
	if filepath.Dir(rel) != "paciente-abc" {
		t.Errorf("arquivo não segregado por paciente: %q", rel)
	}
	// Escopo vazio → perfil do terapeuta.
	relT, _, _, _ := store.Write("", "f2", bytes.NewReader([]byte("y")))
	if filepath.Dir(relT) != "_therapist" {
		t.Errorf("escopo do terapeuta errado: %q", relT)
	}
}
