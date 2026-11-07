package drivesync

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"

	"github.com/RafaelFino/psicoman/internal/platform/crypto"
)

// fakeStore é um SnapshotStore em memória: registra uploads e permite
// controlar List/Trash/Upload.
type fakeStore struct {
	calls    int
	lastN    string
	lastC    []byte
	uplErr   error
	fileID   string
	files    []File
	listErr  error
	trashErr error
	listed   int
	trashed  []string
}

func (f *fakeStore) Upload(_ context.Context, name string, content []byte) (string, error) {
	f.calls++
	f.lastN = name
	f.lastC = content
	if f.uplErr != nil {
		return "", f.uplErr
	}
	return f.fileID, nil
}

func (f *fakeStore) List(context.Context) ([]File, error) {
	f.listed++
	return f.files, f.listErr
}

func (f *fakeStore) Trash(_ context.Context, id string) error {
	if f.trashErr != nil {
		return f.trashErr
	}
	f.trashed = append(f.trashed, id)
	return nil
}

// recordingObserver registra os eventos recebidos.
type recordingObserver struct {
	mu          sync.Mutex
	backups     []string // "destino:ok|erro"
	mirrors     []string
	retention   []int
	retFailures []string
	sizes       map[string]int64
}

func (o *recordingObserver) Backup(destino string, ok bool, _ time.Duration, size int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	r := "erro"
	if ok {
		r = "ok"
	}
	o.backups = append(o.backups, destino+":"+r)
	if o.sizes == nil {
		o.sizes = map[string]int64{}
	}
	o.sizes[destino] = size
}
func (o *recordingObserver) Mirror(r string)           { o.mirrors = append(o.mirrors, r) }
func (o *recordingObserver) Retention(k int)           { o.retention = append(o.retention, k) }
func (o *recordingObserver) RetentionFailure(e string) { o.retFailures = append(o.retFailures, e) }

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

