package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist
var web embed.FS

func MustGetWebHandler() http.Handler {
	fs, err := fs.Sub(web, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fs))
}
