package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

const (
	snapshotName = "psicoman-backup.db.gz.enc"
	manifestName = "psicoman-ged-manifest.json"
)

// backupCipher cifra/decifra o snapshot (platform/crypto.Cipher).
type backupCipher interface {
	Encrypt(plaintext []byte) (string, error)
	Decrypt(encoded string) ([]byte, error)
}

// GEDManifestSource lista os arquivos do GED (path+sha) para backup incremental.
type GEDManifestSource interface {
	AllFiles(ctx context.Context) ([]GEDFileRef, error)
	ReadRaw(ctx context.Context, relPath string) ([]byte, error)
}

// GEDFileRef é a referência mínima de um arquivo do GED para backup.
type GEDFileRef struct {
	RelPath string
	SHA256  string
}

// gedFileLister devolve todas as referências de arquivos do GED (repo de metadados).
type gedFileLister interface {
	AllFiles(ctx context.Context) ([]GEDFileRef, error)
}

// gedRawReader lê o conteúdo bruto de um arquivo do GED (store físico).
type gedRawReader interface {
	ReadRaw(relPath string) ([]byte, error)
}

// gedManifestAdapter combina o repo de metadados e o store físico numa fonte
// única para o backup incremental.
type gedManifestAdapter struct {
	lister gedFileLister
	reader gedRawReader
}

// NewGEDManifestSource cria a fonte de manifesto do GED para o backup.
func NewGEDManifestSource(lister interface {
	AllFiles(ctx context.Context) ([]GEDFileRef, error)
}, reader interface {
	ReadRaw(relPath string) ([]byte, error)
}) GEDManifestSource {
	return gedManifestAdapter{lister: lister, reader: reader}
}

func (a gedManifestAdapter) AllFiles(ctx context.Context) ([]GEDFileRef, error) {
	return a.lister.AllFiles(ctx)
}

func (a gedManifestAdapter) ReadRaw(_ context.Context, relPath string) ([]byte, error) {
	return a.reader.ReadRaw(relPath)
}

// BackupService faz backup cifrado do SQLite e do GED (incremental) no Drive, e
// restaura a base a partir do último snapshot (docs/architecture.md §4.4).
type BackupService struct {
	db          *sql.DB
	dbPath      string
	cipher      backupCipher
	drive       DriveClient
	driveFolder string
	ged         GEDManifestSource
	audit       *AuditService
	clock       clock.Clock
}

// NewBackupService cria o serviço de backup.
func NewBackupService(db *sql.DB, dbPath string, cipher backupCipher, drive DriveClient, driveFolder string, ged GEDManifestSource, audit *AuditService) *BackupService {
	return &BackupService{
		db: db, dbPath: dbPath, cipher: cipher, drive: drive,
		driveFolder: driveFolder, ged: ged, audit: audit, clock: clock.System{},
	}
}

// BackupResult resume o backup executado.
type BackupResult struct {
	SnapshotFileID string `json:"snapshot_file_id"`
	GEDUploaded    int    `json:"ged_uploaded"`
	GEDSkipped     int    `json:"ged_skipped"`
}

// Backup gera o snapshot cifrado do SQLite, sobe ao Drive e faz o backup
// incremental do GED (só arquivos cujo hash ainda não está no manifesto).
func (s *BackupService) Backup(ctx context.Context) (*BackupResult, error) {
	// 1. Snapshot consistente do SQLite via VACUUM INTO (arquivo temporário).
	tmp, err := os.CreateTemp("", "psicoman-snap-*.db")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, tmpPath); err != nil {
		return nil, fmt.Errorf("backup: VACUUM INTO: %w", err)
	}
	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}

	// 2. Compacta (gzip) + cifra (AES-GCM).
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	enc, err := s.cipher.Encrypt(gz.Bytes())
	if err != nil {
		return nil, err
	}

	// 3. Upload do snapshot ao Drive.
	fileID, err := s.drive.Upload(ctx, s.driveFolder, DriveFile{
		Name: snapshotName, Content: []byte(enc), MIME: "application/octet-stream",
	})
	if err != nil {
		return nil, fmt.Errorf("backup: upload snapshot: %w", err)
	}

	// 4. GED incremental: envia só arquivos com hash ainda não backupeado.
	uploaded, skipped, err := s.backupGED(ctx)
	if err != nil {
		return nil, err
	}

	res := &BackupResult{SnapshotFileID: fileID, GEDUploaded: uploaded, GEDSkipped: skipped}
	_ = s.audit.Record(ctx, "sistema", "backup", "backup", "", map[string]any{
		"ged_uploaded": uploaded, "ged_skipped": skipped,
	})
	return res, nil
}

