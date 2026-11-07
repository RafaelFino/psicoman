package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/RafaelFino/psicoman/internal/api/service"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/drivesync"
)

// idCipher é uma cifra identidade só para o round-trip do readiness.
type idCipher struct{}

func (idCipher) Seal(_ context.Context, p string) (string, error) { return p, nil }
func (idCipher) Open(_ context.Context, e string) (string, error) { return e, nil }

// semTokenRepo nunca tem token (Google "não autorizado").
type semTokenRepo struct{}

func (semTokenRepo) Load(context.Context) (*domain.GoogleToken, error) {
	return nil, app.ErrNotFound
}
func (semTokenRepo) Save(context.Context, *domain.GoogleToken) error { return nil }

// readyzBody chama o handler readyz com um Services montado à mão.
func readyzBody(t *testing.T, st *drivesync.Status) string {
	t.Helper()
	a := newTestApp(t)
	svc := &service.Services{
		Cipher:      idCipher{},
		GoogleAuth:  app.NewGoogleAuthService(semTokenRepo{}),
		DriveStatus: st,
	}
	rec := httptest.NewRecorder()
	e := &core.RequestEvent{App: a}
	e.Event = router.Event{Response: rec, Request: httptest.NewRequest(http.MethodGet, "/v1/readyz", nil)}
	if err := readyz(svc)(e); err != nil {
		t.Fatalf("readyz: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz: quero 200, veio %d (%s)", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// TestReadyz_DriveStatusNilOmiteChave: com Services montado à mão (sem
// DriveStatus), o readyz omite checks.drive_dr em vez de panicar.
func TestReadyz_DriveStatusNilOmiteChave(t *testing.T) {
	body := readyzBody(t, nil)
	if strings.Contains(body, "drive_dr") {
		t.Errorf("sem DriveStatus a chave drive_dr deve ser omitida: %s", body)
	}
}

// TestReadyz_DriveStatusTresEstados: desativado, ok e degradado aparecem em
// checks.drive_dr sem alterar ready.
func TestReadyz_DriveStatusTresEstados(t *testing.T) {
	desativado := &drivesync.Status{}

	ok := &drivesync.Status{}
	ok.SetConfigured(true)

	degradado := &drivesync.Status{}
	degradado.SetConfigured(true)
	degradado.Set("erro")

	casos := map[string]*drivesync.Status{"desativado": desativado, "ok": ok, "degradado": degradado}
	for want, st := range casos {
		body := readyzBody(t, st)
		if !strings.Contains(body, `"drive_dr":"`+want+`"`) {
			t.Errorf("quero drive_dr=%s, corpo: %s", want, body)
		}
		if !strings.Contains(body, `"ready":true`) {
			t.Errorf("drive_dr=%s não pode alterar ready: %s", want, body)
		}
	}
}
