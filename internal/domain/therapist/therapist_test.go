package therapist

import "testing"

func TestProfileValidate(t *testing.T) {
	if err := (&TherapistProfile{Name: "Dra. Ana"}).Validate(); err != nil {
		t.Fatalf("perfil válido: %v", err)
	}
	if err := (&TherapistProfile{Name: " "}).Validate(); err == nil {
		t.Fatal("perfil sem nome deveria falhar")
	}
}

func TestPublicView(t *testing.T) {
	p := &TherapistProfile{
		Name:        "Dra. Ana",
		CRP:         "06/12345",
		Email:       "ana@example.com",
		Bio:         "Psicóloga clínica.",
		PhotoFileID: "ged-1",
		Contacts:    map[string]string{"telefone": "11999990001"},
	}
	v := p.PublicView()
	if v["name"] != "Dra. Ana" || v["bio"] != "Psicóloga clínica." || v["photo_id"] != "ged-1" {
		t.Fatalf("public view com campos errados: %v", v)
	}
	// Não deve expor CRP/email/contatos.
	for _, k := range []string{"crp", "email", "contacts", "contatos"} {
		if _, ok := v[k]; ok {
			t.Errorf("public view não deveria conter %q", k)
		}
	}
}

func TestPlatformLinkValidate(t *testing.T) {
	ok := &TherapistPlatformLink{Label: "Doctoralia", URL: "https://x"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("link válido: %v", err)
	}
	if err := (&TherapistPlatformLink{URL: "https://x"}).Validate(); err == nil {
		t.Fatal("link sem label deveria falhar")
	}
	if err := (&TherapistPlatformLink{Label: "x"}).Validate(); err == nil {
		t.Fatal("link sem url deveria falhar")
	}
}