// TestUploadSealed_CifraEEnvia: o conteúdo enviado é o snapshot CIFRADO (nunca
// o claro), com nome .enc, e o round-trip decifra de volta.
func TestUploadSealed_CifraEEnvia(t *testing.T) {
	up := &fakeStore{fileID: "drive-1"}
	sealer := testSealer(t)
	s := New(up, sealer, 0, nil, nil)

	raw := []byte("conteúdo-do-zip-de-backup")
	name, id, size, err := s.uploadSealed(context.Background(), "pb_backup_2026.zip", raw)
	if err != nil {
		t.Fatal(err)
	}
	if up.calls != 1 {
		t.Fatalf("esperava 1 upload, veio %d", up.calls)
	}
	if name != "pb_backup_2026.enc" || up.lastN != "pb_backup_2026.enc" {
		t.Fatalf("nome remoto inesperado: %q / %q", name, up.lastN)
	}
	if id != "drive-1" {
		t.Errorf("id remoto: quero drive-1, veio %q", id)
	}
	if size != int64(len(up.lastC)) {
		t.Errorf("tamanho deveria ser o do blob cifrado (%d), veio %d", len(up.lastC), size)
	}
	if string(up.lastC) == string(raw) {
		t.Fatal("snapshot subiu em claro (sem cifra)")
	}
	got, err := sealer.OpenBytes(up.lastC)
	if err != nil {
		t.Fatalf("decifrar snapshot enviado: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatal("round-trip do snapshot não bate")
	}
}

func TestUploadSealed_PropagaErroUpload(t *testing.T) {
	s := New(&fakeStore{uplErr: errors.New("drive fora do ar")}, testSealer(t), 0, nil, nil)
	if _, _, _, err := s.uploadSealed(context.Background(), "x.zip", []byte("y")); err == nil {
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

// TestNew_SemStoreDesativado: sem store (DR desativado), New aceita nil e o
// Status reporta "desativado".
func TestNew_SemStoreDesativado(t *testing.T) {
	st := &Status{}
	s := New(nil, testSealer(t), 14, nil, st)
	if s.store != nil {
		t.Fatal("store deveria ser nil (DR desativado)")
	}
	if got := st.State(); got != StateDesativado {
		t.Errorf("State: quero %q, veio %q", StateDesativado, got)
	}
}

// failSealer falha ao cifrar, para exercitar o ramo de erro de cifra.
type failSealer struct{}

func (failSealer) SealBytes([]byte) ([]byte, error) { return nil, errors.New("cifra indisponível") }

func TestUploadSealed_PropagaErroCifra(t *testing.T) {
	up := &fakeStore{}
	s := New(up, failSealer{}, 0, nil, nil)
	if _, _, _, err := s.uploadSealed(context.Background(), "x.zip", []byte("y")); err == nil {
		t.Fatal("erro de cifra deveria propagar")
	}
	if up.calls != 0 {
		t.Errorf("store não deveria ser chamado quando a cifra falha, foi %d", up.calls)
	}
}

// TestStatus_State cobre a tabela de State(): zero value, configurado sem
// tentativa, erro, ok, e ok depois de erro.
func TestStatus_State(t *testing.T) {
	var zero Status
	if got := zero.State(); got != StateDesativado {
		t.Errorf("zero value: quero desativado, veio %q", got)
	}
	var nilSt *Status
	if got := nilSt.State(); got != StateDesativado {
		t.Errorf("nil: quero desativado, veio %q", got)
	}
	nilSt.Set("ok") // nil-safe, não panica
	nilSt.SetConfigured(true)

	st := &Status{}
	st.SetConfigured(true)
	if got := st.State(); got != StateOK {
		t.Errorf("configurado sem tentativa: quero ok, veio %q", got)
	}
	st.Set(resultErro)
	if got := st.State(); got != StateDegradado {
		t.Errorf("após erro: quero degradado, veio %q", got)
	}
	st.Set(resultOK)
	if got := st.State(); got != StateOK {
		t.Errorf("ok depois de erro: quero ok, veio %q", got)
	}
	st.SetConfigured(false)
	if got := st.State(); got != StateDesativado {
		t.Errorf("desconfigurado: quero desativado, veio %q", got)
	}
}

// newBackupApp sobe um TestApp real do PocketBase (backup local verdadeiro) e
// registra o Syncer no hook de backup.
func newBackupApp(t *testing.T, s *Syncer) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	s.Register(app)
	return app
}

// TestHook_UploadERetencaoApagamExcedentes: o backup local ocorre, o snapshot
// sobe, a retenção manda os excedentes para a lixeira e o observer/Status veem
// os fatos certos.
func TestHook_UploadERetencaoApagamExcedentes(t *testing.T) {
	store := &fakeStore{
		fileID: "novo",
		files: []File{
			{ID: "novo", Name: "n.enc"}, {ID: "a", Name: "a.enc"},
			{ID: "b", Name: "b.enc"}, {ID: "c", Name: "c.enc"},
		},
	}
	obs := &recordingObserver{}
	st := &Status{}
	s := New(store, testSealer(t), 2, obs, st)
	app := newBackupApp(t, s)

	if err := app.CreateBackup(context.Background(), ""); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if store.calls != 1 {
		t.Fatalf("deveria subir 1 snapshot, subiu %d", store.calls)
	}
	// Mantém 2 (novo + a); b e c vão para a lixeira.
	if len(store.trashed) != 2 || store.trashed[0] != "b" || store.trashed[1] != "c" {
		t.Errorf("lixeira esperada [b c], veio %v", store.trashed)
	}
	if len(obs.retention) != 1 || obs.retention[0] != 2 {
		t.Errorf("Retention(kept) esperado [2], veio %v", obs.retention)
	}
	if len(obs.backups) != 2 || obs.backups[0] != "local:ok" || obs.backups[1] != "drive:ok" {
		t.Errorf("Backup esperado [local:ok drive:ok], veio %v", obs.backups)
	}
	if obs.sizes["local"] <= 0 || obs.sizes["drive"] <= 0 {
		t.Errorf("tamanhos deveriam ser > 0: %v", obs.sizes)
	}
	if len(obs.mirrors) != 1 || obs.mirrors[0] != "ok" {
		t.Errorf("Mirror esperado [ok], veio %v", obs.mirrors)
	}
	if got := st.State(); got != StateOK {
		t.Errorf("Status após sucesso: quero ok, veio %q", got)
	}
}

// TestHook_TrashComErroNaoFalhaOBackup: erro de Trash é contado e logado, mas o
// espelhamento segue ok.
func TestHook_TrashComErroNaoFalhaOBackup(t *testing.T) {
	store := &fakeStore{
		fileID:   "novo",
		files:    []File{{ID: "novo", Name: "n.enc"}, {ID: "a", Name: "a.enc"}},
		trashErr: errors.New("403"),
	}
	obs := &recordingObserver{}
	st := &Status{}
	app := newBackupApp(t, New(store, testSealer(t), 1, obs, st))

	if err := app.CreateBackup(context.Background(), ""); err != nil {
		t.Fatalf("CreateBackup deveria passar mesmo com Trash falhando: %v", err)
	}
	if len(obs.retFailures) != 1 || obs.retFailures[0] != "lixeira" {
		t.Errorf("RetentionFailure esperado [lixeira], veio %v", obs.retFailures)
	}
	if len(obs.mirrors) != 1 || obs.mirrors[0] != "ok" || st.State() != StateOK {
		t.Errorf("espelhamento deveria ser ok: mirrors=%v state=%q", obs.mirrors, st.State())
	}
}

// TestHook_ListComErroNaoAtualizaGauge: List falha => RetentionFailure("listar"),
// sem Retention(kept) e sem derrubar o espelhamento.
func TestHook_ListComErroNaoAtualizaGauge(t *testing.T) {
	store := &fakeStore{fileID: "novo", listErr: errors.New("403")}
	obs := &recordingObserver{}
	app := newBackupApp(t, New(store, testSealer(t), 5, obs, &Status{}))

	if err := app.CreateBackup(context.Background(), ""); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if len(obs.retFailures) != 1 || obs.retFailures[0] != "listar" {
		t.Errorf("RetentionFailure esperado [listar], veio %v", obs.retFailures)
	}
	if len(obs.retention) != 0 {
		t.Errorf("Retention não deveria ser chamado quando List falha, veio %v", obs.retention)
	}
}

// TestHook_RetencaoZeroNaoLista: retention=0 => retenção manual, nunca lista.
func TestHook_RetencaoZeroNaoLista(t *testing.T) {
	store := &fakeStore{fileID: "novo"}
	app := newBackupApp(t, New(store, testSealer(t), 0, nil, &Status{}))

	if err := app.CreateBackup(context.Background(), ""); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if store.calls != 1 || store.listed != 0 {
		t.Errorf("retention=0: upload=1 e list=0 esperados, veio upload=%d list=%d", store.calls, store.listed)
	}
}

// TestHook_FalhaDeUploadMarcaDegradado: falha de upload => Mirror("erro"),
// Status degradado e o backup local segue válido (sem erro propagado).
func TestHook_FalhaDeUploadMarcaDegradado(t *testing.T) {
	store := &fakeStore{uplErr: errors.New("drive fora do ar")}
	obs := &recordingObserver{}
	st := &Status{}
	app := newBackupApp(t, New(store, testSealer(t), 14, obs, st))

	if err := app.CreateBackup(context.Background(), ""); err != nil {
		t.Fatalf("falha de DR não pode derrubar o backup local: %v", err)
	}
	if len(obs.mirrors) != 1 || obs.mirrors[0] != "erro" {
		t.Errorf("Mirror esperado [erro], veio %v", obs.mirrors)
	}
	if got := st.State(); got != StateDegradado {
		t.Errorf("Status: quero degradado, veio %q", got)
	}
	if len(obs.backups) < 2 || obs.backups[1] != "drive:erro" {
		t.Errorf("Backup drive:erro esperado, veio %v", obs.backups)
	}
}

// TestHook_SemStoreMirrorDesativado: DR desativado => Mirror("desativado") e o
// backup local ok.
func TestHook_SemStoreMirrorDesativado(t *testing.T) {
	obs := &recordingObserver{}
	st := &Status{}
	app := newBackupApp(t, New(nil, testSealer(t), 14, obs, st))

	if err := app.CreateBackup(context.Background(), ""); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if len(obs.mirrors) != 1 || obs.mirrors[0] != "desativado" {
		t.Errorf("Mirror esperado [desativado], veio %v", obs.mirrors)
	}
	if st.State() != StateDesativado {
		t.Errorf("Status: quero desativado, veio %q", st.State())
	}
}
