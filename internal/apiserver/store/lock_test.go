package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

func TestLockStore_Count_missing_is_zero(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := store.NewLockStore(rdb)

	n, err := s.Count(context.Background(), "alice")
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestLockStore_Incr_then_Count_and_ttl(t *testing.T) {
	mr, rdb := newTestRedis(t)
	s := store.NewLockStore(rdb)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		n, err := s.Incr(ctx, "alice")
		require.NoError(t, err)
		require.Equal(t, i, n)
	}
	n, err := s.Count(ctx, "alice")
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, store.LockTTL, mr.TTL("lock:alice"))
}

func TestLockStore_Incr_refreshes_ttl(t *testing.T) {
	mr, rdb := newTestRedis(t)
	s := store.NewLockStore(rdb)
	ctx := context.Background()

	require.NoError(t, mr.Set("lock:alice", "4"))
	mr.SetTTL("lock:alice", time.Minute)

	n, err := s.Incr(ctx, "alice")
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, store.LockTTL, mr.TTL("lock:alice"))
}

func TestRateLimiter_Allow_budget(t *testing.T) {
	mr, rdb := newTestRedis(t)
	r := store.NewRateLimiter(rdb)
	ctx := context.Background()

	for i := 0; i < store.RateLimitPerMinute; i++ {
		allowed, err := r.Allow(ctx, "10.0.0.1")
		require.NoError(t, err)
		require.True(t, allowed, "attempt %d must be allowed", i+1)
	}

	allowed, err := r.Allow(ctx, "10.0.0.1")
	require.NoError(t, err)
	require.False(t, allowed, "attempt %d must be denied", store.RateLimitPerMinute+1)

	// a different IP has its own fresh budget
	allowed, err = r.Allow(ctx, "10.0.0.2")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, store.RateLimitWindow, mr.TTL("rl:10.0.0.2"))
}
