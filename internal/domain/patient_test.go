package domain

import "testing"

func TestPatientValidate(t *testing.T) {
	base := func() *Patient {
		// Telefone já canônico (DDI 55 + DDD + número), como o serviço persiste.
		return &Patient{Name: "Maria", Phone: "5511999998888", Email: "maria@example.com"}
	}
	t.Run("telefone sem DDD/implausível", func(t *testing.T) {
		p := base()
		p.Phone = "123"
		if err := p.Validate(); err == nil {
			t.Error("esperava erro por telefone implausível")
		}
	})
	t.Run("válido", func(t *testing.T) {
		if err := base().Validate(); err != nil {
			t.Errorf("esperava válido, veio erro: %v", err)
		}
	})
	t.Run("sem nome", func(t *testing.T) {
		p := base()
		p.Name = " "
		if err := p.Validate(); err == nil {
			t.Error("esperava erro por nome vazio")
		}
	})
	t.Run("email inválido", func(t *testing.T) {
		p := base()
		p.Email = "nao-eh-email"
		if err := p.Validate(); err == nil {
			t.Error("esperava erro por email inválido")
		}
	})
	t.Run("cpf inválido", func(t *testing.T) {
		p := base()
		p.CPF = "12345678900"
		if err := p.Validate(); err == nil {
			t.Error("esperava erro por CPF inválido")
		}
	})
	t.Run("cpf válido", func(t *testing.T) {
		p := base()
		p.CPF = "52998224725" // CPF válido de teste
		if err := p.Validate(); err != nil {
			t.Errorf("esperava CPF válido, veio erro: %v", err)
		}
	})
}

func TestCanIssueReceipt(t *testing.T) {
	p := &Patient{}
	if p.CanIssueReceipt() {
		t.Error("sem CPF não deveria poder emitir recibo")
	}
	p.CPF = "52998224725"
	if !p.CanIssueReceipt() {
		t.Error("com CPF deveria poder emitir recibo")
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"11999990001", "5511999990001"},     // celular 11 díg -> +55
		{"(11) 99999-0001", "5511999990001"}, // máscara
		{"5511999990001", "5511999990001"},   // já com DDI (celular)
		{"+55 11 3333-4444", "551133334444"}, // fixo 10 díg + DDI
		{"1133334444", "551133334444"},       // fixo nacional 10 díg -> +55
		{"551133334444", "551133334444"},     // fixo já com DDI
		{"", ""},                             // vazio
		{"123", "123"},                       // ambíguo (rejeitado no Validate)
	}
	for _, c := range cases {
		if got := NormalizePhone(c.in); got != c.want {
			t.Errorf("NormalizePhone(%q) = %q, quer %q", c.in, got, c.want)
		}
	}
}

func TestNormalizePhoneThenValidate(t *testing.T) {
	t.Run("nacional vira canônico e valida", func(t *testing.T) {
		p := &Patient{Name: "N", Email: "n@x.com", Phone: NormalizePhone("(11) 99999-0001")}
		if p.Phone != "5511999990001" {
			t.Fatalf("canônico = %q", p.Phone)
		}
		if err := p.Validate(); err != nil {
			t.Errorf("telefone canônico deveria validar: %v", err)
		}
	})
	t.Run("implausível é rejeitado", func(t *testing.T) {
		p := &Patient{Name: "N", Email: "n@x.com", Phone: NormalizePhone("123")}
		if err := p.Validate(); err == nil {
			t.Error("telefone implausível deveria falhar")
		}
	})
}

func TestNormalizeCPF(t *testing.T) {
	if got := NormalizeCPF("529.982.247-25"); got != "52998224725" {
		t.Errorf("NormalizeCPF = %q", got)
	}
}

func TestValidCPFRejectsRepeated(t *testing.T) {
	if validCPF("11111111111") {
		t.Error("CPF com dígitos repetidos deveria ser inválido")
	}
}

func TestPatientApprovalTransition(t *testing.T) {
	t.Run("pendente pode aprovar", func(t *testing.T) {
		p := &Patient{ApprovalStatus: PatientPending}
		if !p.CanTransitionApproval(PatientApproved) {
			t.Error("pendente deveria poder virar aprovado")
		}
		if !p.CanTransitionApproval(PatientRejected) {
			t.Error("pendente deveria poder virar rejeitado")
		}
	})
	t.Run("aprovado não retrocede", func(t *testing.T) {
		p := &Patient{ApprovalStatus: PatientApproved}
		if p.CanTransitionApproval(PatientRejected) {
			t.Error("aprovado não deveria virar rejeitado")
		}
		if p.CanTransitionApproval(PatientPending) {
			t.Error("aprovado não deveria voltar a pendente")
		}
	})
	t.Run("rejeitado não transita", func(t *testing.T) {
		p := &Patient{ApprovalStatus: PatientRejected}
		if p.CanTransitionApproval(PatientApproved) {
			t.Error("rejeitado não deveria virar aprovado")
		}
	})
	t.Run("estado inválido nunca é destino", func(t *testing.T) {
		p := &Patient{ApprovalStatus: PatientPending}
		if p.CanTransitionApproval("qualquer") {
			t.Error("destino inválido não deveria ser aceito")
		}
	})
}

func TestPatientIsApproved(t *testing.T) {
	if (&Patient{ApprovalStatus: PatientApproved}).IsApproved() != true {
		t.Error("aprovado deveria ser IsApproved")
	}
	if (&Patient{ApprovalStatus: PatientPending}).IsApproved() != false {
		t.Error("pendente não deveria ser IsApproved")
	}
}

func TestPatientValidateApprovalStatus(t *testing.T) {
	p := &Patient{Name: "Maria", Phone: "5511999998888", Email: "maria@example.com"}
	p.ApprovalStatus = "invalido"
	if err := p.Validate(); err == nil {
		t.Error("estado de aprovação inválido deveria falhar na validação")
	}
	p.ApprovalStatus = PatientPending
	if err := p.Validate(); err != nil {
		t.Errorf("estado pendente deveria ser válido: %v", err)
	}
}
