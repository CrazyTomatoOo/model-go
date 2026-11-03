package store_test

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
	"github.com/CrazyTomatoOo/model-go/internal/testutil"
	pkgstore "github.com/CrazyTomatoOo/model-go/pkg/store"
)

// TestStoreIntegration_PostgresStores exercises the Postgres half of the
// store package (users + audit_logs) against a real containerized database.
// All subtests share one stack to keep container churn down.
func TestStoreIntegration_PostgresStores(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, _ := testutil.StartStack(ctx, t)
	require.NoError(t, pkgstore.Migrate(ctx, pool))

	t.Run("users_by_username_by_id_touch_last_login", func(t *testing.T) {
		users := store.NewUserStore(pool)
		hash, err := password.Hash("correct-horse-battery-staple")
		require.NoError(t, err)

		var id int64
		err = pool.QueryRow(ctx,
			`INSERT INTO users (username, password_hash, role, status)
			 VALUES ('alice', $1, 'user', 'active') RETURNING id`, hash).Scan(&id)
		require.NoError(t, err)

		u, err := users.ByUsername(ctx, "alice")
		require.NoError(t, err)
		require.Equal(t, id, u.ID)
		require.Equal(t, "alice", u.Username)
		require.Equal(t, hash, u.PasswordHash)
		require.Equal(t, "user", u.Role)
		require.Equal(t, "active", u.Status)
		require.True(t, u.MustChangePassword)

		byID, err := users.ByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, u, byID)

		_, err = users.ByUsername(ctx, "nobody")
		require.ErrorIs(t, err, store.ErrUserNotFound)
		_, err = users.ByID(ctx, 999999)
		require.ErrorIs(t, err, store.ErrUserNotFound)

		require.NoError(t, users.TouchLastLogin(ctx, id))
		var lastLogin time.Time
		err = pool.QueryRow(ctx, `SELECT last_login_at FROM users WHERE id = $1`, id).Scan(&lastLogin)
		require.NoError(t, err)
		require.WithinDuration(t, time.Now(), lastLogin, time.Minute)
	})

	t.Run("create_admins_and_has_any_admin", func(t *testing.T) {
		users := store.NewUserStore(pool)
		has, err := users.HasAnyAdmin(ctx)
		require.NoError(t, err)
		require.False(t, has)

		hash, err := password.Hash("Admin-Initial-Passw0rd!")
		require.NoError(t, err)
		require.NoError(t, users.CreateAdmins(ctx, []store.NewAdmin{
			{Username: "sysadmin", PasswordHash: hash, Role: store.RoleSysadmin},
			{Username: "secadmin", PasswordHash: hash, Role: store.RoleSecadmin},
			{Username: "auditadmin", PasswordHash: hash, Role: store.RoleAuditadmin},
		}))

		has, err = users.HasAnyAdmin(ctx)
		require.NoError(t, err)
		require.True(t, has)

		for _, role := range []string{store.RoleSysadmin, store.RoleSecadmin, store.RoleAuditadmin} {
			var status string
			var mustChange bool
			err = pool.QueryRow(ctx,
				`SELECT status, must_change_password FROM users WHERE role = $1`, role).
				Scan(&status, &mustChange)
			require.NoError(t, err)
			require.Equal(t, "active", status, "%s must be created active", role)
			require.True(t, mustChange, "%s must be created must_change_password=true", role)
		}

		var n int
		err = pool.QueryRow(ctx,
			`SELECT count(*) FROM audit_logs WHERE action = 'admin.bootstrap' AND actor_id IS NULL`).Scan(&n)
		require.NoError(t, err)
		require.Equal(t, 3, n, "each bootstrap admin gets an admin.bootstrap audit row")
	})

	t.Run("audit_log_with_detail_and_ip", func(t *testing.T) {
		audits := store.NewAuditStore(pool)
		actorID := int64(1)
		require.NoError(t, audits.Log(ctx, store.AuditEntry{
			ActorID:       &actorID,
			ActorUsername: "alice",
			ActorRole:     "user",
			Action:        "auth.login_success",
			Target:        "sess:abc123",
			Detail:        map[string]any{"must_change_password": true, "attempt": float64(3)},
			IP:            "203.0.113.9",
		}))

		var username, action, target, ipText string
		var rawDetail []byte
		err := pool.QueryRow(ctx,
			`SELECT actor_username, action, target, ip::text, detail
			   FROM audit_logs WHERE action = 'auth.login_success'`).
			Scan(&username, &action, &target, &ipText, &rawDetail)
		require.NoError(t, err)
		require.Equal(t, "alice", username)
		require.Equal(t, "auth.login_success", action)
		require.Equal(t, "sess:abc123", target)
		// inet text always carries a mask (/32 for a plain host)
		prefix, err := netip.ParsePrefix(ipText)
		require.NoError(t, err)
		require.True(t, prefix.Addr().Is4())
		require.Equal(t, "203.0.113.9", prefix.Addr().String())

		var detail map[string]any
		require.NoError(t, json.Unmarshal(rawDetail, &detail))
		require.Equal(t, true, detail["must_change_password"])
		require.Equal(t, float64(3), detail["attempt"])

		// an unparseable IP and nil detail store as NULL
		require.NoError(t, audits.Log(ctx, store.AuditEntry{
			ActorUsername: "nobody",
			Action:        "auth.login_failure",
			Detail:        nil,
			IP:            "not-an-ip",
		}))
		var ipIsNull bool
		err = pool.QueryRow(ctx,
			`SELECT ip IS NULL FROM audit_logs WHERE action = 'auth.login_failure'`).Scan(&ipIsNull)
		require.NoError(t, err)
		require.True(t, ipIsNull)
	})
}
