package domain

import "time"

// GEDFile é o metadado de um arquivo no GED (Gestão Eletrônica de Documentos),
// segregado por paciente. A integridade é garantida por SHA-256; a dedup é por
// hash dentro do escopo do paciente (requirements §3.6, §4.2).
type GEDFile struct {
	ID        string
	PatientID string // vazio = arquivo do perfil do terapeuta
	SessionID string
	DebtID    string
	PaymentID string
	RelPath   string // caminho relativo dentro do <ged_root>
	MIME      string
	Size      int64
	SHA256    string
	CreatedAt time.Time
}
