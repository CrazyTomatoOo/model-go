package apiserver_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver"
	"github.com/CrazyTomatoOo/model-go/internal/testutil"
	"github.com/CrazyTomatoOo/model-go/pkg/store"
)

func TestIntegration_MigrateFromScratch_then_healthz_serves_200_with_security_headers(t *testing.T) {
	// Given
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, rdb := testutil.StartStack(ctx, t)

	// When — migrations applied to a fresh database
	require.NoError(t, store.Migrate(ctx, pool))

	// Then — all three spec §3.1 tables exist
	for _, table := range []string{"users", "audit_logs", "password_history"} {
		var n int
		err := pool.QueryRow(ctx,
			`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1`,
			table).Scan(&n)
		require.NoError(t, err, "query %s existence", table)
		require.Equal(t, 1, n, "table %s must exist", table)
	}

	// Then — spec indexes exist
	for _, index := range []string{"idx_audit_created", "idx_audit_actor", "idx_password_history_user"} {
		var n int
		err := pool.QueryRow(ctx,
			`SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1`,
			index).Scan(&n)
		require.NoError(t, err, "query index %s", index)
		require.Equal(t, 1, n, "index %s must exist", index)
	}

	// Then — users constraints hold: must_change_password defaults true,
	// role/status CHECK constraints reject unknown values, username unique
	var mustChange bool
	err := pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash, role) VALUES ('alice', 'phc', 'user') RETURNING must_change_password`).
		Scan(&mustChange)
	require.NoError(t, err)
	require.True(t, mustChange, "must_change_password must default to true")

	_, err = pool.Exec(ctx, `INSERT INTO users (username, password_hash, role) VALUES ('bob', 'phc', 'superuser')`)
	require.Error(t, err, "role CHECK constraint must reject 'superuser'")

	_, err = pool.Exec(ctx, `INSERT INTO users (username, password_hash, role, status) VALUES ('carol', 'phc', 'user', 'banned')`)
	require.Error(t, err, "status CHECK constraint must reject 'banned'")

	_, err = pool.Exec(ctx, `INSERT INTO users (username, password_hash, role) VALUES ('alice', 'phc', 'user')`)
	require.Error(t, err, "username UNIQUE constraint must reject duplicates")

	_, err = pool.Exec(ctx, `INSERT INTO password_history (user_id, password_hash) VALUES (999999, 'phc')`)
	require.Error(t, err, "password_history user_id FK must reject unknown user")

	// Then — migration recorded cleanly; second run is a no-op leaving no dirty state
	var version int
	var dirty bool
	err = pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty)
	require.NoError(t, err)
	require.Equal(t, 1, version, "schema_migrations version must record migration 1")
	require.False(t, dirty, "schema_migrations must not be dirty after a clean run")

	require.NoError(t, store.Migrate(ctx, pool), "second migrate run must be an idempotent no-op")
	err = pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty)
	require.NoError(t, err)
	require.Equal(t, 1, version)
	require.False(t, dirty, "second run must leave schema_migrations clean")

	// When — GET /healthz through the real middleware chain
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := apiserver.NewEngine(pool, rdb, logger)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	engine.ServeHTTP(rec, req)

	// Then — 200 with spec §8 security headers on the response
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	require.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
	require.Equal(t, "default-src 'none'; frame-ancestors 'none'", rec.Header().Get("Content-Security-Policy"))
	require.NotEmpty(t, rec.Header().Get("X-Request-ID"), "requestid middleware must stamp X-Request-ID")
}
