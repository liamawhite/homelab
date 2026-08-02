package server

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/liamawhite/finances/gen/finances/v1/financesv1connect"
	"github.com/liamawhite/finances/internal/userservice"
	"github.com/liamawhite/finances/internal/webui"
)

func New(userSvc *userservice.Service) (http.Handler, error) {
	mux := http.NewServeMux()

	userPath, userHandler := financesv1connect.NewUserServiceHandler(userSvc)
	mux.Handle(userPath, userHandler)

	spa, err := newSPAHandler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spa)

	return mux, nil
}

// newSPAHandler serves the embedded frontend build, falling back to
// index.html for any path that isn't a real file.
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
