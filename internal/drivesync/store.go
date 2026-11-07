package drivesync

import (
	"context"
	"sync/atomic"
	"time"
)

// File é a visão mínima de um snapshot remoto (independente do tipo do google).
type File struct {
	ID      string
	Name    string
	Created time.Time
}

// SnapshotStore substitui o SnapshotUploader no Syncer. É satisfeita pelo
// driveUploaderAdapter (wiring), que já conhece a pasta de DR (FolderResolver)
// e as appProperties de snapshot: o drivesync não sabe de pasta nem de props.
type SnapshotStore interface {
	SnapshotUploader
	// List devolve só os snapshots do app na pasta de DR, mais recentes primeiro.
	List(ctx context.Context) ([]File, error)
	// Trash move o arquivo para a lixeira (reversível; nunca delete).
	Trash(ctx context.Context, fileID string) error
}

// Observer recebe os fatos que viram métrica. É implementado em platform/metrics
// (adapter de borda); NopObserver{} nos testes e quando métricas estão desligadas.
//
// Não há métodos para drive_dr_pronto nem para o timestamp do último sucesso:
// ambos são DERIVADOS no adapter (último Backup ok / último Mirror "ok").
type Observer interface {
	Backup(destino string, ok bool, d time.Duration, sizeBytes int64) // destino: "local" | "drive"
	Mirror(resultado string)                                          // "ok" | "erro" | "desativado"
	Retention(kept int)                                               // snapshots mantidos após a limpeza
	RetentionFailure(etapa string)                                    // "listar" | "lixeira"
}

// NopObserver descarta todos os eventos.
type NopObserver struct{}

func (NopObserver) Backup(string, bool, time.Duration, int64) {}
func (NopObserver) Mirror(string)                             {}
func (NopObserver) Retention(int)                             {}
func (NopObserver) RetentionFailure(string)                   {}

// Resultados do último espelhamento guardados no Status.
const (
	resultOK   = "ok"
	resultErro = "erro"
)

// Estados expostos por Status.State (e por /readyz em checks.drive_dr).
const (
	StateDesativado = "desativado"
	StateOK         = "ok"
	StateDegradado  = "degradado"
)

// Status guarda o último resultado do espelhamento; alimenta /readyz. Seguro
// para uso concorrente. O valor zero é "desativado" e todos os métodos são
// nil-safe.
type Status struct {
	configured atomic.Bool
	last       atomic.Value // string: resultOK | resultErro
}

// SetConfigured registra se há DR configurado (store != nil).
func (s *Status) SetConfigured(b bool) {
	if s != nil {
		s.configured.Store(b)
	}
}

// Set registra o resultado do último espelhamento ("ok" | "erro").
func (s *Status) Set(res string) {
	if s != nil {
		s.last.Store(res)
	}
}

// State resolve o estado, avaliado em ordem:
//
//	!configured                 -> "desativado"
//	último Set("erro")          -> "degradado"
//	configurado, sem tentativa  -> "ok"
//	último Set("ok")            -> "ok"
func (s *Status) State() string {
	if s == nil || !s.configured.Load() {
		return StateDesativado
	}
	if v, _ := s.last.Load().(string); v == resultErro {
		return StateDegradado
	}
	return StateOK
}
