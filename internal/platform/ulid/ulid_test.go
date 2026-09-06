package ulid

import (
	"sort"
	"testing"
	"time"
)

func TestNewUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 10000; i++ {
		id := New()
		if seen[id] {
			t.Fatalf("ULID duplicado: %s", id)
		}
		seen[id] = true
		if !Valid(id) {
			t.Fatalf("ULID inválido gerado: %s", id)
		}
	}
}

func TestOrdering(t *testing.T) {
	// IDs gerados em timestamps crescentes devem ordenar lexicograficamente.
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var ids []string
	for i := 0; i < 100; i++ {
		ids = append(ids, NewAt(base.Add(time.Duration(i)*time.Millisecond)))
	}
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)
	for i := range ids {
		if ids[i] != sorted[i] {
			t.Fatalf("ordenação quebrada na posição %d", i)
		}
	}
}

func TestValid(t *testing.T) {
	if Valid("não-é-ulid") {
		t.Error("string inválida aceita como ULID")
	}
	if !Valid(New()) {
		t.Error("ULID gerado rejeitado")
	}
}
