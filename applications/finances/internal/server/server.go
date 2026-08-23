package server

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/liamawhite/finances/gen/finances/v1/financesv1connect"
	"github.com/liamawhite/finances/internal/assetservice"
	"github.com/liamawhite/finances/internal/exchangerateservice"
	"github.com/liamawhite/finances/internal/userservice"
	"github.com/liamawhite/finances/internal/webui"
)

func New(userSvc *userservice.Service, assetSvc *assetservice.Service, exchangeRateSvc *exchangerateservice.Service) (http.Handler, error) {
	mux := http.NewServeMux()

	userPath, userHandler := financesv1connect.NewUserServiceHandler(userSvc)
	mux.Handle(userPath, userHandler)

	assetPath, assetHandler := financesv1connect.NewAssetServiceHandler(assetSvc)
	mux.Handle(assetPath, assetHandler)

	exchangeRatePath, exchangeRateHandler := financesv1connect.NewExchangeRateServiceHandler(exchangeRateSvc)
	mux.Handle(exchangeRatePath, exchangeRateHandler)

	spa, err := newSPAHandler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spa)

	return mux, nil
}

// newSPAHandler serves the embedded frontend build, falling back to
// index.html for any path that isn't a real file. Directories don't count
// as a match even when one exists on disk (e.g. the client-side route
// "/assets" collides with Vite's own "dist/assets/" bundle directory) -
// only client-side routes ever resolve to a bare directory path, real
// static assets are always served by their exact file path.
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
		stat, err := fs.Stat(dist, name)
		if err != nil || stat.IsDir() {
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
