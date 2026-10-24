package scheduling

// Modalidade de atendimento. Dona do context scheduling (usada por session e
// location); costs/outros referenciam via estas constantes.
const (
	ModalityPresencial = "presencial"
	ModalityOnline     = "online"
)

// ValidModality indica se a modalidade é válida.
func ValidModality(m string) bool {
	return m == ModalityPresencial || m == ModalityOnline
}
