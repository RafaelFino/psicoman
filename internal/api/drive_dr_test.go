package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/drivesync"
	"github.com/RafaelFino/psicoman/internal/integration/google"
	"github.com/RafaelFino/psicoman/internal/platform/crypto"
	"github.com/RafaelFino/psicoman/internal/platform/pb"
)

// withDriveSync reatribui o seam driveSyncWirer para montar o pipeline de DR
// real (FolderResolver + driveUploaderAdapter + Syncer) sobre um google.DriveClient
// fake — NENHUM teste toca a rede/API do Google. retention é o número de
// snapshots mantidos (0 = retenção manual). O Status é o MESMO do wiring, então
// o /readyz reflete o resultado.
func withDriveSync(t *testing.T, client google.DriveClient, retention int) {
	t.Helper()
	prev := driveSyncWirer
	driveSyncWirer = func(a core.App, kp *crypto.MapKeyProvider, _ *app.GoogleAuthService,
		folders app.DriveFolderRepository, obs drivesync.Observer, st *drivesync.Status) {
		sealer := crypto.NewBlobSealer(kp)
		resolver := drivesync.NewFolderResolver(
			"", // sem drive_folder: força busca/criação automática
			&driveFolderClientAdapter{client: client},
			&driveFolderStoreAdapter{repo: folders},
		)
		uploader := &driveUploaderAdapter{client: client, resolver: resolver}
		drivesync.New(uploader, sealer, retention, obs, st).Register(a)
	}
	t.Cleanup(func() { driveSyncWirer = prev })
}

// backup dispara POST /v1/admin/backup e exige 202.
func backup(t *testing.T, c *apiClient) {
	t.Helper()
	if res := c.do(http.MethodPost, "/v1/admin/backup", "", appHeaders()); res.status != http.StatusAccepted {
		t.Fatalf("backup: status %d (%s)", res.status, res.body)
	}
}

