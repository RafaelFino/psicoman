package drivesync

import (
	"context"
	"errors"
	"testing"
)

// fakeFolderClient registra as chamadas e devolve respostas controladas.
type fakeFolderClient struct {
	findID     string
	findFound  bool
	findErr    error
	createID   string
	createErr  error
	getTrashed bool
	getErr     error

	findCalls   int
	createCalls int
	getCalls    int
}

func (f *fakeFolderClient) FindFolder(_ context.Context, _ string, _ map[string]string) (string, bool, error) {
	f.findCalls++
	return f.findID, f.findFound, f.findErr
}
func (f *fakeFolderClient) CreateFolder(_ context.Context, _ string, _ map[string]string) (string, error) {
	f.createCalls++
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.createID, nil
}
func (f *fakeFolderClient) GetFolder(_ context.Context, _ string) (bool, error) {
	f.getCalls++
	return f.getTrashed, f.getErr
}

// fakeFolderStore é um FolderStore em memória.
type fakeFolderStore struct {
	id      string
	has     bool
	saved   []string
	loadErr error
	saveErr error
}

func (s *fakeFolderStore) Load(_ context.Context) (string, error) {
	if s.loadErr != nil {
		return "", s.loadErr
	}
	if !s.has {
		return "", ErrNoStoredFolder
	}
	return s.id, nil
}
func (s *fakeFolderStore) Save(_ context.Context, id string) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = append(s.saved, id)
	s.id = id
	s.has = true
	return nil
}

func TestFolderResolver_ConfigVenceSemChamarClient(t *testing.T) {
	c := &fakeFolderClient{}
	s := &fakeFolderStore{}
	r := NewFolderResolver("config-folder", c, s)

	id, err := r.ID(context.Background())
	if err != nil || id != "config-folder" {
		t.Fatalf("config deveria vencer, veio id=%q err=%v", id, err)
	}
	if c.findCalls+c.createCalls+c.getCalls != 0 {
		t.Errorf("config não deveria tocar o client: find=%d create=%d get=%d", c.findCalls, c.createCalls, c.getCalls)
	}
	if len(s.saved) != 0 {
		t.Error("config nunca persiste")
	}
}

