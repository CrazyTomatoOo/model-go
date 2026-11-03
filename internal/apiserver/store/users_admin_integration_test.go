package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/testutil"
	pkgstore "github.com/CrazyTomatoOo/model-go/pkg/store"
)

// TestStoreIntegration_UserAdmin exercises the sysadmin account-management
// half of UserStore (create, reset with history trim, disable) against a
// real containerized database. All subtests share one stack.
func TestStoreIntegration_UserAdmin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, _ := testutil.StartStack(ctx, t)
	require.NoError(t, pkgstore.Migrate(ctx, pool))
	users := store.NewUserStore(pool)

	t.Run("create_user", func(t *testing.T) {
		var actorID int64
		err := pool.QueryRow(ctx,
			`INSERT INTO users (username, password_hash, role, status)
			 VALUES ('bob', 'phc', 'sysadmin', 'active') RETURNING id`).Scan(&actorID)
		require.NoError(t, err)

		id, err := users.Create(ctx, "carol", "phc-carol", store.RoleUser, actorID)
		require.NoError(t, err)
		require.Positive(t, id)

		var username, role, status string
		var mustChange bool
		var createdBy int64
		err = pool.QueryRow(ctx,
			`SELECT username, role, status, must_change_password, created_by
			   FROM users WHERE id = $1`, id).
			Scan(&username, &role, &status, &mustChange, &createdBy)
		require.NoError(t, err)
		require.Equal(t, "carol", username)
		require.Equal(t, store.RoleUser, role)
		require.Equal(t, store.StatusPending, status)
		require.True(t, mustChange, "created users must be must_change_password=true")
		require.Equal(t, actorID, createdBy)

		_, err = users.Create(ctx, "carol", "phc-again", store.RoleUser, actorID)
		require.ErrorIs(t, err, store.ErrUsernameTaken)
	})

	t.Run("reset_password_inserts_history_and_trims_to_five", func(t *testing.T) {
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO users (username, password_hash, role, status)
			 VALUES ('alice', 'h0', 'user', 'active') RETURNING id`).Scan(&id)
		require.NoError(t, err)

		for i := 1; i <= 6; i++ {
			require.NoError(t, users.ResetPassword(ctx, id, fmt.Sprintf("h%d", i)))
		}

		var current string
		var mustChange bool
		err = pool.QueryRow(ctx,
			`SELECT password_hash, must_change_password FROM users WHERE id = $1`, id).
			Scan(&current, &mustChange)
		require.NoError(t, err)
		require.Equal(t, "h6", current)
		require.True(t, mustChange, "reset must force a change on next login")

		var count int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM password_history WHERE user_id = $1`, id).Scan(&count))
		require.Equal(t, 5, count, "history must be trimmed to the newest 5")

		var newest string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT password_hash FROM password_history
			   WHERE user_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, id).Scan(&newest))
		require.Equal(t, "h5", newest)

		var oldCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM password_history WHERE user_id = $1 AND password_hash = 'h0'`, id).Scan(&oldCount))
		require.Zero(t, oldCount, "the oldest hash must fall off the 5-entry window")

		require.ErrorIs(t, users.ResetPassword(ctx, 999999, "h7"), store.ErrUserNotFound)
	})

	t.Run("disable_user", func(t *testing.T) {
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO users (username, password_hash, role, status)
			 VALUES ('dave', 'phc', 'user', 'active') RETURNING id`).Scan(&id)
		require.NoError(t, err)

		require.NoError(t, users.Disable(ctx, id))
		var status string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM users WHERE id = $1`, id).Scan(&status))
		require.Equal(t, store.StatusDisabled, status)

		require.NoError(t, users.Disable(ctx, id), "disabling an already-disabled user is idempotent")
		require.ErrorIs(t, users.Disable(ctx, 999999), store.ErrUserNotFound)
	})
}
