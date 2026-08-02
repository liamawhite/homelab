package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liamawhite/reminders/internal/oneoffitemservice"
	"github.com/liamawhite/reminders/internal/server"
	"github.com/liamawhite/reminders/internal/storage"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dbPath := envOrDefault("DB_PATH", "/data/reminders.db")
	port := envOrDefault("PORT", "8080")

	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := storage.Migrate(db); err != nil {
		return err
	}

	items := oneoffitemservice.New(storage.NewOneOffItems(db))

	handler, err := server.New(items)
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
