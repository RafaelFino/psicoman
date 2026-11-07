package drivesync

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

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

// Etapas de falha do espelhamento (campo "etapa" do log de erro).
const (
	etapaCifra    = "cifra"
	etapaUpload   = "upload"
	etapaRetencao = "retencao"
)

// Syncer espelha os backups do PocketBase para o Drive (fallback/DR). Cada
// backup sobe um SNAPSHOT INTEIRO cifrado (sem incremental); a retenção por
// contagem apaga (lixeira) os mais antigos. Registra um hook no ciclo de backup;
// fora do caminho de request. O Drive NUNCA é fonte da verdade. Sem store
// configurado (sem credenciais Google), o hook é um no-op observável — o backup
// local continua, sem derrubar o processo (D9).
type Syncer struct {
	store     SnapshotStore
	sealer    BlobSealer
	retention int // snapshots mantidos; 0 = não apagar (retenção manual)
	obs       Observer
	status    *Status
}

// New cria o Syncer. store nil => DR desativado (no-op, só backup local).
// retention <= 0 desliga a limpeza. obs nil vira NopObserver; st nil vira um
// Status descartável. Registra no Status se há DR configurado.
func New(store SnapshotStore, sealer BlobSealer, retention int, obs Observer, st *Status) *Syncer {
	if obs == nil {
		obs = NopObserver{}
	}
	if st == nil {
		st = &Status{}
	}
	st.SetConfigured(store != nil)
	if retention < 0 {
		retention = 0
	}
	return &Syncer{store: store, sealer: sealer, retention: retention, obs: obs, status: st}
}

// Register liga o hook OnBackupCreate do PocketBase. O hook deixa o backup local
// acontecer (e.Next()), mede-o e, em seguida, lê o snapshot, cifra, sobe ao
// Drive e aplica a retenção. Falha no upload/cifra é LOGADA mas NÃO propagada:
// o backup local permanece válido e o processo não cai (DR é best-effort, fora
// do caminho de request).
func (s *Syncer) Register(app core.App) {
	app.OnBackupCreate().BindFunc(func(e *core.BackupEvent) error {
		// 1. Deixa o PocketBase criar o backup local normalmente e mede.
		t0 := time.Now()
		if err := e.Next(); err != nil {
			s.obs.Backup("local", false, time.Since(t0), 0)
			return err
		}
		s.obs.Backup("local", true, time.Since(t0), localSize(e.Context, e.App, e.Name))

		// 2. DR desativado? No-op observável.
		if s.store == nil {
			s.obs.Mirror("desativado")
			e.App.Logger().Warn("Drive DR desativado: sem credenciais Google — snapshot não espelhado", "backup", e.Name)
			return nil
		}
		// 3. Espelha best-effort: erro aqui não invalida o backup local.
		if etapa, err := s.mirror(e.Context, e.App, e.Name); err != nil {
			s.obs.Mirror("erro")
			s.status.Set(resultErro)
			e.App.Logger().Error("falha ao espelhar backup no Drive (DR)",
				"backup", e.Name, "etapa", etapa, "erro", err.Error())
		} else {
			s.obs.Mirror("ok")
			s.status.Set(resultOK)
		}
		return nil
	})
}

// localSize devolve o tamanho do backup local (0 se indisponível; só métrica).
func localSize(ctx context.Context, app core.App, name string) int64 {
	fsys, err := app.NewBackupsFilesystem()
	if err != nil {
		return 0
	}
	defer func() { _ = fsys.Close() }()
	fsys.SetContext(ctx)
	attrs, err := fsys.Attributes(name)
	if err != nil || attrs == nil {
		return 0
	}
	return attrs.Size
}

// mirror lê o snapshot do backups-filesystem, cifra, faz upload ao Drive e
// aplica a retenção. Devolve a etapa que falhou (para o log) junto do erro. A
// retenção NUNCA devolve erro: falha de List/Trash é logada e contada, sem
// invalidar o upload.
func (s *Syncer) mirror(ctx context.Context, app core.App, name string) (string, error) {
	fsys, err := app.NewBackupsFilesystem()
	if err != nil {
		return etapaUpload, fmt.Errorf("drivesync: abrir backups filesystem: %w", err)
	}
	defer func() { _ = fsys.Close() }()
	fsys.SetContext(ctx)

	r, err := fsys.GetReader(name)
	if err != nil {
		return etapaUpload, fmt.Errorf("drivesync: ler snapshot %q: %w", name, err)
	}
	defer func() { _ = r.Close() }()
	raw, err := io.ReadAll(r)
	if err != nil {
		return etapaUpload, fmt.Errorf("drivesync: carregar snapshot: %w", err)
	}

	t0 := time.Now()
	remoteName, fileID, sealedSize, err := s.uploadSealed(ctx, name, raw)
	if err != nil {
		s.obs.Backup("drive", false, time.Since(t0), 0)
		etapa := etapaUpload
		if strings.Contains(err.Error(), "cifrar") {
			etapa = etapaCifra
		}
		return etapa, err
	}
	s.obs.Backup("drive", true, time.Since(t0), sealedSize)
	app.Logger().Info("snapshot espelhado no Drive (DR)", "backup", name, "arquivo", remoteName)

	s.applyRetention(ctx, app, fileID)
	return "", nil
}

// applyRetention apaga (lixeira) os snapshots além dos `retention` mais
// recentes. Só roda após upload ok e retention > 0. Nunca propaga falha.
func (s *Syncer) applyRetention(ctx context.Context, app core.App, justUploadedID string) {
	if s.retention <= 0 {
		return
	}
	files, err := s.store.List(ctx)
	if err != nil {
		s.obs.RetentionFailure("listar")
		app.Logger().Warn("retenção do Drive: falha ao listar snapshots", "etapa", etapaRetencao, "erro", err.Error())
		return
	}
	kept := countSnapshots(files)
	if !containsID(files, justUploadedID) {
		kept++ // a listagem ainda não mostra o recém-enviado
	}
	for _, id := range selectToTrash(files, s.retention, justUploadedID) {
		if err := s.store.Trash(ctx, id); err != nil {
			s.obs.RetentionFailure("lixeira")
			app.Logger().Warn("retenção do Drive: falha ao mover para a lixeira", "etapa", etapaRetencao, "erro", err.Error())
			continue
		}
		kept--
	}
	s.obs.Retention(kept)
}

func containsID(files []File, id string) bool {
	for _, f := range files {
		if f.ID == id {
			return true
		}
	}
	return false
}

// uploadSealed cifra raw e faz upload sob o nome derivado de name. Isolado do
// I/O de filesystem para ser testável em unit (o hook completo é coberto por E2E).
// Devolve o nome remoto, o id remoto e o tamanho do blob cifrado. Requer
// store != nil (checado pelo chamador).
func (s *Syncer) uploadSealed(ctx context.Context, name string, raw []byte) (remoteName, fileID string, size int64, err error) {
	sealed, err := s.sealer.SealBytes(raw)
	if err != nil {
		return "", "", 0, fmt.Errorf("drivesync: cifrar snapshot: %w", err)
	}
	remoteName = encName(name)
	fileID, err = s.store.Upload(ctx, remoteName, sealed)
	if err != nil {
		return "", "", 0, fmt.Errorf("drivesync: upload ao Drive: %w", err)
	}
	return remoteName, fileID, int64(len(sealed)), nil
}

// encName deriva o nome do arquivo cifrado no Drive a partir do nome do backup.
func encName(name string) string {
	return strings.TrimSuffix(name, ".zip") + snapshotSuffix
}
