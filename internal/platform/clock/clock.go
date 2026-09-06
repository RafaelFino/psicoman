// Package clock centraliza o acesso ao tempo no fuso America/Sao_Paulo.
//
// Regra do projeto (psicoman-golang.md): nunca usar time.Now() puro; todo tempo
// de domínio é gerado por este helper, garantindo o fuso fixo e permitindo
// injeção de relógio determinístico nos testes.
package clock

import (
	"sync"
	"time"
)

const locationName = "America/Sao_Paulo"

var (
	loc     *time.Location
	locOnce sync.Once
)

// Location devolve a location fixa America/Sao_Paulo.
// Faz fallback para um fuso fixo -03:00 caso a tzdata não esteja disponível.
func Location() *time.Location {
	locOnce.Do(func() {
		l, err := time.LoadLocation(locationName)
		if err != nil {
			l = time.FixedZone("-03", -3*60*60)
		}
		loc = l
	})
	return loc
}

// Clock abstrai a fonte de tempo, permitindo fakes em teste.
type Clock interface {
	Now() time.Time
}

// System é o relógio real, sempre no fuso America/Sao_Paulo.
type System struct{}

// Now devolve o instante atual no fuso do projeto.
func (System) Now() time.Time { return time.Now().In(Location()) }

// Now é o atalho para o relógio do sistema no fuso do projeto.
func Now() time.Time { return System{}.Now() }

// Format serializa um instante em ISO-8601 (RFC3339) no fuso do projeto.
func Format(t time.Time) string {
	return t.In(Location()).Format(time.RFC3339)
}

// Parse interpreta uma string ISO-8601 (RFC3339) no fuso do projeto.
func Parse(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.In(Location()), nil
}

// Fixed é um Clock determinístico para testes.
type Fixed struct{ T time.Time }

// Now devolve sempre o mesmo instante configurado.
func (f Fixed) Now() time.Time { return f.T.In(Location()) }
