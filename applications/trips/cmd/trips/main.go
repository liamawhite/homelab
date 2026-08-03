package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/liamawhite/trips/internal/accommodationservice"
	"github.com/liamawhite/trips/internal/flightdata"
	"github.com/liamawhite/trips/internal/flightservice"
	"github.com/liamawhite/trips/internal/itineraryservice"
	"github.com/liamawhite/trips/internal/placesdata"
	"github.com/liamawhite/trips/internal/server"
	"github.com/liamawhite/trips/internal/storage"
	"github.com/liamawhite/trips/internal/tripservice"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dbPath := envOrDefault("DB_PATH", "/data/trips.db")
	port := envOrDefault("PORT", "8080")

	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := storage.Migrate(db); err != nil {
		return err
	}

	trips := tripservice.New(storage.NewTrips(db))

	flightClient, err := newFlightDataClient()
	if err != nil {
		return err
	}
	flights := flightservice.New(storage.NewFlights(db), flightClient)

	accommodations := accommodationservice.New(storage.NewAccommodations(db))

	placesClient, err := newPlacesClient()
	if err != nil {
		return err
	}
	itinerary := itineraryservice.New(storage.NewItineraryItems(db), placesClient)

	handler, err := server.New(trips, flights, accommodations, itinerary)
	if err != nil {
		return err
	}

	// Allow HTTP/2 over cleartext (h2c): there's no TLS terminator in front
	// of this pod, so gRPC/gRPC-Web clients need this to negotiate HTTP/2 at
	// all. HTTP/1.1 stays enabled too, for the Connect protocol's
	// JSON/HTTP-1.1 mode and for plain browser asset requests.
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	httpServer := &http.Server{
		Addr:      net.JoinHostPort("", port),
		Handler:   handler,
		Protocols: protocols,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on :%s", port)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}

	return nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// newFlightDataClient reads the AeroDataBox API key from the path in
// FLIGHT_API_KEY_FILE (mounted from a Kubernetes Secret - see
// pkg/deploy/applications/trips.go). It's not an error for the file to be
// unset or empty: flight sync is an optional feature, and returning a nil
// client here is what tells flightservice.New to skip syncing rather than
// fail startup entirely.
func newFlightDataClient() (*flightdata.Client, error) {
	path := os.Getenv("FLIGHT_API_KEY_FILE")
	if path == "" {
		log.Print("FLIGHT_API_KEY_FILE not set, flight sync disabled")
		return nil, nil
	}

	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(key))
	if trimmed == "" {
		log.Print("flight API key file is empty, flight sync disabled")
		return nil, nil
	}

	return flightdata.NewClient(trimmed), nil
}

// newPlacesClient reads the Google Places API key from the path in
// GOOGLE_PLACES_API_KEY_FILE (mounted from a Kubernetes Secret - see
// pkg/deploy/applications/trips.go). Same "not an error, just disables the
// feature" posture as newFlightDataClient - itinerary location search is
// optional.
func newPlacesClient() (*placesdata.Client, error) {
	path := os.Getenv("GOOGLE_PLACES_API_KEY_FILE")
	if path == "" {
		log.Print("GOOGLE_PLACES_API_KEY_FILE not set, itinerary location search disabled")
		return nil, nil
	}

	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(key))
	if trimmed == "" {
		log.Print("Google Places API key file is empty, itinerary location search disabled")
		return nil, nil
	}

	return placesdata.NewClient(trimmed), nil
}
