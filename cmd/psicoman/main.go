// Command psicoman é o binário único do Psicoman.
//
// Embarca o PocketBase como framework (biblioteca Go): o SQLite relacional, o
// servidor HTTP, o file storage, o logging, o auth/OAuth2, as migrations e a
// admin UI (/_/) vêm de dentro dele. As rotas próprias /v1/* são registradas
// no hook app.OnServe() (ver internal/api).
//
// A aplicação roda SEMPRE atrás do Pangolin e nunca é exposta direto à
// internet (ver .kiro/specs/pocketbase-core, docs/decisions.md 2026-10-21).
package main

import (
	"log"

	"github.com/pocketbase/pocketbase"

	"github.com/RafaelFino/psicoman/internal/api"
)

func main() {
	app := pocketbase.New()

	// Registra as rotas próprias /v1/* sobre o router do PocketBase.
	api.Register(app)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
