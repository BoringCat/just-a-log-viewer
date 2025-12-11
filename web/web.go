package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist
var web embed.FS

func MustGetWebHandler(prefix string) http.Handler {
	fs, err := fs.Sub(web, "dist")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix(prefix, http.FileServer(http.FS(fs)))
}
