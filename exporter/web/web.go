// Package web serves the dashboard's static files. They are built by `npm run
// build` in frontend/ straight into ./dist and embedded in the binary, so the
// whole app is a single file with nothing to install next to it.
package web

import (
	"compress/gzip"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

//go:embed all:dist
var dist embed.FS

func init() {
	// Not in Go's built-in table on every version.
	_ = mime.AddExtensionType(".woff2", "font/woff2")
}

// Handler serves the embedded build. The dashboard has no routes of its own and
// asks for everything with relative addresses, so index.html is served only at
// "/" and a path that matches nothing is a real 404 (answering it with index.html
// would load a page whose assets resolve to the wrong place).
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // the embedded tree always has dist/
	}
	return newHandler(sub)
}

func newHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		ctype := mime.TypeByExtension(path.Ext(name))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		h := w.Header()
		h.Set("Content-Type", ctype)
		h.Set("Cache-Control", cacheControl(name))
		h.Add("Vary", "Accept-Encoding")

		if compressible(ctype) && acceptsGzip(r) {
			h.Set("Content-Encoding", "gzip")
			if r.Method == http.MethodHead {
				return
			}
			gz := gzip.NewWriter(w)
			_, _ = gz.Write(data)
			_ = gz.Close()
			return
		}
		h.Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(data)
	})
}

// cacheControl: Vite names everything under assets/ with a content hash, so it
// can be cached forever; the page itself must always be revalidated.
func cacheControl(name string) string {
	switch {
	case strings.HasPrefix(name, "assets/"):
		return "public, max-age=31536000, immutable"
	case name == "index.html":
		return "no-cache"
	}
	return "public, max-age=3600"
}

func compressible(ctype string) bool {
	return strings.HasPrefix(ctype, "text/") ||
		strings.Contains(ctype, "javascript") ||
		strings.Contains(ctype, "json") ||
		strings.Contains(ctype, "svg") ||
		strings.Contains(ctype, "xml")
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		enc, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(enc, "gzip") {
			return true
		}
	}
	return false
}
