// Command api starts the REST API service.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yidecode/go-rest-api/internal/config"
	"github.com/yidecode/go-rest-api/internal/httpapi"
	"github.com/yidecode/go-rest-api/internal/store"
)

func main() {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	log := slog.Default()

	ctx := context.Background()
	var st store.Store
	if cfg.DatabaseURL != "" {
		pg, err := store.NewPostgresStore(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Error("postgres unavailable, falling back to in-memory store", "err", err)
			st = store.NewMemoryStore()
		} else {
			log.Info("using postgres store")
			st = pg
		}
	} else {
		log.Info("DATABASE_URL not set, using in-memory store")
		st = store.NewMemoryStore()
	}
	defer st.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(st, cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down")
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
}
