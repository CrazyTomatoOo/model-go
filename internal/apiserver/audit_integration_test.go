package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	apistore "github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/testutil"
	pkgstore "github.com/CrazyTomatoOo/model-go/pkg/store"
)

type auditItemResp struct {
	ID            int64          `json:"id"`
	ActorID       *int64         `json:"actor_id"`
	ActorUsername string         `json:"actor_username"`
	ActorRole     *string        `json:"actor_role"`
	Action        string         `json:"action"`
	Target        *string        `json:"target"`
	Detail        map[string]any `json:"detail"`
	IP            *string        `json:"ip"`
	CreatedAt     time.Time      `json:"created_at"`
}

type auditPageResp struct {
	Items    []auditItemResp `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int64           `json:"total"`
}

func getAuditPage(t *testing.T, s *authStack, sid, path string) auditPageResp {
	t.Helper()
	rec := authedRequest(s.engine, http.MethodGet, path, sid, "", "203.0.113.50")
	require.Equal(t, http.StatusOK, rec.Code)
	var page auditPageResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	return page
}

func assertAuditDescending(t *testing.T, items []auditItemResp) {
	t.Helper()
	for i := 1; i < len(items); i++ {
		prev, cur := items[i-1], items[i]
		require.True(t, prev.CreatedAt.After(cur.CreatedAt) || prev.CreatedAt.Equal(cur.CreatedAt),
			"items %d and %d must not be out of created_at DESC order", i-1, i)
		if prev.CreatedAt.Equal(cur.CreatedAt) {
			require.Greater(t, prev.ID, cur.ID, "same-timestamp rows must break ties by id DESC")
		}
	}
}

func TestIntegration_AuditQueryPaginationFiltersAndOrder(t *testing.T) {
	s := newAuthStack(t)
	audits := apistore.NewAuditStore(s.pool)

	// Given — 55 rows (≥2 pages at page_size 50) through the append-only
	// store, the same path production writes use. One distinctive action
	// keeps the login-written rows out of the asserted counts.
	for i := 0; i < 55; i++ {
		require.NoError(t, audits.Log(s.ctx, apistore.AuditEntry{
			ActorUsername: "bulk-seeder",
			ActorRole:     apistore.RoleSysadmin,
			Action:        "account.disable",
		}))
	}

	// And — mixed rows for the filter assertions: actor_id/action combos
	// plus explicit timestamps to prove the inclusive from/to bounds.
	for i := 0; i < 2; i++ {
		require.NoError(t, audits.Log(s.ctx, apistore.AuditEntry{
			ActorID:       &[]int64{42}[0],
			ActorUsername: "filter-seeder",
			ActorRole:     apistore.RoleSysadmin,
			Action:        "account.disable",
		}))
		require.NoError(t, audits.Log(s.ctx, apistore.AuditEntry{
			ActorID:       &[]int64{43}[0],
			ActorUsername: "filter-seeder",
			ActorRole:     apistore.RoleSysadmin,
			Action:        "account.disable",
		}))
		require.NoError(t, audits.Log(s.ctx, apistore.AuditEntry{
			ActorUsername: "filter-seeder",
			Action:        "auth.login_failure",
		}))
	}
	for _, ts := range []string{"2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", "2026-01-03T00:00:00Z"} {
		_, err := s.pool.Exec(s.ctx,
			`INSERT INTO audit_logs (actor_username, actor_role, action, created_at)
			 VALUES ('timing-seeder', 'auditadmin', 'auth.test_timing', $1)`, ts)
		require.NoError(t, err)
	}

	s.seedUser(t, "audit-viewer", apistore.RoleAuditadmin, testPassword)
	login := loginRequest(s.engine, "audit-viewer", testPassword, "203.0.113.50")
	require.Equal(t, http.StatusOK, login.Code)
	sid := sidOf(t, login)

	// When — the auditadmin pages through action=account.disable rows:
	// page 1 holds 50 rows, page 2 the remaining 9, total stable across pages.
	page1 := getAuditPage(t, s, sid, "/api/v1/audit-logs?action=account.disable&page=1")
	require.Equal(t, 1, page1.Page)
	require.Equal(t, 50, page1.PageSize)
	require.Equal(t, int64(59), page1.Total)
	require.Len(t, page1.Items, 50)
	page2 := getAuditPage(t, s, sid, "/api/v1/audit-logs?action=account.disable&page=2")
	require.Equal(t, 2, page2.Page)
	require.Equal(t, 50, page2.PageSize)
	require.Equal(t, int64(59), page2.Total)
	require.Len(t, page2.Items, 9)

	// Then — the pages are newest-first with no overlap or gap between them.
	assertAuditDescending(t, page1.Items)
	assertAuditDescending(t, page2.Items)
	last, first := page1.Items[len(page1.Items)-1], page2.Items[0]
	require.True(t, last.CreatedAt.After(first.CreatedAt) || last.CreatedAt.Equal(first.CreatedAt))
	if last.CreatedAt.Equal(first.CreatedAt) {
		require.Greater(t, last.ID, first.ID)
	}

	// And — actor_id narrows to that actor only.
	byActor := getAuditPage(t, s, sid, "/api/v1/audit-logs?actor_id=42")
	require.Equal(t, int64(2), byActor.Total)
	for _, it := range byActor.Items {
		require.NotNil(t, it.ActorID)
		require.Equal(t, int64(42), *it.ActorID)
	}

	// And — filters combine: actor_id AND action.
	combined := getAuditPage(t, s, sid, "/api/v1/audit-logs?actor_id=42&action=account.disable")
	require.Equal(t, int64(2), combined.Total)

	// And — time bounds are inclusive on both ends and applied exactly.
	inWindow := getAuditPage(t, s, sid,
		"/api/v1/audit-logs?action=auth.test_timing&from=2026-01-02T00:00:00Z&to=2026-01-03T00:00:00Z")
	require.Equal(t, int64(2), inWindow.Total)
	require.Len(t, inWindow.Items, 2)
	require.True(t, inWindow.Items[0].CreatedAt.Equal(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)))
	require.True(t, inWindow.Items[1].CreatedAt.Equal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)))
}

func TestIntegration_AuditQueryRoleIsolation(t *testing.T) {
	s := newAuthStack(t)
	for _, tc := range []struct{ username, role string }{
		{"auditadmin", apistore.RoleAuditadmin},
		{"plain", apistore.RoleUser},
		{"sysadmin", apistore.RoleSysadmin},
		{"secadmin", apistore.RoleSecadmin},
	} {
		s.seedUser(t, tc.username, tc.role, testPassword)
	}

	// Given — an unauthenticated request hits the session middleware first.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	rec := httptest.NewRecorder()
	s.engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, rec.Body.String())

	// When — the auditadmin queries: 200 with the §4.4 envelope.
	login := loginRequest(s.engine, "auditadmin", testPassword, "203.0.113.60")
	require.Equal(t, http.StatusOK, login.Code)
	ok := authedRequest(s.engine, http.MethodGet, "/api/v1/audit-logs", sidOf(t, login), "", "203.0.113.60")
	require.Equal(t, http.StatusOK, ok.Code)
	var page auditPageResp
	require.NoError(t, json.Unmarshal(ok.Body.Bytes(), &page))
	require.Equal(t, 1, page.Page)
	require.Equal(t, 50, page.PageSize)
	require.GreaterOrEqual(t, page.Total, int64(1))
	require.NotEmpty(t, page.Items)
	require.NotZero(t, page.Items[0].ID)
	require.NotEmpty(t, page.Items[0].Action)

	// Then — user/sysadmin/secadmin are all rejected with 403 FORBIDDEN.
	for _, tc := range []struct{ username, role string }{
		{"plain", apistore.RoleUser},
		{"sysadmin", apistore.RoleSysadmin},
		{"secadmin", apistore.RoleSecadmin},
	} {
		l := loginRequest(s.engine, tc.username, testPassword, "203.0.113.70")
		require.Equal(t, http.StatusOK, l.Code)
		denied := authedRequest(s.engine, http.MethodGet, "/api/v1/audit-logs", sidOf(t, l), "", "203.0.113.70")
		require.Equal(t, http.StatusForbidden, denied.Code, "%s must be forbidden", tc.role)
		require.JSONEq(t, `{"code":"FORBIDDEN","message":"无权限执行此操作"}`, denied.Body.String())
	}
}

// TestIntegration_AuditAppendOnlyRoleEnforcement proves the §3.1
// append-only property can be enforced by database provisioning: a
// dedicated role granted only INSERT/SELECT on audit_logs can append but
// can never UPDATE or DELETE rows (SQLSTATE 42501). Creating that role and
// running the GRANT/REVOKE statements below is a production ops step (the
// DBA provisions the application role with INSERT/SELECT only, per spec
// §3.1); the application code itself never issues UPDATE/DELETE on
// audit_logs. The test only demonstrates the DB-level guarantee.
func TestIntegration_AuditAppendOnlyRoleEnforcement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, _ := testutil.StartStack(ctx, t)
	require.NoError(t, pkgstore.Migrate(ctx, pool))

	const (
		roleName = "audit_appender"
		rolePass = "append-only-pass"
	)
	_, err := pool.Exec(ctx, `CREATE ROLE audit_appender LOGIN PASSWORD 'append-only-pass'`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DROP ROLE IF EXISTS audit_appender`) })
	_, err = pool.Exec(ctx, `GRANT USAGE ON SCHEMA public TO audit_appender`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `GRANT INSERT, SELECT ON audit_logs TO audit_appender`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `GRANT USAGE ON SEQUENCE audit_logs_id_seq TO audit_appender`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `REVOKE UPDATE, DELETE ON audit_logs FROM audit_appender`)
	require.NoError(t, err)

	limited := connectAs(ctx, t, pool, roleName, rolePass)

	_, err = limited.Exec(ctx, `INSERT INTO audit_logs (actor_username, action) VALUES ('appender', 'auth.test_append')`)
	require.NoError(t, err, "append-only role must be able to INSERT")

	_, err = limited.Exec(ctx, `UPDATE audit_logs SET action = 'tampered' WHERE actor_username = 'appender'`)
	requirePGCode(t, err, "42501")
	_, err = limited.Exec(ctx, `DELETE FROM audit_logs WHERE actor_username = 'appender'`)
	requirePGCode(t, err, "42501")

	var n int
	err = limited.QueryRow(ctx,
		`SELECT count(*) FROM audit_logs WHERE actor_username = 'appender' AND action = 'auth.test_append'`).Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "the appended row must survive with its original content")
}

func connectAs(ctx context.Context, t *testing.T, pool *pgxpool.Pool, role, pass string) *pgxpool.Pool {
	t.Helper()
	dsn := pool.Config().ConnString()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.User = url.UserPassword(role, pass)
	limited, err := pgxpool.New(ctx, u.String())
	require.NoError(t, err)
	t.Cleanup(limited.Close)
	return limited
}

func requirePGCode(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, code, pgErr.Code)
}
