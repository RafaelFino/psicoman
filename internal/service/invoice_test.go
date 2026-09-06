package service

import "testing"

func TestFormatBRL(t *testing.T) {
	cases := map[int64]string{
		0:       "R$ 0,00",
		5:       "R$ 0,05",
		150:     "R$ 1,50",
		15000:   "R$ 150,00",
		123456:  "R$ 1.234,56",
		1000000: "R$ 10.000,00",
		-2500:   "-R$ 25,00",
	}
	for cents, want := range cases {
		if got := formatBRL(cents); got != want {
			t.Errorf("formatBRL(%d) = %q, quer %q", cents, got, want)
		}
	}
}
