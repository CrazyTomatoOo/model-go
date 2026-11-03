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

func TestListByUser_returnsOnlyOwnSessions(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	sessions := store.NewSessionStore(rdb)
	ctx := context.Background()

	own, err := sessions.Create(ctx, 42)
	require.NoError(t, err)
	other, err := sessions.Create(ctx, 43)
	require.NoError(t, err)

	sids, err := sessions.ListByUser(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, []string{own.SID}, sids)
	require.NotContains(t, sids, other.SID)
}

func TestDeleteByUserID_removesOnlyOwnSessions(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	sessions := store.NewSessionStore(rdb)
	ctx := context.Background()

	own, err := sessions.Create(ctx, 42)
	require.NoError(t, err)
	other, err := sessions.Create(ctx, 43)
	require.NoError(t, err)

	removed, err := sessions.DeleteByUserID(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	_, err = sessions.Get(ctx, own.SID)
	require.ErrorIs(t, err, store.ErrSessionNotFound)
	kept, err := sessions.Get(ctx, other.SID)
	require.NoError(t, err)
	require.Equal(t, int64(43), kept.UserID)
}

func TestListByUser_excludesExpiredSessions(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	sessions := store.NewSessionStore(rdb)
	ctx := context.Background()

	_, err := sessions.Create(ctx, 42)
	require.NoError(t, err)

	mr.FastForward(store.SessionTTL + time.Minute)

	sids, err := sessions.ListByUser(ctx, 42)
	require.NoError(t, err)
	require.Empty(t, sids)
}
