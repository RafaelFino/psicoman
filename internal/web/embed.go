//go:build embedfrontend

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var embeddedFrontend embed.FS

func frontendFS() http.FileSystem {
	sub, err := fs.Sub(embeddedFrontend, "static")
	if err != nil {
		return nil
	}
	return http.FS(sub)
}
