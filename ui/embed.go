package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded filesystem
func FS() fs.FS {
	return distFS
}

// Handler returns an HTTP handler for serving the UI
func Handler() http.Handler {
	fsys, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fsys))
}
