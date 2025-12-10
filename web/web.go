package web

import (
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist
var web embed.FS

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func makeGzipHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzr := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		h.ServeHTTP(gzr, r)
	}
}

func MustGetWebHandler(prefix string) http.Handler {
	fs, err := fs.Sub(web, "dist")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix(prefix, makeGzipHandler(http.FileServer(http.FS(fs))))
}
