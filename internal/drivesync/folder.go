package drivesync

import (
	"context"
	"errors"
	"sync"
)

// Sentinelas LOCAIS do drivesync. O pacote NÃO importa integration/google nem
// app (regra de dependência): o adapter do wiring traduz google.ErrDriveNotFound
// em ErrFolderNotFound e app.ErrNotFound em ErrNoStoredFolder.
var (
	// ErrFolderNotFound: o Drive respondeu 404 para a pasta (ou para o pai no upload).
	ErrFolderNotFound = errors.New("drivesync: pasta não encontrada")
	// ErrNoStoredFolder: FolderStore.Load não tem id persistido ("não existe").
	ErrNoStoredFolder = errors.New("drivesync: nenhuma pasta persistida")
)

// Nome e appProperties da pasta de DR. O resolver os usa na busca/criação.
const (
	drFolderName = "Psicoman DR"
	drRoleKey    = "psicoman_role"
	drRoleValue  = "dr_folder"
)

// FolderClient é o subconjunto do DriveClient que o resolver usa, implementado
// pelo adapter do wiring sobre google.DriveClient.
type FolderClient interface {
	FindFolder(ctx context.Context, name string, appProps map[string]string) (string, bool, error)
	CreateFolder(ctx context.Context, name string, appProps map[string]string) (string, error)
	GetFolder(ctx context.Context, id string) (trashed bool, err error) // ErrFolderNotFound em 404
}

// FolderStore persiste o id da pasta resolvida. Satisfeito por um adapter fino
// sobre app.DriveFolderRepository (que converte app.ErrNotFound em
// ErrNoStoredFolder).
type FolderStore interface {
	Load(ctx context.Context) (string, error) // ErrNoStoredFolder se não houver
	Save(ctx context.Context, id string) error
}

// FolderResolver resolve (lazy) o id da pasta de DR na ordem:
//  1. configured (google.drive_folder): usado como está; nunca revalidado,
//     nunca recriado, nunca persistido (o operador manda).
//  2. persistido (FolderStore): revalidado UMA vez por processo via GetFolder;
//     se na lixeira/404, descartado e segue para 3/4.
//  3. busca por nome+appProperties (só o que o app criou); persiste se achar.
//  4. criação (CreateFolder); persiste o id novo.
//
// mu serializa resoluções concorrentes (um processo, backups sequenciais).
type FolderResolver struct {
	configured string
	client     FolderClient
	store      FolderStore

	mu        sync.Mutex
	cached    string // id resolvido em memória
	validated bool   // o id persistido já foi revalidado neste processo?
}

// NewFolderResolver cria o resolver. configured vindo de google.drive_folder
// ("" = resolução automática).
func NewFolderResolver(configured string, c FolderClient, s FolderStore) *FolderResolver {
	return &FolderResolver{configured: configured, client: c, store: s}
}

// ID devolve o id da pasta de DR, resolvendo-o na primeira chamada e reusando o
// cache nas seguintes.
func (r *FolderResolver) ID(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Config vence e nunca é revalidado/recriado.
	if r.configured != "" {
		return r.configured, nil
	}

	// Cache em memória (já resolvido neste processo).
	if r.cached != "" {
		return r.cached, nil
	}

	// 2. Id persistido, revalidado uma vez por processo.
	if !r.validated {
		id, err := r.store.Load(ctx)
		switch {
		case err == nil && id != "":
			r.validated = true
			trashed, gerr := r.client.GetFolder(ctx, id)
			switch {
			case gerr == nil && !trashed:
				r.cached = id
				return id, nil
			case errors.Is(gerr, ErrFolderNotFound) || (gerr == nil && trashed):
				// Obsoleto: segue para busca/criação (não relê o persistido).
			default:
				return "", gerr
			}
		case errors.Is(err, ErrNoStoredFolder):
			r.validated = true // não há persistido; segue para busca/criação
		default:
			return "", err
		}
	}

	// 3. Busca por nome+appProperties (só o que o app criou).
	appProps := map[string]string{drRoleKey: drRoleValue}
	if id, found, err := r.client.FindFolder(ctx, drFolderName, appProps); err != nil {
		return "", err
	} else if found {
		if err := r.store.Save(ctx, id); err != nil {
			return "", err
		}
		r.cached = id
		return id, nil
	}

	// 4. Criação.
	id, err := r.client.CreateFolder(ctx, drFolderName, appProps)
	if err != nil {
		return "", err // falha de criação não persiste nada
	}
	if err := r.store.Save(ctx, id); err != nil {
		return "", err
	}
	r.cached = id
	return id, nil
}

// Invalidate descarta o cache em memória do id obsoleto (staleID), forçando a
// próxima ID a pular o passo 2 (persistido) e ir à busca/criação. Ignorado
// quando a pasta vem do config (o operador manda; nunca recriamos).
func (r *FolderResolver) Invalidate(staleID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.configured != "" {
		return
	}
	if r.cached == staleID || staleID == "" {
		r.cached = ""
	}
	// validated permanece true: a próxima ID não relê o persistido, vai direto
	// à busca/criação (o Save do passo 3/4 sobrescreve o registro obsoleto).
	r.validated = true
}