// backupGED envia ao Drive apenas os arquivos do GED cujo hash não consta no
// manifesto remoto. Atualiza o manifesto ao final.
func (s *BackupService) backupGED(ctx context.Context) (uploaded, skipped int, err error) {
	if s.ged == nil {
		return 0, 0, nil
	}
	manifest := s.loadManifest(ctx)
	files, err := s.ged.AllFiles(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, f := range files {
		if manifest[f.SHA256] {
			skipped++
			continue
		}
		content, err := s.ged.ReadRaw(ctx, f.RelPath)
		if err != nil {
			return uploaded, skipped, err
		}
		if _, err := s.drive.Upload(ctx, s.driveFolder, DriveFile{
			Name: "ged-" + f.SHA256, Content: content, MIME: "application/octet-stream",
		}); err != nil {
			return uploaded, skipped, err
		}
		manifest[f.SHA256] = true
		uploaded++
	}
	if err := s.saveManifest(ctx, manifest); err != nil {
		return uploaded, skipped, err
	}
	return uploaded, skipped, nil
}

// loadManifest lê o manifesto do Drive (hashes já backupeados).
func (s *BackupService) loadManifest(ctx context.Context) map[string]bool {
	m := map[string]bool{}
	names, err := s.drive.List(ctx, s.driveFolder)
	if err != nil {
		return m
	}
	id, ok := names[manifestName]
	if !ok {
		return m
	}
	data, err := s.drive.Download(ctx, id)
	if err != nil {
		return m
	}
	var hashes []string
	if err := json.Unmarshal(data, &hashes); err == nil {
		for _, h := range hashes {
			m[h] = true
		}
	}
	return m
}

// saveManifest grava o manifesto atualizado no Drive.
func (s *BackupService) saveManifest(ctx context.Context, m map[string]bool) error {
	hashes := make([]string, 0, len(m))
	for h := range m {
		hashes = append(hashes, h)
	}
	data, _ := json.Marshal(hashes)
	_, err := s.drive.Upload(ctx, s.driveFolder, DriveFile{
		Name: manifestName, Content: data, MIME: "application/json",
	})
	return err
}

// Restore baixa o último snapshot do Drive, decifra, descompacta e substitui a
// base atual (guardando um backup de segurança da base substituída).
func (s *BackupService) Restore(ctx context.Context) error {
	names, err := s.drive.List(ctx, s.driveFolder)
	if err != nil {
		return err
	}
	id, ok := names[snapshotName]
	if !ok {
		return NewValidation("Nenhum backup encontrado no Drive.")
	}
	encData, err := s.drive.Download(ctx, id)
	if err != nil {
		return err
	}
	gzData, err := s.cipher.Decrypt(string(encData))
	if err != nil {
		return fmt.Errorf("restore: decifrando snapshot: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(gzData))
	if err != nil {
		return fmt.Errorf("restore: descompactando: %w", err)
	}
	dbBytes, err := io.ReadAll(zr)
	_ = zr.Close()
	if err != nil {
		return fmt.Errorf("restore: lendo snapshot: %w", err)
	}
	// Valida que é um arquivo SQLite (header mágico).
	if len(dbBytes) < 16 || string(dbBytes[:15]) != "SQLite format 3" {
		return NewValidation("Backup inválido: não parece um banco SQLite.")
	}

	// Backup de segurança da base atual antes de substituir.
	safety := s.dbPath + ".pre-restore." + s.clock.Now().Format("20060102150405")
	if cur, err := os.ReadFile(s.dbPath); err == nil {
		_ = os.WriteFile(safety, cur, 0o640)
	}
	// Escreve o snapshot restaurado no lugar da base.
	if err := os.WriteFile(s.dbPath, dbBytes, 0o640); err != nil {
		return fmt.Errorf("restore: gravando base: %w", err)
	}
	_ = s.audit.Record(ctx, "sistema", "restore", "backup", "", map[string]any{"safety_copy": filepath.Base(safety)})
	return nil
}
