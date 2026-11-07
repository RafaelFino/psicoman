package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/drivesync"
	"github.com/RafaelFino/psicoman/internal/integration/google"
	"github.com/RafaelFino/psicoman/internal/platform/crypto"
	"github.com/RafaelFino/psicoman/internal/platform/pb"
)

// withFakeDriveSync reatribui o seam driveSyncWirer para montar o pipeline de
// DR real (FolderResolver + driveUploaderAdapter + Syncer) sobre um
// FakeDriveClient — NENHUM teste toca a rede/API do Google. Devolve o fake para
// inspeção. kp da cifra vem da MESMA chave de teste (cipherConfigLoader).
func withFakeDriveSync(t *testing.T, fake *google.FakeDriveClient) {
	t.Helper()
	prev := driveSyncWirer
	driveSyncWirer = func(a core.App, kp *crypto.MapKeyProvider, _ *app.GoogleAuthService, folders app.DriveFolderRepository) {
		sealer := crypto.NewBlobSealer(kp)
		resolver := drivesync.NewFolderResolver(
			"", // sem drive_folder: força busca/criação automática
			&driveFolderClientAdapter{client: fake},
			&driveFolderStoreAdapter{repo: folders},
		)
		uploader := &driveUploaderAdapter{client: fake, resolver: resolver}
		drivesync.New(uploader, sealer).Register(a)
	}
	t.Cleanup(func() { driveSyncWirer = prev })
}

// TestDriveDR_CriaPastaEReusa: o primeiro backup cria a pasta de DR (1
// CreateFolder), sobe o snapshot com o FolderID da pasta criada e persiste o id
// em drive_dr; o segundo backup reusa a pasta sem nova criação.
func TestDriveDR_CriaPastaEReusa(t *testing.T) {
	fake := &google.FakeDriveClient{FolderID: "folder-dr", FolderFound: false, FileID: "snap-1"}
	withFakeDriveSync(t, fake)
	app := newTestApp(t)
	c := newClient(t, app)

	// 1º backup.
	if res := c.do(http.MethodPost, "/v1/admin/backup", "", appHeaders()); res.status != http.StatusAccepted {
		t.Fatalf("1º backup: status %d (%s)", res.status, res.body)
	}
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
		t.Errorf("upload deveria marcar appProperties psicoman_kind=snapshot, veio %+v", fake.Uploads[0].AppProps)
	}

	// Registro persistido em drive_dr.
	repo := pb.NewDriveFolderRepository(app)
	if id, err := repo.Load(context.Background()); err != nil || id != "folder-dr" {
		t.Fatalf("drive_dr deveria ter o id da pasta, veio id=%q err=%v", id, err)
	}

	// 2º backup: reusa a pasta (cache em memória do resolver), sem nova criação.
	if res := c.do(http.MethodPost, "/v1/admin/backup", "", appHeaders()); res.status != http.StatusAccepted {
		t.Fatalf("2º backup: status %d (%s)", res.status, res.body)
	}
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
	prev := driveSyncWirer
	driveSyncWirer = func(a core.App, kp *crypto.MapKeyProvider, _ *app.GoogleAuthService, folders app.DriveFolderRepository) {
		sealer := crypto.NewBlobSealer(kp)
		resolver := drivesync.NewFolderResolver("",
			&driveFolderClientAdapter{client: fake},
			&driveFolderStoreAdapter{repo: folders})
		uploader := &driveUploaderAdapter{client: fake, resolver: resolver}
		drivesync.New(uploader, sealer).Register(a)
	}
	t.Cleanup(func() { driveSyncWirer = prev })

	app := newTestApp(t)
	c := newClient(t, app)

	if res := c.do(http.MethodPost, "/v1/admin/backup", "", appHeaders()); res.status != http.StatusAccepted {
		t.Fatalf("backup com pasta obsoleta: status %d (%s)", res.status, res.body)
	}
	if fake.createCalls < 1 {
		t.Errorf("deveria ter recriado a pasta ao menos uma vez, create=%d", fake.createCalls)
	}
	if fake.uploadCalls < 2 {
		t.Errorf("deveria ter repetido o upload após recriar, upload=%d", fake.uploadCalls)
	}
}

// recriaDriveClient implementa google.DriveClient. O primeiro Upload devolve
// google.ErrDriveNotFound (pasta obsoleta); os seguintes funcionam. FindFolder
// nunca acha (força criação); CreateFolder sempre devolve folderID.
type recriaDriveClient struct {
	folderID        string
	fileID          string
	failFirstUpload bool
	uploadCalls     int
	createCalls     int
}

var _ google.DriveClient = (*recriaDriveClient)(nil)

func (f *recriaDriveClient) Upload(_ context.Context, _ google.DriveUploadInput) (*google.DriveUploadResult, error) {
	f.uploadCalls++
	if f.failFirstUpload && f.uploadCalls == 1 {
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
