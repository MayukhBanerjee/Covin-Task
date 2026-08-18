// Package testutil provides shared setup for tests that need Postgres.
package testutil

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/convin/webhook-ingest/internal/config"
	"github.com/convin/webhook-ingest/internal/httpapi"
	"github.com/convin/webhook-ingest/internal/ingest"
	"github.com/convin/webhook-ingest/internal/redisclient"
	"github.com/convin/webhook-ingest/internal/stats"
	"github.com/convin/webhook-ingest/internal/store"
)

// IDs returns event, call, and account identifiers unique to this test, and
// removes any rows owned by them before the test runs and again afterwards.
//
// `go test ./...` runs package test binaries in parallel against one shared
// database, so tests must never truncate shared tables. Every row carries an
// account_id, so deleting by account removes exactly this test's data and
// nothing else.
func IDs(t *testing.T, s *store.Store) (eventID, callID, accountID string) {
	t.Helper()
	base := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	eventID, callID, accountID = "evt_"+base, "call_"+base, "acc_"+base

	clean := func() {
		ctx := context.Background()
		for _, table := range []string{"events", "calls", "account_stats"} {
			if _, err := s.Pool().Exec(ctx,
				"DELETE FROM "+table+" WHERE account_id = $1", accountID); err != nil {
				t.Fatalf("clean %s: %v", table, err)
			}
		}
	}
	clean()
	t.Cleanup(clean)

	return eventID, callID, accountID
}

// NewStore opens a store against the configured database and closes it
// when the test finishes.
func NewStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := config.Load()
	s, err := store.New(context.Background(), cfg.PostgresDSN, cfg.DBMaxConns)
	if err != nil {
		t.Fatalf("connect to postgres (is `docker compose up` running?): %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

// NewServer starts an in-process HTTP server backed by the configured
// Postgres and Redis, and returns it alongside the store for assertions.
func NewServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	cfg := config.Load()

	s := NewStore(t)

	rdb, err := redisclient.New(context.Background(), cfg.RedisAddr)
	if err != nil {
		t.Fatalf("connect to redis (is `docker compose up` running?): %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := ingest.New(s, stats.NewCache(), rdb, log)

	srv := httptest.NewServer(httpapi.NewRouter(svc, log))
	t.Cleanup(srv.Close)
	return srv, s
}
