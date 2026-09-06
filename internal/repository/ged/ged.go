// Package ged implementa o armazenamento físico do GED em disco, segregado por
// paciente: <ged_root>/<scope>/<file_ulid> (docs/architecture.md §4.2).
//
// A escrita calcula o SHA-256; a leitura valida o hash antes de servir
// (integridade). Metadados ficam no SQLite (ged_file); este pacote cuida só do
// arquivo em si.
package ged

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store é o repositório físico de arquivos do GED.
type Store struct {
	root string
}

// ErrIntegrity indica que o hash lido não bate com o esperado (arquivo corrompido).
var ErrIntegrity = errors.New("falha de integridade: hash do arquivo não confere")

// NewStore cria o store enraizado em root (criado se não existir).
func NewStore(root string) (*Store, error) {
	if root == "" {
		return nil, fmt.Errorf("ged: root é obrigatório")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("ged: criando root %q: %w", root, err)
	}
	return &Store{root: root}, nil
}

// scopeDir devolve o diretório de um escopo (patient_id ou "_therapist").
func scopeDir(scope string) string {
	if scope == "" {
		return "_therapist"
	}
	return scope
}

// Write grava o conteúdo em <root>/<scope>/<fileID>, devolvendo o caminho
// relativo, o SHA-256 e o tamanho. O caller persiste os metadados.
func (s *Store) Write(scope, fileID string, content io.Reader) (relPath, sha string, size int64, err error) {
	dir := filepath.Join(s.root, scopeDir(scope))
	if err = os.MkdirAll(dir, 0o750); err != nil {
		return "", "", 0, fmt.Errorf("ged: criando dir do escopo: %w", err)
	}
	abs := filepath.Join(dir, fileID)
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return "", "", 0, fmt.Errorf("ged: abrindo arquivo: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), content)
	if err != nil {
		_ = os.Remove(abs)
		return "", "", 0, fmt.Errorf("ged: escrevendo arquivo: %w", err)
	}
	relPath = filepath.Join(scopeDir(scope), fileID)
	sha = hex.EncodeToString(h.Sum(nil))
	return relPath, sha, n, nil
}

// Read abre um arquivo pelo caminho relativo, validando o hash esperado.
// Devolve o conteúdo já lido em memória (arquivos do GED são pequenos).
func (s *Store) Read(relPath, expectedSHA string) ([]byte, error) {
	abs := filepath.Join(s.root, relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("ged: lendo arquivo: %w", err)
	}
	if expectedSHA != "" {
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != expectedSHA {
			return nil, ErrIntegrity
		}
	}
	return data, nil
}

// ReadRaw lê o conteúdo bruto pelo caminho relativo, sem validar hash (backup).
func (s *Store) ReadRaw(relPath string) ([]byte, error) {
	abs := filepath.Join(s.root, relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("ged: lendo arquivo: %w", err)
	}
	return data, nil
}

// Delete remove o arquivo físico pelo caminho relativo.
func (s *Store) Delete(relPath string) error {
	abs := filepath.Join(s.root, relPath)
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ged: removendo arquivo: %w", err)
	}
	return nil
}

// Root devolve a raiz do GED.
func (s *Store) Root() string { return s.root }
