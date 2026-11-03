package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuditQuerySQL_FilterCombinations(t *testing.T) {
	actorID := int64(42)
	from := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	t.Run("no filters", func(t *testing.T) {
		where, args, limit, offset := auditQuerySQL(AuditQuery{Page: 1})
		require.Equal(t, "", where)
		require.Empty(t, args)
		require.Equal(t, 50, limit)
		require.Equal(t, 0, offset)
	})

	t.Run("actor_id only", func(t *testing.T) {
		where, args, _, _ := auditQuerySQL(AuditQuery{ActorID: &actorID})
		require.Equal(t, " WHERE actor_id = $1", where)
		require.Equal(t, []any{int64(42)}, args)
	})

	t.Run("action only", func(t *testing.T) {
		where, args, _, _ := auditQuerySQL(AuditQuery{Action: "auth.login_failure"})
		require.Equal(t, " WHERE action = $1", where)
		require.Equal(t, []any{"auth.login_failure"}, args)
	})

	t.Run("from and to are inclusive bounds", func(t *testing.T) {
		where, args, _, _ := auditQuerySQL(AuditQuery{From: &from, To: &to})
		require.Equal(t, " WHERE created_at >= $1 AND created_at <= $2", where)
		require.Equal(t, []any{from, to}, args)
	})

	t.Run("all filters combined with numbered args", func(t *testing.T) {
		where, args, _, _ := auditQuerySQL(AuditQuery{ActorID: &actorID, Action: "account.disable", From: &from, To: &to})
		require.Equal(t, " WHERE actor_id = $1 AND action = $2 AND created_at >= $3 AND created_at <= $4", where)
		require.Equal(t, []any{int64(42), "account.disable", from, to}, args)
	})
}

func TestAuditQuerySQL_PaginationMath(t *testing.T) {
	for _, tc := range []struct {
		page   int
		offset int
	}{
		{1, 0},
		{2, 50},
		{3, 100},
		{7, 300},
		{0, 0},  // clamped to page 1
		{-3, 0}, // clamped to page 1
	} {
		_, _, limit, offset := auditQuerySQL(AuditQuery{Page: tc.page})
		require.Equal(t, 50, limit, "page size must be the fixed 50")
		require.Equal(t, tc.offset, offset, "page %d offset", tc.page)
	}
}
