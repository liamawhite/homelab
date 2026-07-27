// Package server wires the Connect API handler and the embedded React
// frontend onto a single http.Handler.
package server

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/liamawhite/trips/gen/trips/v1/tripsv1connect"
	"github.com/liamawhite/trips/internal/accommodationservice"
	"github.com/liamawhite/trips/internal/flightservice"
	"github.com/liamawhite/trips/internal/tripservice"
	"github.com/liamawhite/trips/internal/webui"
)

// New builds the handler that serves both the Connect API and the embedded
// frontend on one port. There's no TLS terminator in front of this pod -
// mesh mTLS carries transport security, not app-layer TLS to the container -
// so gRPC/gRPC-Web clients need HTTP/2 over cleartext (h2c) to negotiate at
// all; the caller enables that via http.Server's Protocols field rather than
// wrapping the handler, since Go 1.24+ supports h2c natively.
func New(trips *tripservice.Service, flights *flightservice.Service, accommodations *accommodationservice.Service) (http.Handler, error) {
	mux := http.NewServeMux()

	tripsPath, tripsHandler := tripsv1connect.NewTripServiceHandler(trips)
	mux.Handle(tripsPath, tripsHandler)

	flightsPath, flightsHandler := tripsv1connect.NewFlightServiceHandler(flights)
	mux.Handle(flightsPath, flightsHandler)

	accommodationsPath, accommodationsHandler := tripsv1connect.NewAccommodationServiceHandler(accommodations)
	mux.Handle(accommodationsPath, accommodationsHandler)

	spa, err := newSPAHandler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spa)

	return mux, nil
}

// newSPAHandler serves the embedded frontend build, falling back to
// index.html for any path that isn't a real file - harmless today (there's
// no client-side routing yet) and cheap to have in place for when there is.
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
