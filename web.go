package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/dist
var web embed.FS

func MustGetWebHandler() http.Handler {
	fs, err := fs.Sub(web, "web/dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fs))
}
