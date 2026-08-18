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

	"github.com/convin/webhook-ingest/internal/config"
	"github.com/convin/webhook-ingest/internal/httpapi"
	"github.com/convin/webhook-ingest/internal/ingest"
	"github.com/convin/webhook-ingest/internal/redisclient"
	"github.com/convin/webhook-ingest/internal/stats"
	"github.com/convin/webhook-ingest/internal/store"
)

const shutdownTimeout = 10 * time.Second

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	ctx := context.Background()

	st, err := store.New(ctx, cfg.PostgresDSN, cfg.DBMaxConns)
	if err != nil {
		log.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	rdb, err := redisclient.New(ctx, cfg.RedisAddr)
	if err != nil {
		log.Error("connect redis", "err", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	svc := ingest.New(st, stats.NewCache(), rdb, log)
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.NewRouter(svc, log)}

	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}
