package e2e

import (
	"net/http"
	"testing"
)

func TestPatientCRUD(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Criar.
	resp := env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name":  "Maria Silva",
		"phone": "11999998888",
		"email": "maria@example.com",
		"cpf":   "529.982.247-25",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, quer 201", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var created struct {
		ID              string `json:"id"`
		CanIssueReceipt bool   `json:"can_issue_receipt"`
		CPF             string `json:"cpf"`
	}
	e.DataAs(t, &created)
	if created.ID == "" {
		t.Fatal("paciente criado sem id")
	}
	if !created.CanIssueReceipt {
		t.Error("com CPF deveria poder emitir recibo")
	}
	if created.CPF != "52998224725" {
		t.Errorf("cpf normalizado = %q", created.CPF)
	}

	// Buscar.
	resp = env.AdminGET(t, "/v1/admin/patients/"+created.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Listar.
	resp = env.AdminGET(t, "/v1/admin/patients")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, quer 200", resp.StatusCode)
	}
	le := DecodeEnvelope(t, resp)
	var list []map[string]any
	le.DataAs(t, &list)
	if len(list) != 1 {
		t.Errorf("lista com %d pacientes, quer 1", len(list))
	}

	// Atualizar.
	resp = env.AdminPUT(t, "/v1/admin/patients/"+created.ID, map[string]any{
		"name":  "Maria Silva Souza",
		"phone": "11999990000",
		"email": "maria@example.com",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Remover.
	resp = env.AdminDELETE(t, "/v1/admin/patients/"+created.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Após remover, get deve dar 404.
	resp = env.AdminGET(t, "/v1/admin/patients/"+created.ID)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get pós-delete status = %d, quer 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPatientValidations(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Sem email → 422.
	resp := env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name":  "Sem Email",
		"phone": "11999998888",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("sem email status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()

	// CPF inválido → 422.
	resp = env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name":  "CPF Ruim",
		"phone": "11999998888",
		"email": "cpfruim@example.com",
		"cpf":   "12345678900",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("cpf inválido status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPatientEmailUnique(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	body := map[string]any{"name": "A", "phone": "11999990001", "email": "dup@example.com"}
	resp := env.AdminPOST(t, "/v1/admin/patients", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("primeiro create status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Segundo com mesmo email → 409.
	resp = env.AdminPOST(t, "/v1/admin/patients", body)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("email duplicado status = %d, quer 409", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPatientCPFUnique(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	resp := env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "A", "phone": "11999990001", "email": "a@example.com", "cpf": "529.982.247-25",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("primeiro create status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Outro paciente, mesmo CPF → 409.
	resp = env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "B", "phone": "11988880002", "email": "b@example.com", "cpf": "52998224725",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("cpf duplicado status = %d, quer 409", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestPatientPhoneNormalized cobre R3.1: o telefone é persistido em forma
// canônica (só dígitos, com DDI 55) tanto no cadastro pelo admin quanto no
// auto-cadastro pelo portal; números implausíveis são rejeitados (422).
func TestPatientPhoneNormalized(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Admin: telefone mascarado -> canônico na resposta.
	resp := env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Fone Admin", "phone": "(11) 99999-0001", "email": "fone-admin@example.com",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, quer 201", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var p struct {
		Phone string `json:"phone"`
	}
	e.DataAs(t, &p)
	if p.Phone != "5511999990001" {
		t.Errorf("telefone admin = %q, quer 5511999990001 (canônico)", p.Phone)
	}

	// Telefone implausível -> 422.
	resp = env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Fone Ruim", "phone": "123", "email": "fone-ruim@example.com",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("telefone implausível status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestPortalRegisterPhoneNormalized cobre R3.1 no portal: o auto-cadastro
// persiste o telefone canônico, visível pelo admin.
func TestPortalRegisterPhoneNormalized(t *testing.T) {
	adminEnv, portalEnv := StartAdminAndPortal(t)
	defer adminEnv.Stop()
	defer portalEnv.Stop()

	pc := portalEnv.NewPortalClient(t)
	resp := pc.PUT(t, "/v1/portal/register", map[string]any{
		"credential": "fone-portal@example.com", "name": "Fone Portal", "phone": "11 98888-7777",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Admin vê o telefone canônico na fila de pendentes.
	le := DecodeEnvelope(t, adminEnv.AdminGET(t, "/v1/admin/patients/pending"))
	var pendings []struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
	}
	le.DataAs(t, &pendings)
	found := false
	for _, pp := range pendings {
		if pp.Email == "fone-portal@example.com" {
			found = true
			if pp.Phone != "5511988887777" {
				t.Errorf("telefone portal = %q, quer 5511988887777 (canônico)", pp.Phone)
			}
		}
	}
	if !found {
		t.Fatal("paciente do portal não apareceu na fila de pendentes")
	}
}
