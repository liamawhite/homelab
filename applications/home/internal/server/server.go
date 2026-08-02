// Package server wires the embedded React frontend onto a single
// http.Handler. Unlike this repo's other apps, there's no API to mount
// alongside it - this app has nothing to call, it's just links out to every
// other app.
package server

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/liamawhite/home/internal/webui"
)

// New builds the handler that serves the embedded frontend.
func New() (http.Handler, error) {
	mux := http.NewServeMux()

	spa, err := newSPAHandler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spa)

	return mux, nil
}

// newSPAHandler serves the embedded frontend build, falling back to
// index.html for any path that isn't a real file - harmless today (there's
// no client-side routing) and cheap to have in place for when there is.
func newSPAHandler() (http.Handler, error) {
	dist, err := fs.Sub(webui.DistFS, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "."
		}
		if _, err := fs.Stat(dist, name); err != nil {
			// Not a real file - serve index.html's contents directly rather
			// than delegating back to fileServer: http.FileServer special-
			// cases requests that resolve to a file literally named
			// index.html and 301-redirects them to "/", which would loop
			// this fallback back on itself.
			serveIndex(w, r, dist)
			return
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}

func serveIndex(w http.ResponseWriter, r *http.Request, dist fs.FS) {
	f, err := dist.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "embedded index.html is not seekable", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, "index.html", stat.ModTime(), rs)
}