func TestFolderResolver_PersistidoReusadoEValidadoUmaVez(t *testing.T) {
	c := &fakeFolderClient{getTrashed: false}
	s := &fakeFolderStore{id: "persistido", has: true}
	r := NewFolderResolver("", c, s)

	id, err := r.ID(context.Background())
	if err != nil || id != "persistido" {
		t.Fatalf("esperava persistido, veio id=%q err=%v", id, err)
	}
	// Segunda chamada usa o cache: GetFolder não é chamado de novo.
	if _, err := r.ID(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.getCalls != 1 {
		t.Errorf("GetFolder deveria ser chamado uma vez por processo, veio %d", c.getCalls)
	}
}

func TestFolderResolver_PersistidoNaLixeiraRecria(t *testing.T) {
	c := &fakeFolderClient{getTrashed: true, createID: "nova"}
	s := &fakeFolderStore{id: "velha", has: true}
	r := NewFolderResolver("", c, s)

	id, err := r.ID(context.Background())
	if err != nil || id != "nova" {
		t.Fatalf("pasta na lixeira deveria recriar, veio id=%q err=%v", id, err)
	}
	if c.createCalls != 1 {
		t.Errorf("deveria ter criado uma pasta, create=%d", c.createCalls)
	}
	if len(s.saved) != 1 || s.saved[0] != "nova" {
		t.Errorf("o id novo deveria ser persistido, saved=%v", s.saved)
	}
}

func TestFolderResolver_Persistido404Recria(t *testing.T) {
	c := &fakeFolderClient{getErr: ErrFolderNotFound, createID: "nova"}
	s := &fakeFolderStore{id: "velha", has: true}
	r := NewFolderResolver("", c, s)

	id, err := r.ID(context.Background())
	if err != nil || id != "nova" {
		t.Fatalf("404 no persistido deveria recriar, veio id=%q err=%v", id, err)
	}
}

func TestFolderResolver_BuscaAchaEPersiste(t *testing.T) {
	c := &fakeFolderClient{findID: "achada", findFound: true}
	s := &fakeFolderStore{has: false}
	r := NewFolderResolver("", c, s)

	id, err := r.ID(context.Background())
	if err != nil || id != "achada" {
		t.Fatalf("busca deveria achar, veio id=%q err=%v", id, err)
	}
	if c.createCalls != 0 {
		t.Error("achou na busca: não deveria criar")
	}
	if len(s.saved) != 1 || s.saved[0] != "achada" {
		t.Errorf("busca deveria persistir o id, saved=%v", s.saved)
	}
}

func TestFolderResolver_CriacaoQuandoNaoAcha(t *testing.T) {
	c := &fakeFolderClient{findFound: false, createID: "criada"}
	s := &fakeFolderStore{has: false}
	r := NewFolderResolver("", c, s)

	id, err := r.ID(context.Background())
	if err != nil || id != "criada" {
		t.Fatalf("deveria criar, veio id=%q err=%v", id, err)
	}
	if len(s.saved) != 1 || s.saved[0] != "criada" {
		t.Errorf("criação deveria persistir, saved=%v", s.saved)
	}
}

func TestFolderResolver_DoisIDsUmCreate(t *testing.T) {
	c := &fakeFolderClient{findFound: false, createID: "criada"}
	s := &fakeFolderStore{has: false}
	r := NewFolderResolver("", c, s)

	_, _ = r.ID(context.Background())
	_, _ = r.ID(context.Background())
	if c.createCalls != 1 {
		t.Errorf("dois ID() seguidos = 1 CreateFolder, veio %d", c.createCalls)
	}
}

func TestFolderResolver_FalhaCreateNaoPersiste(t *testing.T) {
	c := &fakeFolderClient{findFound: false, createErr: errors.New("boom")}
	s := &fakeFolderStore{has: false}
	r := NewFolderResolver("", c, s)

	if _, err := r.ID(context.Background()); err == nil {
		t.Fatal("falha de CreateFolder deveria propagar erro")
	}
	if len(s.saved) != 0 {
		t.Errorf("falha de criação não deveria persistir, saved=%v", s.saved)
	}
}

func TestFolderResolver_InvalidateNaoReleOPersistido(t *testing.T) {
	// Começa resolvendo o persistido.
	c2 := &fakeFolderClient{getTrashed: false, findFound: false, createID: "criada"}
	store := &fakeFolderStore{id: "velha", has: true}
	r := NewFolderResolver("", c2, store)
	if id, err := r.ID(context.Background()); err != nil || id != "velha" {
		t.Fatalf("setup: esperava velha, veio id=%q err=%v", id, err)
	}
	// Invalida e resolve de novo: NÃO deve reler o persistido (vai à busca/criação).
	r.Invalidate("velha")
	id, err := r.ID(context.Background())
	if err != nil || id != "criada" {
		t.Fatalf("após Invalidate deveria recriar, veio id=%q err=%v", id, err)
	}
	// GetFolder não é chamado de novo (o persistido não é relido).
	if c2.getCalls != 1 {
		t.Errorf("GetFolder deveria ter sido chamado só na 1ª resolução, veio %d", c2.getCalls)
	}
}

func TestFolderResolver_InvalidateComConfigNaoRecria(t *testing.T) {
	c := &fakeFolderClient{}
	s := &fakeFolderStore{}
	r := NewFolderResolver("config-folder", c, s)

	_, _ = r.ID(context.Background())
	r.Invalidate("config-folder")
	id, err := r.ID(context.Background())
	if err != nil || id != "config-folder" {
		t.Fatalf("config + Invalidate: ainda deve devolver o config, veio id=%q err=%v", id, err)
	}
	if c.createCalls != 0 || c.findCalls != 0 {
		t.Error("config + Invalidate não deve tocar o client")
	}
}
