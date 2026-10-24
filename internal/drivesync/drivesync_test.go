package drivesync

import (
	"context"
	"errors"
	"testing"

	"github.com/RafaelFino/psicoman/internal/platform/crypto"
)

// fakeUploader registra os uploads e devolve resultado/erro determinístico.
type fakeUploader struct {
	calls  int
	lastN  string
	lastC  []byte
	uplErr error
	fileID string
}

func (f *fakeUploader) Upload(_ context.Context, name string, content []byte) (string, error) {
	f.calls++
	f.lastN = name
	f.lastC = content
	if f.uplErr != nil {
		return "", f.uplErr
	}
	return f.fileID, nil
}

func testSealer(t *testing.T) *crypto.BlobSealer {
	t.Helper()
	// Chave de teste fixa (32 bytes base64) — mesma usada nos E2E.
	kp, err := crypto.NewMapKeyProvider("k1", []crypto.KeySpec{
		{ID: "k1", B64Key: "wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY="},
	})
	if err != nil {
		t.Fatalf("key provider: %v", err)
	}
	return crypto.NewBlobSealer(kp)
}

// TestUploadSealed_CifraEEnvia: o conteúdo enviado ao uploader é o snapshot
// CIFRADO (nunca o claro), com nome .enc, e o round-trip decifra de volta.
func TestUploadSealed_CifraEEnvia(t *testing.T) {
	up := &fakeUploader{fileID: "drive-1"}
	sealer := testSealer(t)
	s := New(up, sealer)

	raw := []byte("conteúdo-do-zip-de-backup")
	name, err := s.uploadSealed(context.Background(), "pb_backup_2026.zip", raw)
	if err != nil {
		t.Fatal(err)
	}
	if up.calls != 1 {
		t.Fatalf("esperava 1 upload, veio %d", up.calls)
	}
	if name != "pb_backup_2026.enc" || up.lastN != "pb_backup_2026.enc" {
		t.Fatalf("nome remoto inesperado: %q / %q", name, up.lastN)
	}
	// O que foi para o Drive NÃO é o claro.
	if string(up.lastC) == string(raw) {
		t.Fatal("snapshot subiu em claro (sem cifra)")
	}
	// E decifra de volta ao original.
	got, err := sealer.OpenBytes(up.lastC)
	if err != nil {
		t.Fatalf("decifrar snapshot enviado: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatal("round-trip do snapshot não bate")
	}
}

// TestUploadSealed_PropagaErroUpload: falha de upload vira erro (o Register o
// trata como best-effort e não derruba o processo; aqui validamos a propagação).
func TestUploadSealed_PropagaErroUpload(t *testing.T) {
	boom := errors.New("drive fora do ar")
	s := New(&fakeUploader{uplErr: boom}, testSealer(t))
	if _, err := s.uploadSealed(context.Background(), "x.zip", []byte("y")); err == nil {
		t.Fatal("erro de upload deveria propagar de uploadSealed")
	}
}

func TestEncName(t *testing.T) {
	cases := map[string]string{
		"pb_backup_2026.zip": "pb_backup_2026.enc",
		"semext":             "semext.enc",
	}
	for in, want := range cases {
		if got := encName(in); got != want {
			t.Errorf("encName(%q) = %q, quero %q", in, got, want)
		}
	}
}

// TestNew_SemUploaderNoOp: sem uploader (DR desativado), New aceita nil — o hook
// Register vira no-op (coberto pelo E2E de backup sem Google).
func TestNew_SemUploaderNoOp(t *testing.T) {
	s := New(nil, testSealer(t))
	if s.uploader != nil {
		t.Fatal("uploader deveria ser nil (DR desativado)")
	}
}
