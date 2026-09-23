package web

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// Build the Vue app before compiling Go: make web-build.
// all: keeps Vite hashes that start with "_" or ".". A plain embed skips those
// names, and the browser then 404s a chunk the page still references.
//
//go:embed all:dist
var assets embed.FS

func NewHandler() (http.Handler, error) {
	root, err := fs.Sub(assets, "dist")
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(root, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read embedded frontend: %w", err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" || name == "index.html" {
			serveIndex(w, r, index)
			return
		}
		if data, err := fs.ReadFile(root, name); err == nil {
			w.Header().Set("Cache-Control", "no-cache")
			// Vite owns this directory and generates content-hashed filenames.
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
			return
		}
		segment, _, _ := strings.Cut(name, "/")
		reserved := segment == "assets" || segment == "api" || segment == "uploads" || segment == "health_check"
		if !reserved && path.Ext(name) == "" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			serveIndex(w, r, index)
			return
		}
		http.NotFound(w, r)
	}), nil
}

func serveIndex(w http.ResponseWriter, r *http.Request, index []byte) {
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
}
