package main

import (
	"context"
	"io/fs"
	"log/slog"
	"melodical/internal/server"
	"melodical/internal/store"
	"melodical/web"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("melodical stopped", "error", err)
		os.Exit(1)
	}
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func run() error {
	storage, err := store.OpenSQLite(env("MELODICAL_DB", "data/melodical.db"))
	if err != nil {
		return err
	}
	defer storage.Close()
	assets, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		return err
	}
	app := &http.Server{Addr: env("MELODICAL_ADDR", ":8080"), Handler: server.New(storage, assets), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { slog.Info("Melodical is ready", "address", app.Addr); done <- app.ListenAndServe() }()
	select {
	case err = <-done:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return app.Shutdown(shutdown)
	}
}