// driveDRState lê checks.drive_dr de /v1/readyz ("" se ausente) e confere que
// a instância segue pronta (o DR nunca derruba o ready).
func driveDRState(t *testing.T, c *apiClient) string {
	t.Helper()
	res := c.do(http.MethodGet, "/v1/readyz", "", nil)
	if res.status != http.StatusOK {
		t.Fatalf("/readyz: quero 200, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"ready":true`) {
		t.Fatalf("DR não pode alterar ready: %s", res.body)
	}
	for _, st := range []string{"desativado", "degradado", "ok"} {
		if strings.Contains(res.body, `"drive_dr":"`+st+`"`) {
			return st
		}
	}
	return ""
}

// TestDriveDR_CriaPastaEReusa: o primeiro backup cria a pasta de DR (1
// CreateFolder), sobe o snapshot com o FolderID da pasta criada e persiste o id
// em drive_dr; o segundo backup reusa a pasta sem nova criação.
func TestDriveDR_CriaPastaEReusa(t *testing.T) {
	fake := &google.FakeDriveClient{FolderID: "folder-dr", FolderFound: false, FileID: "snap-1"}
	withDriveSync(t, fake, 0)
	app := newTestApp(t)
	c := newClient(t, app)

	backup(t, c)
	if fake.CreateFolderCall != 1 {
		t.Fatalf("1º backup deveria criar a pasta uma vez, veio %d", fake.CreateFolderCall)
	}
	if len(fake.Uploads) != 1 {
		t.Fatalf("1º backup deveria subir 1 snapshot, veio %d", len(fake.Uploads))
	}
	if fake.Uploads[0].FolderID != "folder-dr" {
		t.Errorf("upload deveria usar a pasta criada, veio %q", fake.Uploads[0].FolderID)
	}
	if fake.Uploads[0].AppProps["psicoman_kind"] != "snapshot" {
		t.Errorf("upload deveria marcar psicoman_kind=snapshot, veio %+v", fake.Uploads[0].AppProps)
	}

	// Registro persistido em drive_dr.
	repo := pb.NewDriveFolderRepository(app)
	if id, err := repo.Load(context.Background()); err != nil || id != "folder-dr" {
		t.Fatalf("drive_dr deveria ter o id da pasta, veio id=%q err=%v", id, err)
	}

	// 2º backup: reusa a pasta (cache em memória do resolver), sem nova criação.
	backup(t, c)
	if fake.CreateFolderCall != 1 {
		t.Errorf("2º backup NÃO deveria criar outra pasta, create=%d", fake.CreateFolderCall)
	}
	if len(fake.Uploads) != 2 {
		t.Errorf("2º backup deveria subir outro snapshot, uploads=%d", len(fake.Uploads))
	}
}

// TestDriveDR_PastaObsoletaRecriaUmaVez: se o upload devolve 404 na pasta
// (persistida obsoleta), o adapter invalida o resolver e recria a pasta uma vez,
// repetindo o upload. Uma segunda falha não vira laço.
func TestDriveDR_PastaObsoletaRecriaUmaVez(t *testing.T) {
	fake := &recriaDriveClient{folderID: "folder-novo", fileID: "snap-x", failFirstUpload: true}
	withDriveSync(t, fake, 0)
	app := newTestApp(t)
	c := newClient(t, app)

	backup(t, c)
	if fake.createCalls < 1 {
		t.Errorf("deveria ter recriado a pasta ao menos uma vez, create=%d", fake.createCalls)
	}
	if fake.uploadCalls != 2 {
		t.Errorf("deveria ter repetido o upload UMA vez após recriar, upload=%d", fake.uploadCalls)
	}
}

// TestDriveDR_PastaObsoleta404PersistenteNaoFazLaco: se o upload devolve 404
// SEMPRE, o adapter tenta no máximo duas vezes (original + 1 retry), o backup
// local segue válido e o DR fica degradado.
func TestDriveDR_PastaObsoleta404PersistenteNaoFazLaco(t *testing.T) {
	fake := &recriaDriveClient{folderID: "folder-novo", fileID: "x", alwaysFail: true}
	withDriveSync(t, fake, 0)
	app := newTestApp(t)
	c := newClient(t, app)

	backup(t, c) // 202: a falha de DR não derruba o backup
	if fake.uploadCalls != 2 {
		t.Errorf("sem laço: esperava exatamente 2 uploads, veio %d", fake.uploadCalls)
	}
	if got := driveDRState(t, c); got != "degradado" {
		t.Errorf("drive_dr: quero degradado, veio %q", got)
	}
}

// TestDriveDR_RetencaoEReadyz: vários backups com retenção pequena mandam os
// excedentes para a lixeira (nunca delete) e o /readyz reflete ok -> degradado
// quando o upload passa a falhar, sem nunca derrubar o ready.
func TestDriveDR_RetencaoEReadyz(t *testing.T) {
	fake := &google.FakeDriveClient{
		FolderID: "folder-dr", FileID: "novo",
		Files: []google.DriveFile{
			{ID: "novo", Name: "n.enc"}, {ID: "a", Name: "a.enc"},
			{ID: "b", Name: "b.enc"}, {ID: "c", Name: "c.enc"},
			{ID: "manual", Name: "anotacao.txt"}, // sem .enc: nunca tocado
		},
	}
	withDriveSync(t, fake, 2)
	app := newTestApp(t)
	c := newClient(t, app)

	// Configurado e sem tentativa ainda => ok.
	if got := driveDRState(t, c); got != "ok" {
		t.Fatalf("antes do 1º backup: quero ok, veio %q", got)
	}

	backup(t, c)
	if len(fake.Trashed) != 2 || fake.Trashed[0] != "b" || fake.Trashed[1] != "c" {
		t.Fatalf("lixeira esperada [b c], veio %v", fake.Trashed)
	}
	if got := driveDRState(t, c); got != "ok" {
		t.Errorf("após backup ok: quero ok, veio %q", got)
	}

	// Falha forçada de upload => degradado.
	fake.UploadErr = errors.New("drive fora do ar")
	backup(t, c)
	if got := driveDRState(t, c); got != "degradado" {
		t.Errorf("após falha: quero degradado, veio %q", got)
	}

	// Recupera => volta a ok.
	fake.UploadErr = nil
	backup(t, c)
	if got := driveDRState(t, c); got != "ok" {
		t.Errorf("após recuperar: quero ok, veio %q", got)
	}
}

// TestDriveDR_SemCredenciaisDesativado: sem store (DR desativado), o /readyz
// mostra drive_dr=desativado e o backup local segue (202).
func TestDriveDR_SemCredenciaisDesativado(t *testing.T) {
	prev := driveSyncWirer
	driveSyncWirer = func(a core.App, kp *crypto.MapKeyProvider, _ *app.GoogleAuthService,
		_ app.DriveFolderRepository, obs drivesync.Observer, st *drivesync.Status) {
		drivesync.New(nil, crypto.NewBlobSealer(kp), 14, obs, st).Register(a)
	}
	t.Cleanup(func() { driveSyncWirer = prev })

	app := newTestApp(t)
	c := newClient(t, app)

	if got := driveDRState(t, c); got != "desativado" {
		t.Errorf("sem credenciais: quero desativado, veio %q", got)
	}
	backup(t, c)
	if got := driveDRState(t, c); got != "desativado" {
		t.Errorf("após backup sem DR: quero desativado, veio %q", got)
	}
}

// recriaDriveClient implementa google.DriveClient. O primeiro Upload devolve
// google.ErrDriveNotFound (pasta obsoleta; ou todos, com alwaysFail); os demais
// funcionam. FindFolder nunca acha (força criação); CreateFolder devolve folderID.
type recriaDriveClient struct {
	folderID        string
	fileID          string
	failFirstUpload bool
	alwaysFail      bool
	uploadCalls     int
	createCalls     int
}

var _ google.DriveClient = (*recriaDriveClient)(nil)

func (f *recriaDriveClient) Upload(_ context.Context, _ google.DriveUploadInput) (*google.DriveUploadResult, error) {
	f.uploadCalls++
	if f.alwaysFail || (f.failFirstUpload && f.uploadCalls == 1) {
		return nil, google.ErrDriveNotFound
	}
	return &google.DriveUploadResult{FileID: f.fileID}, nil
}

func (f *recriaDriveClient) List(context.Context, google.DriveListInput) ([]google.DriveFile, error) {
	return nil, nil
}

func (f *recriaDriveClient) Trash(context.Context, string) error { return nil }

func (f *recriaDriveClient) FindFolder(context.Context, string, map[string]string) (string, bool, error) {
	return "", false, nil
}

func (f *recriaDriveClient) CreateFolder(context.Context, string, map[string]string) (string, error) {
	f.createCalls++
	return f.folderID, nil
}

func (f *recriaDriveClient) GetFolder(context.Context, string) (bool, error) {
	return false, nil
}
