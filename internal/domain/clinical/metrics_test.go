package clinical

import (
	"strings"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func mustNow() time.Time {
	return time.Date(2026, 5, 15, 10, 0, 0, 0, clock.Location())
}

func TestAggregateSessions(t *testing.T) {
	sessions := []*scheduling.Session{
		{ID: "a", Status: scheduling.SessionRealizada},
		{ID: "b", Status: scheduling.SessionRealizada},
		{ID: "c", Status: scheduling.SessionFalta},
		{ID: "d", Status: scheduling.SessionCancelada, CanceledBy: scheduling.CancelByPatient},
	}
	faturado := map[string]int64{"a": 10000, "b": 15000, "c": 9999}
	agg := AggregateSessions(sessions, faturado)

	if agg.Total != 4 {
		t.Errorf("total = %d, quer 4", agg.Total)
	}
	if agg.Counts[scheduling.CategoryRealizada] != 2 {
		t.Errorf("realizadas = %d", agg.Counts[scheduling.CategoryRealizada])
	}
	// Faturado só das realizadas (não conta a falta "c").
	if agg.FaturadoReal != 25000 {
		t.Errorf("faturado realizadas = %d, quer 25000", agg.FaturadoReal)
	}

	// Conjunto vazio.
	empty := AggregateSessions(nil, nil)
	if empty.Total != 0 || len(empty.Counts) != 0 {
		t.Errorf("vazio deveria zerar, veio %+v", empty)
	}
}

func TestRatesZeroSafe(t *testing.T) {
	if AttendanceRate(map[string]int{}) != 0 {
		t.Error("taxa de comparecimento com denominador 0 deveria ser 0")
	}
	if CancelRate(map[string]int{}, 0) != 0 {
		t.Error("taxa de cancelamento com total 0 deveria ser 0")
	}
	counts := map[string]int{
		scheduling.CategoryRealizada:         3,
		scheduling.CategoryFalta:             1,
		scheduling.CategoryCanceladaPaciente: 0,
	}
	if got := AttendanceRate(counts); got != 0.75 {
		t.Errorf("taxa comparecimento = %v, quer 0.75", got)
	}
	counts2 := map[string]int{
		scheduling.CategoryCanceladaPaciente:  1,
		scheduling.CategoryCanceladaTerapeuta: 1,
	}
	if got := CancelRate(counts2, 4); got != 0.5 {
		t.Errorf("taxa cancelamento = %v, quer 0.5", got)
	}
}

// TestTimelineNaoVazaSealedText garante que o resumo de um item de nota na
// timeline NÃO contém o conteúdo clínico em claro (R4.3/R6.3): a timeline usa
// só rótulos fixos.
func TestTimelineNaoVazaSealedText(t *testing.T) {
	segredo := "paciente relatou crise grave e detalhes sensíveis"
	note := &Note{
		ID:        "n1",
		PatientID: "p1",
		SessionID: "s1",
		Content:   domain.NewSealedText(segredo),
	}
	item := NoteItem(note)
	if strings.Contains(item.Resumo, segredo) || strings.Contains(item.Titulo, segredo) {
		t.Fatalf("timeline vazou conteúdo clínico: %+v", item)
	}

	free := &Note{ID: "n2", PatientID: "p1", Content: domain.NewSealedText(segredo)}
	freeItem := NoteItem(free)
	if strings.Contains(freeItem.Resumo, segredo) {
		t.Fatalf("timeline (nota livre) vazou conteúdo clínico: %+v", freeItem)
	}
}

func TestWindowRange(t *testing.T) {
	// Normaliza janela desconhecida para "hoje".
	_, _, norm := WindowRange("xpto", mustNow())
	if norm != DayWindowHoje {
		t.Errorf("janela desconhecida deveria normalizar para hoje, veio %q", norm)
	}
	from, to, _ := WindowRange(DayWindowProximos7, mustNow())
	if to.Sub(from).Hours() != 8*24 {
		t.Errorf("proximos7 deveria abranger 8 dias, veio %v", to.Sub(from))
	}
}
