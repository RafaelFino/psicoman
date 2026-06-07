//go:build !embedfrontend

package web

import "net/http"

func frontendFS() http.FileSystem {
	return nil
}
