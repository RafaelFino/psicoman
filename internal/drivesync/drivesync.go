package drivesync

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// SnapshotUploader envia o snapshot (JÁ cifrado) para o armazenamento frio de
// DR (Google Drive, escopo drive.file). Interface LOCAL do drivesync para que o
// hook seja testável com um fake, sem acoplar ao pacote integration/google nem
// tocar a rede (psicoman-google-api.md). O adapter real é satisfeito pelo
// DriveClient do google via driveUploaderAdapter (wiring).
type SnapshotUploader interface {
	// Upload envia content sob name na pasta fria; devolve o id remoto.
	Upload(ctx context.Context, name string, content []byte) (fileID string, err error)
}

// BlobSealer cifra os bytes do snapshot antes do upload (AES-256-GCM, mesmas
// chaves da cifra de campo — ver platform/crypto.BlobSealer). Interface local
// para injeção/teste.
type BlobSealer interface {
	SealBytes(plaintext []byte) ([]byte, error)
}

// Syncer espelha os backups do PocketBase para o Drive (fallback/DR). Registra
// um hook no ciclo de backup; fora do caminho de request. O Drive NUNCA é fonte
// da verdade. Sem uploader configurado (sem credenciais Google), o hook é um
// no-op observável — o backup local continua, sem derrubar o processo (D9).
type Syncer struct {
	uploader SnapshotUploader
	sealer   BlobSealer
}

// New cria o Syncer. uploader nil => DR desativado (no-op, só backup local).
func New(uploader SnapshotUploader, sealer BlobSealer) *Syncer {
	return &Syncer{uploader: uploader, sealer: sealer}
}

// Register liga o hook OnBackupCreate do PocketBase. O hook deixa o backup local
// acontecer (e.Next()) e, em seguida, lê o snapshot, cifra e sobe ao Drive.
// Falha no upload/cifra é LOGADA mas NÃO propagada: o backup local permanece
// válido e o processo não cai (DR é best-effort, fora do caminho de request).
func (s *Syncer) Register(app core.App) {
	app.OnBackupCreate().BindFunc(func(e *core.BackupEvent) error {
		// 1. Deixa o PocketBase criar o backup local normalmente.
		if err := e.Next(); err != nil {
			return err
		}
		// 2. DR desativado? No-op observável.
		if s.uploader == nil {
			e.App.Logger().Warn("Drive DR desativado: sem credenciais Google — snapshot não espelhado", "backup", e.Name)
			return nil
		}
		// 3. Espelha best-effort: erro aqui não invalida o backup local.
		if err := s.mirror(e.Context, e.App, e.Name); err != nil {
			e.App.Logger().Error("falha ao espelhar backup no Drive (DR)", "backup", e.Name, "erro", err.Error())
		}
		return nil
	})
}

// mirror lê o snapshot do backups-filesystem, cifra e faz upload ao Drive.
func (s *Syncer) mirror(ctx context.Context, app core.App, name string) error {
	fsys, err := app.NewBackupsFilesystem()
	if err != nil {
		return fmt.Errorf("drivesync: abrir backups filesystem: %w", err)
	}
	defer fsys.Close()
	fsys.SetContext(ctx)

	r, err := fsys.GetReader(name)
	if err != nil {
		return fmt.Errorf("drivesync: ler snapshot %q: %w", name, err)
	}
	defer r.Close()
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("drivesync: carregar snapshot: %w", err)
	}

	remoteName, err := s.uploadSealed(ctx, name, raw)
	if err != nil {
		return err
	}
	app.Logger().Info("snapshot espelhado no Drive (DR)", "backup", name, "arquivo", remoteName)
	return nil
}

// uploadSealed cifra raw e faz upload sob o nome derivado de name. Isolado do
// I/O de filesystem para ser testável em unit (o hook completo é coberto por E2E).
// Devolve o nome remoto usado. Requer uploader != nil (checado pelo chamador).
func (s *Syncer) uploadSealed(ctx context.Context, name string, raw []byte) (string, error) {
	sealed, err := s.sealer.SealBytes(raw)
	if err != nil {
		return "", fmt.Errorf("drivesync: cifrar snapshot: %w", err)
	}
	remoteName := encName(name)
	if _, err := s.uploader.Upload(ctx, remoteName, sealed); err != nil {
		return "", fmt.Errorf("drivesync: upload ao Drive: %w", err)
	}
	return remoteName, nil
}

// encName deriva o nome do arquivo cifrado no Drive a partir do nome do backup.
func encName(name string) string {
	if strings.HasSuffix(name, ".zip") {
		name = strings.TrimSuffix(name, ".zip")
	}
	return name + ".enc"
}
