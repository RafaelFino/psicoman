package service

import "errors"

// Erros sentinela compartilhados entre serviços. A camada de API os traduz
// para códigos HTTP e mensagens PT-BR.
var (
	// ErrNotFound: entidade não encontrada.
	ErrNotFound = errors.New("não encontrado")
	// ErrConflict: violação de unicidade de negócio (email/cpf duplicado, etc.).
	ErrConflict = errors.New("conflito")
	// ErrValidation: regra de validação de negócio violada.
	ErrValidation = errors.New("validação")
)

// ValidationError embrulha uma mensagem PT-BR de validação.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }
func (e *ValidationError) Unwrap() error { return ErrValidation }

// NewValidation cria um erro de validação com mensagem PT-BR.
func NewValidation(msg string) error { return &ValidationError{Msg: msg} }

// ConflictError embrulha uma mensagem PT-BR de conflito.
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }
func (e *ConflictError) Unwrap() error { return ErrConflict }

// NewConflict cria um erro de conflito com mensagem PT-BR.
func NewConflict(msg string) error { return &ConflictError{Msg: msg} }
