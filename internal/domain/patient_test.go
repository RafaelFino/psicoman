package domain

import "testing"

func TestPatientValidate(t *testing.T) {
	base := func() *Patient {
		return &Patient{Name: "Maria", Phone: "11999998888", Email: "maria@example.com"}
	}
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
