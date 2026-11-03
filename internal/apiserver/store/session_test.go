package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// newTestRedis starts an in-memory Redis and returns it with a connected
// client; the client is closed by test cleanup.
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestSessionStore_CreateGet_roundtrip(t *testing.T) {
	mr, rdb := newTestRedis(t)
	s := store.NewSessionStore(rdb)
	ctx := context.Background()

	sess, err := s.Create(ctx, 42)
	require.NoError(t, err)
	require.NotEmpty(t, sess.SID)
	require.NotEmpty(t, sess.CSRF)
	require.NotEqual(t, sess.SID, sess.CSRF)
	require.LessOrEqual(t, len(sess.SID), 64)

	got, err := s.Get(ctx, sess.SID)
	require.NoError(t, err)
	require.Equal(t, sess, got)
	require.WithinDuration(t, time.Now().Add(store.SessionAbsLifetime), sess.AbsExpires, 2*time.Second)

	// the sliding TTL is set at creation
	require.Equal(t, store.SessionTTL, mr.TTL("sess:"+sess.SID))
}

func TestSessionStore_Create_draws_fresh_sid_per_session(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := store.NewSessionStore(rdb)
	ctx := context.Background()

	first, err := s.Create(ctx, 1)
	require.NoError(t, err)
	second, err := s.Create(ctx, 1)
	require.NoError(t, err)
	require.NotEqual(t, first.SID, second.SID)
	require.NotEqual(t, first.CSRF, second.CSRF)
}

func TestSessionStore_Get_unknown_sid(t *testing.T) {
	_, rdb := newTestRedis(t)
	s := store.NewSessionStore(rdb)

	_, err := s.Get(context.Background(), "does-not-exist")
	require.ErrorIs(t, err, store.ErrSessionNotFound)
}

func TestSessionStore_Get_malformed_hash_fails_closed(t *testing.T) {
	mr, rdb := newTestRedis(t)
	s := store.NewSessionStore(rdb)
	ctx := context.Background()

	mr.HSet("sess:broken", "user_id", "1", "csrf", "")
	_, err := s.Get(ctx, "broken")
	require.Error(t, err, "missing csrf must fail closed")

	mr.HSet("sess:broken2", "user_id", "not-a-number", "csrf", "tok")
	_, err = s.Get(ctx, "broken2")
	require.Error(t, err, "malformed user_id must fail closed")
}

func TestSessionStore_Delete(t *testing.T) {
	mr, rdb := newTestRedis(t)
	s := store.NewSessionStore(rdb)
	ctx := context.Background()

	sess, err := s.Create(ctx, 7)
	require.NoError(t, err)
	require.NoError(t, s.Delete(ctx, sess.SID))

	_, err = s.Get(ctx, sess.SID)
	require.ErrorIs(t, err, store.ErrSessionNotFound)
	require.False(t, mr.Exists("sess:"+sess.SID))
}

func TestSessionStore_Touch_refreshes_sliding_ttl(t *testing.T) {
	mr, rdb := newTestRedis(t)
	s := store.NewSessionStore(rdb)
	ctx := context.Background()

	sess, err := s.Create(ctx, 7)
	require.NoError(t, err)
	mr.SetTTL("sess:"+sess.SID, time.Minute)

	require.NoError(t, s.Touch(ctx, sess.SID))
	require.Equal(t, store.SessionTTL, mr.TTL("sess:"+sess.SID))
}
