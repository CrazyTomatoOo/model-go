// Package testutil provides reusable integration-test fixtures backed by
// testcontainers (Postgres + Redis). It is the container harness base for
// apiserver integration tests in this and later login-feature tickets.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	redismod "github.com/testcontainers/testcontainers-go/modules/redis"
)

// requireDocker skips the test when the Docker daemon is unreachable.
func requireDocker(t *testing.T, ctx context.Context) {
	t.Helper()
	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		t.Skipf("testcontainers: docker provider unavailable: %v", err)
		return
	}
	if err := provider.Health(ctx); err != nil {
		t.Skipf("testcontainers: docker daemon unreachable: %v", err)
	}
}

// StartPostgres starts a fresh Postgres 16 container, waits until it accepts
// connections, and returns a connected pool. The container is removed by the
// test's cleanup.
func StartPostgres(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	requireDocker(t, ctx)

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("modelgo"),
		postgres.WithUsername("modelgo"),
		postgres.WithPassword("modelgo"),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = container.Terminate(stopCtx)
	})

	// pgx v5 defaults to sslmode=prefer and treats the TLS handshake reset
	// from an SSL-less container as a hard error; the container speaks no TLS.
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	// The container port is mapped before Postgres finishes startup; poll the
	// real readiness condition (a successful ping) instead of assuming it.
	waitReady(t, 30*time.Second, 500*time.Millisecond, func() error {
		return pool.Ping(ctx)
	}, "ping postgres")
	return pool
}

// StartRedis starts a fresh Redis 7 container, waits until it accepts
// connections, and returns a connected client. The container is removed by
// the test's cleanup.
func StartRedis(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()
	requireDocker(t, ctx)

	container, err := redismod.RunContainer(ctx, testcontainers.WithImage("redis:7-alpine"))
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = container.Terminate(stopCtx)
	})

	addr, err := container.PortEndpoint(ctx, "6379/tcp", "")
	if err != nil {
		t.Fatalf("redis endpoint: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })

	waitReady(t, 30*time.Second, 500*time.Millisecond, func() error {
		return client.Ping(ctx).Err()
	}, "ping redis")
	return client
}

// waitReady polls probe until it succeeds or the budget elapses; the sleep
// is backoff between condition checks, the condition itself is the probe.
func waitReady(t *testing.T, budget, backoff time.Duration, probe func() error, what string) {
	t.Helper()
	deadline := time.Now().Add(budget)
	var lastErr error
	for {
		if lastErr = probe(); lastErr == nil {
			return
		}
		if time.Now().Add(backoff).After(deadline) {
			t.Fatalf("%s: %v (not ready within %s)", what, lastErr, budget)
		}
		select {
		case <-time.After(backoff):
		case <-t.Context().Done():
			t.Fatalf("%s: test context done: %v", what, t.Context().Err())
		}
	}
}

// StartStack starts the Postgres and Redis containers together.
func StartStack(ctx context.Context, t *testing.T) (*pgxpool.Pool, *redis.Client) {
	t.Helper()
	pool := StartPostgres(ctx, t)
	rdb := StartRedis(ctx, t)
	return pool, rdb
}
