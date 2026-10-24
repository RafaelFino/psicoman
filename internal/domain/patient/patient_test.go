package patient

import "testing"

func validPatient() *Patient {
	return &Patient{
		Name:  "Maria Silva",
		Phone: "5511912345678",
		Email: "maria@example.com",
	}
}

func TestValidate_Feliz(t *testing.T) {
	p := validPatient()
	if err := p.Validate(); err != nil {
		t.Fatalf("esperava válido, veio: %v", err)
	}
}

func TestValidate_Bordas(t *testing.T) {
	cases := map[string]func(*Patient){
		"nome vazio":      func(p *Patient) { p.Name = "  " },
		"telefone vazio":  func(p *Patient) { p.Phone = "" },
		"telefone curto":  func(p *Patient) { p.Phone = "123" },
		"email vazio":     func(p *Patient) { p.Email = "" },
		"email invalido":  func(p *Patient) { p.Email = "sem-arroba" },
		"cpf invalido":    func(p *Patient) { p.CPF = "12345678900" },
		"status invalido": func(p *Patient) { p.ApprovalStatus = "qualquer" },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			p := validPatient()
			mut(p)
			if err := p.Validate(); err == nil {
				t.Fatalf("esperava erro para %q", name)
			}
		})
	}
}

func TestValidate_CPFValidoAceito(t *testing.T) {
	p := validPatient()
	p.CPF = "52998224725" // CPF válido conhecido
	if err := p.Validate(); err != nil {
		t.Fatalf("CPF válido deveria passar: %v", err)
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"11999990001":      "5511999990001",
		"(11) 99999-0001":  "5511999990001",
		"5511999990001":    "5511999990001",
		"+55 11 3333-4444": "551133334444",
		"123":              "123",
		"":                 "",
	}
	for in, want := range cases {
		if got := NormalizePhone(in); got != want {
			t.Errorf("NormalizePhone(%q) = %q, quer %q", in, got, want)
		}
	}
}

func TestValidCPF(t *testing.T) {
	cases := map[string]bool{
		"529.982.247-25": true,
		"52998224725":    true,
		"52998224724":    false, // dígito verificador errado
		"11111111111":    false, // sequência repetida
		"123":            false, // comprimento errado
	}
	for in, want := range cases {
		if got := validCPF(in); got != want {
			t.Errorf("validCPF(%q) = %v, quer %v", in, got, want)
		}
	}
}

func TestCanTransitionApproval(t *testing.T) {
	cases := []struct {
		cur, next string
		want      bool
	}{
		{PatientPending, PatientApproved, true},
		{PatientPending, PatientRejected, true},
		{PatientApproved, PatientRejected, false},
		{PatientRejected, PatientApproved, false},
		{PatientPending, "qualquer", false},
	}
	for _, c := range cases {
		p := &Patient{ApprovalStatus: c.cur}
		if got := p.CanTransitionApproval(c.next); got != c.want {
			t.Errorf("%s->%s = %v, quer %v", c.cur, c.next, got, c.want)
		}
	}
}

func TestCanIssueReceipt(t *testing.T) {
	if (&Patient{CPF: "52998224725"}).CanIssueReceipt() != true {
		t.Error("com CPF deveria poder emitir recibo")
	}
	if (&Patient{}).CanIssueReceipt() != false {
		t.Error("sem CPF não deveria poder emitir recibo")
	}
}

func TestOriginValidate(t *testing.T) {
	if err := (&Origin{Name: "Doctoralia"}).Validate(); err != nil {
		t.Fatalf("origem válida: %v", err)
	}
	if err := (&Origin{Name: "  "}).Validate(); err == nil {
		t.Fatal("origem sem nome deveria falhar")
	}
}
