package e2e

import (
	"net/http"
	"testing"
	"time"
)

// createPatient cria um paciente e devolve o id (helper para outros testes).
func (e *Env) createPatient(t *testing.T, email string) string {
	t.Helper()
	resp := e.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Paciente " + email, "phone": "1199", "email": email,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar paciente status = %d", resp.StatusCode)
	}
	env := DecodeEnvelope(t, resp)
	var p struct {
		ID string `json:"id"`
	}
	env.DataAs(t, &p)
	return p.ID
}

func TestSessionLifecycle(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "sessao@example.com")
	start := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(25 * time.Hour).Format(time.RFC3339)

	// Cria como "solicitada".
	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start, "ends_at": end, "status": "solicitada",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, quer 201", resp.StatusCode)
	}
	ce := DecodeEnvelope(t, resp)
	var s struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	ce.DataAs(t, &s)
	if s.Status != "solicitada" {
		t.Errorf("status inicial = %q, quer solicitada", s.Status)
	}

	// Agenda.
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/schedule", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("schedule status = %d, quer 200", resp.StatusCode)
	}
	se := DecodeEnvelope(t, resp)
	var sched struct {
		Status string `json:"status"`
	}
	se.DataAs(t, &sched)
	if sched.Status != "agendada" {
		t.Errorf("status = %q, quer agendada", sched.Status)
	}

	// Finaliza com cobrança e custo.
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{
		"bill": true, "consider_cost": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("finish status = %d, quer 200", resp.StatusCode)
	}
	fe := DecodeEnvelope(t, resp)
	var fin struct {
		Status       string `json:"status"`
		Bill         bool   `json:"bill"`
		ConsiderCost bool   `json:"consider_cost"`
	}
	fe.DataAs(t, &fin)
	if fin.Status != "realizada" || !fin.Bill || !fin.ConsiderCost {
		t.Errorf("finalização inconsistente: %+v", fin)
	}

	// Finalizar de novo → 422 (já terminal).
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{"bill": false})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("re-finish status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSessionCancelAndNoShow(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "cancel@example.com")
	start := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(49 * time.Hour).Format(time.RFC3339)

	mk := func() string {
		resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
			"patient_id": pid, "modality": "presencial",
			"starts_at": start, "ends_at": end, "status": "agendada",
		})
		e := DecodeEnvelope(t, resp)
		var s struct {
			ID string `json:"id"`
		}
		e.DataAs(t, &s)
		return s.ID
	}

	// Cancelar a partir de agendada.
	id1 := mk()
	resp := env.AdminPOST(t, "/v1/admin/sessions/"+id1+"/cancel", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("cancel status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Falta a partir de agendada.
	id2 := mk()
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+id2+"/no-show", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("no-show status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSessionInvalidTransition(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "invalid@example.com")
	start := time.Now().Add(72 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(73 * time.Hour).Format(time.RFC3339)

	// Cria como solicitada e tenta finalizar direto → 422.
	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start, "ends_at": end, "status": "solicitada",
	})
	e := DecodeEnvelope(t, resp)
	var s struct {
		ID string `json:"id"`
	}
	e.DataAs(t, &s)

	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{"bill": true})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("finish de solicitada status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestSessionListByRange cobre o filtro por intervalo [from, to) usado pelas
// visões de agenda (semana/mês). Cria três sessões em dias distintos e verifica
// que o range devolve apenas as que começam dentro da janela.
func TestSessionListByRange(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "range@example.com")

	// Base: hoje às 10:00, +3 dias e +40 dias (fora de uma janela semanal/mensal curta).
	base := time.Now().Truncate(time.Hour)
	mk := func(offsetDays int) {
		start := base.AddDate(0, 0, offsetDays)
		resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
			"patient_id": pid, "modality": "online",
			"starts_at": start.Format(time.RFC3339),
			"ends_at":   start.Add(time.Hour).Format(time.RFC3339),
			"status":    "agendada",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("criar sessão (+%dd) status = %d", offsetDays, resp.StatusCode)
		}
		resp.Body.Close()
	}
	mk(0)
	mk(3)
	mk(40)

	// Janela de 7 dias a partir de ontem: deve pegar as sessões em +0 e +3, não a de +40.
	from := base.AddDate(0, 0, -1)
	to := base.AddDate(0, 0, 7)
	path := "/v1/admin/sessions?from=" + from.Format(time.RFC3339) + "&to=" + to.Format(time.RFC3339)
	resp := env.AdminGET(t, path)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list by range status = %d, quer 200", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var got []map[string]any
	e.DataAs(t, &got)
	if len(got) != 2 {
		t.Errorf("range de 7 dias devolveu %d sessões, quer 2", len(got))
	}

	// from/to incompletos → 422.
	resp = env.AdminGET(t, "/v1/admin/sessions?from="+from.Format(time.RFC3339))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("range sem 'to' status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()

	// Datas inválidas → 422.
	resp = env.AdminGET(t, "/v1/admin/sessions?from=ontem&to=amanha")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("range com datas inválidas status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestSessionUpdate cobre a edição de uma sessão (PUT /v1/admin/sessions/{id}):
// edição de sessão agendada (200, campos novos, paciente imutável), datas
// inválidas (422), sessão em estado terminal (422) e inexistente (404).
func TestSessionUpdate(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "edita-sessao@example.com")

	// Local presencial real (FK em session.location_id).
	lresp := env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name": "Consultório Edição", "modality": "presencial", "cost_period": "mensal",
	})
	if lresp.StatusCode != http.StatusCreated {
		t.Fatalf("criar local status = %d", lresp.StatusCode)
	}
	le := DecodeEnvelope(t, lresp)
	var loc struct {
		ID string `json:"id"`
	}
	le.DataAs(t, &loc)

	start := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	mk := func(status string) string {
		resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
			"patient_id": pid, "modality": "online",
			"starts_at": start.Format(time.RFC3339),
			"ends_at":   start.Add(time.Hour).Format(time.RFC3339),
			"status":    status,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("criar sessão (%s) status = %d", status, resp.StatusCode)
		}
		e := DecodeEnvelope(t, resp)
		var s struct {
			ID string `json:"id"`
		}
		e.DataAs(t, &s)
		return s.ID
	}

	// Edita uma sessão agendada: muda horário, local e modalidade.
	id := mk("agendada")
	newStart := start.Add(48 * time.Hour)
	resp := env.AdminPUT(t, "/v1/admin/sessions/"+id, map[string]any{
		"location_id": loc.ID,
		"modality":    "presencial",
		"starts_at":   newStart.Format(time.RFC3339),
		"ends_at":     newStart.Add(50 * time.Minute).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT sessão agendada status = %d, quer 200", resp.StatusCode)
	}
	ue := DecodeEnvelope(t, resp)
	var up struct {
		ID         string `json:"id"`
		PatientID  string `json:"patient_id"`
		LocationID string `json:"location_id"`
		Modality   string `json:"modality"`
		StartsAt   string `json:"starts_at"`
	}
	ue.DataAs(t, &up)
	if up.Modality != "presencial" {
		t.Errorf("modalidade = %q, quer presencial", up.Modality)
	}
	if up.LocationID != loc.ID {
		t.Errorf("local = %q, quer %q", up.LocationID, loc.ID)
	}
	if up.PatientID != pid {
		t.Errorf("paciente mudou: %q != %q", up.PatientID, pid)
	}
	gotStart, _ := time.Parse(time.RFC3339, up.StartsAt)
	if !gotStart.Equal(newStart) {
		t.Errorf("starts_at = %v, quer %v", gotStart, newStart)
	}

	// Datas inválidas → 422.
	resp = env.AdminPUT(t, "/v1/admin/sessions/"+id, map[string]any{
		"modality": "online", "starts_at": "ontem", "ends_at": "amanha",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("PUT datas inválidas status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()

	// Sessão inexistente → 404.
	resp = env.AdminPUT(t, "/v1/admin/sessions/nao-existe", map[string]any{
		"modality":  "online",
		"starts_at": newStart.Format(time.RFC3339),
		"ends_at":   newStart.Add(time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("PUT inexistente status = %d, quer 404", resp.StatusCode)
	}
	resp.Body.Close()

	// Sessão em estado terminal (cancelada) → 422.
	idTerm := mk("agendada")
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+idTerm+"/cancel", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancelar status = %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp = env.AdminPUT(t, "/v1/admin/sessions/"+idTerm, map[string]any{
		"modality":  "online",
		"starts_at": newStart.Format(time.RFC3339),
		"ends_at":   newStart.Add(time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("PUT sessão terminal status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}
