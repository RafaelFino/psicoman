package e2e

import (
	"net/http"
	"testing"
)

func TestLocationCRUD(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Local presencial com custo mensal.
	resp := env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name":        "Consultório Centro",
		"address":     "Rua X, 100",
		"modality":    "presencial",
		"cost_amount": 250000,
		"cost_period": "mensal",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, quer 201", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var loc struct {
		ID string `json:"id"`
	}
	e.DataAs(t, &loc)

	// Local online.
	resp = env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name":        "Online",
		"modality":    "online",
		"cost_amount": 0,
		"cost_period": "por_sessao",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create online status = %d, quer 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Listar deve trazer 2.
	resp = env.AdminGET(t, "/v1/admin/locations")
	le := DecodeEnvelope(t, resp)
	var list []map[string]any
	le.DataAs(t, &list)
	if len(list) != 2 {
		t.Errorf("lista com %d locais, quer 2", len(list))
	}

	// Adicionar disponibilidade.
	resp = env.AdminPOST(t, "/v1/admin/locations/"+loc.ID+"/availability", map[string]any{
		"weekday":    1,
		"start_time": "09:00",
		"end_time":   "12:00",
		"capacity":   1,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("availability status = %d, quer 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Listar disponibilidade.
	resp = env.AdminGET(t, "/v1/admin/locations/"+loc.ID+"/availability")
	ae := DecodeEnvelope(t, resp)
	var avail []map[string]any
	ae.DataAs(t, &avail)
	if len(avail) != 1 {
		t.Errorf("disponibilidades = %d, quer 1", len(avail))
	}
}

func TestLocationValidations(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Modalidade inválida → 422.
	resp := env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name":        "Ruim",
		"modality":    "hibrido",
		"cost_period": "mensal",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("modalidade inválida status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()

	// Periodicidade inválida → 422.
	resp = env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name":        "Ruim2",
		"modality":    "online",
		"cost_period": "semanal",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("periodicidade inválida status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAvailabilityValidation(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	resp := env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name": "L", "modality": "online", "cost_period": "por_sessao",
	})
	e := DecodeEnvelope(t, resp)
	var loc struct {
		ID string `json:"id"`
	}
	e.DataAs(t, &loc)

	// Horário final antes do inicial → 422.
	resp = env.AdminPOST(t, "/v1/admin/locations/"+loc.ID+"/availability", map[string]any{
		"weekday": 2, "start_time": "15:00", "end_time": "10:00", "capacity": 1,
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("horário inválido status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}
