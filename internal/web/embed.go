//go:build embedfrontend

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var embeddedFrontend embed.FS

func frontendFS() fs.FS {
	sub, err := fs.Sub(embeddedFrontend, "static")
	if err != nil {
		return nil
	}
	return sub
}
