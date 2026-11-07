package drivesync

import (
	"reflect"
	"testing"
)

func snap(ids ...string) []File {
	var out []File
	for _, id := range ids {
		out = append(out, File{ID: id, Name: id + ".enc"})
	}
	return out
}

func TestSelectToTrash(t *testing.T) {
	casos := []struct {
		nome  string
		files []File
		keep  int
		novo  string
		want  []string
	}{
		{"menos que N", snap("n", "a"), 14, "n", nil},
		{"exatamente N", snap("n", "a", "b"), 3, "n", nil},
		{"mais que N", snap("n", "a", "b", "c", "d"), 3, "n", []string{"c", "d"}},
		{"recém-enviado fora da listagem", snap("a", "b", "c"), 3, "n", []string{"c"}},
		{"keep=1 só o novo", snap("n", "a", "b"), 1, "n", []string{"a", "b"}},
		{"keep=0 não seleciona", snap("n", "a", "b"), 0, "n", nil},
		{
			"ignora sem .enc",
			[]File{{ID: "n", Name: "n.enc"}, {ID: "x", Name: "foto.jpg"}, {ID: "a", Name: "a.enc"}, {ID: "b", Name: "b.enc"}},
			2, "n", []string{"b"},
		},
		{"lista vazia", nil, 3, "n", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := selectToTrash(c.files, c.keep, c.novo)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("quero %v, veio %v", c.want, got)
			}
		})
	}
}

func TestSelectToTrash_NuncaApagaORecemEnviado(t *testing.T) {
	// Mesmo no fim da lista (ordem inesperada), o recém-enviado fica.
	got := selectToTrash(snap("a", "b", "n"), 2, "n")
	for _, id := range got {
		if id == "n" {
			t.Fatalf("o recém-enviado não pode ir para a lixeira: %v", got)
		}
	}
}

func TestCountSnapshots(t *testing.T) {
	files := []File{{Name: "a.enc"}, {Name: "x.txt"}, {Name: "b.enc"}}
	if got := countSnapshots(files); got != 2 {
		t.Errorf("quero 2, veio %d", got)
	}
}
