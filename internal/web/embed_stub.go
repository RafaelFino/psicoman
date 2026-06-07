//go:build !embedfrontend

package web

import "io/fs"

func frontendFS() fs.FS {
	return nil
}
