package frontend

import (
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var content embed.FS

func HandleFrontend() http.Handler {
	dist, err := fs.Sub(content, "dist")
	if err != nil {
		panic(err)
	}

	if _, err := fs.Stat(dist, "index.html"); err != nil {
		slog.Warn("stat of index.html failed; frontend not built, UI will 404", "hint", "make frontend")
	}

	return serveSPA(dist, http.FileServerFS(dist))
}

func serveSPA(dist fs.FS, files http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")

		if _, err := fs.Stat(dist, name); errors.Is(err, fs.ErrNotExist) && path.Ext(name) == "" {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}

		files.ServeHTTP(w, r)
	}
}
