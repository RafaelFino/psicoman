package e2e

import (
	"net/http"
	"testing"
)

func TestTherapistProfile(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Antes de configurar, GET devolve 404.
	resp := env.AdminGET(t, "/v1/admin/profile")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("perfil inexistente status = %d, quer 404", resp.StatusCode)
	}
	resp.Body.Close()

	// Cria um local para associar.
	resp = env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name": "Consultório", "modality": "presencial", "cost_period": "mensal",
	})
	le := DecodeEnvelope(t, resp)
	var loc struct {
		ID string `json:"id"`
	}
	le.DataAs(t, &loc)

	// Salva perfil.
	resp = env.AdminPUT(t, "/v1/admin/profile", map[string]any{
		"name":         "Dra. Ana",
		"crp":          "06/12345",
		"email":        "ana@example.com",
		"contacts":     map[string]string{"telefone": "1199"},
		"bio":          "Psicóloga clínica",
		"location_ids": []string{loc.ID},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save perfil status = %d, quer 200", resp.StatusCode)
	}
	pe := DecodeEnvelope(t, resp)
	var prof struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		LocationIDs []string `json:"location_ids"`
	}
	pe.DataAs(t, &prof)
	if prof.Name != "Dra. Ana" {
		t.Errorf("nome = %q", prof.Name)
	}
	if len(prof.LocationIDs) != 1 || prof.LocationIDs[0] != loc.ID {
		t.Errorf("locais associados = %v, quer [%s]", prof.LocationIDs, loc.ID)
	}

	// GET agora devolve o perfil.
	resp = env.AdminGET(t, "/v1/admin/profile")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get perfil status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Atualiza (segundo PUT continua sendo o mesmo registro único).
	resp = env.AdminPUT(t, "/v1/admin/profile", map[string]any{
		"name": "Dra. Ana Souza",
	})
	ue := DecodeEnvelope(t, resp)
	var prof2 struct {
		ID string `json:"id"`
	}
	ue.DataAs(t, &prof2)
	if prof2.ID != prof.ID {
		t.Errorf("perfil deveria ser único: id %q != %q", prof2.ID, prof.ID)
	}
}

func TestTherapistPlatformLinks(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Precisa de perfil antes.
	env.AdminPUT(t, "/v1/admin/profile", map[string]any{"name": "Terapeuta"}).Body.Close()

	// Cria origem para referenciar.
	resp := env.AdminPOST(t, "/v1/admin/origins", map[string]any{"name": "Doctoralia"})
	oe := DecodeEnvelope(t, resp)
	var origin struct {
		ID string `json:"id"`
	}
	oe.DataAs(t, &origin)

	// Adiciona link ligado à origem.
	resp = env.AdminPOST(t, "/v1/admin/profile/links", map[string]any{
		"label": "Meu Doctoralia", "url": "https://doctoralia.com.br/ana", "origin_id": origin.ID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add link status = %d, quer 201", resp.StatusCode)
	}
	fe := DecodeEnvelope(t, resp)
	var link struct {
		ID       string `json:"id"`
		OriginID string `json:"origin_id"`
	}
	fe.DataAs(t, &link)
	if link.OriginID != origin.ID {
		t.Errorf("origin_id = %q, quer %q", link.OriginID, origin.ID)
	}

	// Lista.
	resp = env.AdminGET(t, "/v1/admin/profile/links")
	ll := DecodeEnvelope(t, resp)
	var links []map[string]any
	ll.DataAs(t, &links)
	if len(links) != 1 {
		t.Errorf("links = %d, quer 1", len(links))
	}

	// Remove.
	resp = env.AdminDELETE(t, "/v1/admin/profile/links/"+link.ID)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete link status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestTherapistPhoto(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	env.AdminPUT(t, "/v1/admin/profile", map[string]any{"name": "Terapeuta"}).Body.Close()

	resp := env.uploadFile(t, "/v1/admin/profile/photo", "foto.png", []byte("fake-png-bytes"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("photo status = %d, quer 200", resp.StatusCode)
	}
	pe := DecodeEnvelope(t, resp)
	var prof struct {
		PhotoID string `json:"photo_id"`
	}
	pe.DataAs(t, &prof)
	if prof.PhotoID == "" {
		t.Error("foto não vinculada ao perfil")
	}
}
