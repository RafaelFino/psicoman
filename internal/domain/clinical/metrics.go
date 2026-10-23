package clinical

import "github.com/RafaelFino/psicoman/internal/domain/scheduling"

// SessionAggregate é o resultado do motor de agregação: contagem por categoria
// (scheduling.SessionCategory) e o faturado das realizadas no período.
// Reutilizado pelas métricas do paciente (R6.4–R6.5) e pelo relatório por local
// (R7).
type SessionAggregate struct {
	Counts       map[string]int `json:"counts"`              // categoria -> nº de sessões
	Total        int            `json:"total"`               // total de sessões no recorte
	FaturadoReal int64          `json:"faturado_realizadas"` // soma dos débitos das realizadas (centavos)
}

// AggregateSessions é o motor puro: dado um conjunto de sessões e um mapa de
// faturado por sessão (centavos), devolve a contagem por categoria e o total
// faturado das realizadas. Sessões sem categoria conhecida caem em
// CategoryDesconhecida. Não faz I/O; é a base testável de R6.4/R7.
func AggregateSessions(sessions []*scheduling.Session, faturadoPorSessao map[string]int64) SessionAggregate {
	agg := SessionAggregate{Counts: map[string]int{}}
	for _, s := range sessions {
		cat := scheduling.SessionCategory(s)
		agg.Counts[cat]++
		agg.Total++
		if cat == scheduling.CategoryRealizada {
			agg.FaturadoReal += faturadoPorSessao[s.ID]
		}
	}
	return agg
}

// PatientMetrics agrega as sessões do paciente com contagens por categoria,
// taxas zero-safe e o faturado das realizadas (R6.4–R6.5).
type PatientMetrics struct {
	SessionAggregate
	TaxaComparecimento float64 `json:"taxa_comparecimento"` // fração 0..1
	TaxaCancelamento   float64 `json:"taxa_cancelamento"`   // fração 0..1
	From               string  `json:"from,omitempty"`
	To                 string  `json:"to,omitempty"`
}

// AttendanceRate = realizadas / (realizadas + faltas + canceladas_paciente);
// zero-safe (denominador 0 ⇒ 0).
func AttendanceRate(counts map[string]int) float64 {
	num := counts[scheduling.CategoryRealizada]
	den := counts[scheduling.CategoryRealizada] + counts[scheduling.CategoryFalta] + counts[scheduling.CategoryCanceladaPaciente]
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

// CancelRate = (canceladas_paciente + canceladas_terapeuta + canceladas_sem_autor)
// / total; zero-safe (total 0 ⇒ 0).
func CancelRate(counts map[string]int, total int) float64 {
	if total == 0 {
		return 0
	}
	canc := counts[scheduling.CategoryCanceladaPaciente] + counts[scheduling.CategoryCanceladaTerapeuta] + counts[scheduling.CategoryCanceladaSemAutor]
	return float64(canc) / float64(total)
}
