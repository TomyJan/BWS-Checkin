package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist
var embedded embed.FS

func Handler() http.Handler {
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return spaHandler{dist: dist}
}

type spaHandler struct {
	dist fs.FS
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	if stat, err := fs.Stat(h.dist, name); err != nil || stat.IsDir() {
		name = "index.html"
	}
	http.ServeFileFS(w, r, h.dist, name)
}
