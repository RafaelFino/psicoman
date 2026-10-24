package httpx

import (
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// nowFunc é a fonte de tempo da chain (fuso do projeto). Variável de pacote para
// permitir determinismo em teste, se necessário.
var nowFunc = clock.Now

// FmtTime formata um instante em ISO-8601 (America/Sao_Paulo). Instante zero →
// "" (clock.Format emitiria 0001-01-01T...; campo opcional vazio vira string
// vazia). Reusado pelos mappers de DTO dos três atores.
func FmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return clock.Format(t)
}

// ParseDateRange lê de/ate no formato date-only YYYY-MM-DD no fuso do projeto.
// Semântica: [de 00:00:00, ate 23:59:59.999999999] em America/Sao_Paulo (ate
// INCLUSIVO — o dia final inteiro entra no intervalo). Usa ParseInLocation
// porque clock.Parse é RFC3339 e falharia com datas puras (correção H3).
func ParseDateRange(deStr, ateStr string) (de, ate time.Time, err error) {
	de, err = time.ParseInLocation("2006-01-02", deStr, clock.Location())
	if err != nil {
		return
	}
	ate, err = time.ParseInLocation("2006-01-02", ateStr, clock.Location())
	if err != nil {
		return
	}
	ate = ate.Add(24*time.Hour - time.Nanosecond)
	return
}
