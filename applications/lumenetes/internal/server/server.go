// Package server wires the Connect API handlers and the embedded React
// frontend onto a single http.Handler.
package server

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/liamawhite/lumenetes/gen/lumenetes/v1/lumenetesv1connect"
	"github.com/liamawhite/lumenetes/internal/bridgeservice"
	"github.com/liamawhite/lumenetes/internal/circadianscheduleservice"
	"github.com/liamawhite/lumenetes/internal/groupservice"
	"github.com/liamawhite/lumenetes/internal/lightservice"
	"github.com/liamawhite/lumenetes/internal/sceneservice"
	"github.com/liamawhite/lumenetes/internal/switchservice"
	"github.com/liamawhite/lumenetes/internal/webui"
)

// New builds the handler that serves both the Connect API and the embedded
// frontend on one port. There's no TLS terminator in front of this pod -
// mesh mTLS carries transport security, not app-layer TLS to the container -
// so gRPC/gRPC-Web clients need HTTP/2 over cleartext (h2c) to negotiate at
// all; the caller enables that via http.Server's Protocols field rather than
// wrapping the handler, since Go 1.24+ supports h2c natively.
func New(
	bridgeSvc *bridgeservice.Service,
	lightSvc *lightservice.Service,
	switchSvc *switchservice.Service,
	groupSvc *groupservice.Service,
	sceneSvc *sceneservice.Service,
	circadianScheduleSvc *circadianscheduleservice.Service,
) (http.Handler, error) {
	mux := http.NewServeMux()

	bridgePath, bridgeHandler := lumenetesv1connect.NewBridgeServiceHandler(bridgeSvc)
	mux.Handle(bridgePath, bridgeHandler)

	lightPath, lightHandler := lumenetesv1connect.NewLightServiceHandler(lightSvc)
	mux.Handle(lightPath, lightHandler)

	switchPath, switchHandler := lumenetesv1connect.NewSwitchServiceHandler(switchSvc)
	mux.Handle(switchPath, switchHandler)

	groupPath, groupHandler := lumenetesv1connect.NewGroupServiceHandler(groupSvc)
	mux.Handle(groupPath, groupHandler)

	scenePath, sceneHandler := lumenetesv1connect.NewSceneServiceHandler(sceneSvc)
	mux.Handle(scenePath, sceneHandler)

	circadianSchedulePath, circadianScheduleHandler := lumenetesv1connect.NewCircadianScheduleServiceHandler(circadianScheduleSvc)
	mux.Handle(circadianSchedulePath, circadianScheduleHandler)

	spa, err := newSPAHandler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spa)

	return mux, nil
}

// newSPAHandler serves the embedded frontend build, falling back to
// index.html for any path that isn't a real file - harmless today (there's
// no client-side routing state to preserve beyond the URL itself) and lets
// deep links like /lights survive a hard refresh.
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
