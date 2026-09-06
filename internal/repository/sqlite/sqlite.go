// Package sqlite fornece a conexão SQLite compartilhada pelos dois binários,
// em modo WAL, com os PRAGMAs exigidos pelo projeto (psicoman-sqlite.md):
// journal_mode=WAL, busy_timeout, foreign_keys=ON.
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB embrulha *sql.DB com a configuração do projeto.
type DB struct {
	*sql.DB
	path string
}

// Options configura a abertura do banco.
type Options struct {
	// Path do arquivo .db. Diretórios pais são criados se necessário.
	Path string
	// BusyTimeoutMS absorve contenção pontual de escrita entre os dois processos.
	BusyTimeoutMS int
}

// Open abre (ou cria) o banco no caminho informado, aplicando os PRAGMAs.
func Open(opts Options) (*DB, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("sqlite: path é obrigatório")
	}
	if opts.BusyTimeoutMS == 0 {
		opts.BusyTimeoutMS = 5000
	}
	if dir := filepath.Dir(opts.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("sqlite: criando diretório %q: %w", dir, err)
		}
	}

	// DSN com PRAGMAs aplicados a cada conexão do pool (modernc aceita _pragma).
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(%d)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)",
		opts.Path, opts.BusyTimeoutMS)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: abrindo %q: %w", opts.Path, err)
	}

	// WAL suporta múltiplos leitores + 1 escritor; limitar conexões evita
	// "database is locked" sob contenção. Um pool pequeno é suficiente aqui.
	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("sqlite: ping %q: %w", opts.Path, err)
	}

	return &DB{DB: sqlDB, path: opts.Path}, nil
}

// Path devolve o caminho do arquivo do banco.
func (db *DB) Path() string { return db.path }

// Ping verifica a conectividade (usado no readyz).
func (db *DB) Ping() error { return db.DB.Ping() }
