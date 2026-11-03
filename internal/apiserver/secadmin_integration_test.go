package apiserver_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apistore "github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
)

var hexSID = regexp.MustCompile(`^[0-9a-f]{16}$`)

// seedUserStatus inserts a user with the given status and returns its id.
// must_change_password is set explicitly (false for active actors so the
// must-change gate does not block their first request; true for pending
// accounts, matching the create-API behavior).
func (s *authStack) seedUserStatus(t *testing.T, username, role, plaintext, status string) int64 {
	t.Helper()
	hash, err := password.Hash(plaintext)
	require.NoError(t, err)
	var id int64
	err = s.pool.QueryRow(s.ctx,
		`INSERT INTO users (username, password_hash, role, status, must_change_password) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		username, hash, role, status, status == "pending").Scan(&id)
	require.NoError(t, err)
	return id
}

func secadminLogin(t *testing.T, s *authStack) (sid, csrf string) {
	t.Helper()
	rec := loginRequest(s.engine, "secadmin", testPassword, "203.0.113.60")
	require.Equal(t, http.StatusOK, rec.Code)
	return sidOf(t, rec), csrfToken(rec.Body.String())
}

func TestIntegration_SecadminApprovePending(t *testing.T) {
	s := newAuthStack(t)
	secID := s.seedUserStatus(t, "secadmin", "secadmin", testPassword, "active")
	aliceID := s.seedUserStatus(t, "alice", "user", testPassword, "pending")
	secSID, csrf := secadminLogin(t, s)

	// A pending account cannot log in yet
	pending := loginRequest(s.engine, "alice", testPassword, "203.0.113.61")
	require.Equal(t, http.StatusForbidden, pending.Code)
	require.JSONEq(t, `{"code":"ACCOUNT_PENDING","message":"账号待安全管理员审核启用"}`, pending.Body.String())

	// Approve pending → active (204), then login succeeds
	approve := authedRequest(s.engine, http.MethodPost, "/api/v1/users/"+itoa(aliceID)+"/approve", secSID, csrf, "203.0.113.60")
	require.Equal(t, http.StatusNoContent, approve.Code)
	login := loginRequest(s.engine, "alice", testPassword, "203.0.113.62")
	require.Equal(t, http.StatusOK, login.Code)

	// Approving again conflicts (409); an unknown user is a 404
	again := authedRequest(s.engine, http.MethodPost, "/api/v1/users/"+itoa(aliceID)+"/approve", secSID, csrf, "203.0.113.60")
	require.Equal(t, http.StatusConflict, again.Code)
	require.JSONEq(t, `{"code":"CONFLICT","message":"账号已是启用状态"}`, again.Body.String())
	missing := authedRequest(s.engine, http.MethodPost, "/api/v1/users/999999/approve", secSID, csrf, "203.0.113.60")
	require.Equal(t, http.StatusNotFound, missing.Code)
	require.JSONEq(t, `{"code":"NOT_FOUND","message":"资源不存在"}`, missing.Body.String())

	// approved_by is recorded and the account.approve audit row exists
	var approvedBy int64
	err := s.pool.QueryRow(s.ctx, `SELECT approved_by FROM users WHERE id = $1`, aliceID).Scan(&approvedBy)
	require.NoError(t, err)
	require.Equal(t, secID, approvedBy)
	var action, actor, target string
	err = s.pool.QueryRow(s.ctx,
		`SELECT action, actor_username, target FROM audit_logs WHERE action = 'account.approve'`).
		Scan(&action, &actor, &target)
	require.NoError(t, err)
	require.Equal(t, "account.approve", action)
	require.Equal(t, "secadmin", actor)
	require.Equal(t, "users:"+itoa(aliceID), target)
}

func TestIntegration_SecadminApproveDisabled(t *testing.T) {
	s := newAuthStack(t)
	s.seedUserStatus(t, "secadmin", "secadmin", testPassword, "active")
	bobID := s.seedUserStatus(t, "bob", "user", testPassword, "disabled")
	secSID, csrf := secadminLogin(t, s)

	disabled := loginRequest(s.engine, "bob", testPassword, "203.0.113.63")
	require.Equal(t, http.StatusForbidden, disabled.Code)
	require.JSONEq(t, `{"code":"ACCOUNT_DISABLED","message":"账号已停用，请联系管理员"}`, disabled.Body.String())

	approve := authedRequest(s.engine, http.MethodPost, "/api/v1/users/"+itoa(bobID)+"/approve", secSID, csrf, "203.0.113.60")
	require.Equal(t, http.StatusNoContent, approve.Code)
	login := loginRequest(s.engine, "bob", testPassword, "203.0.113.64")
	require.Equal(t, http.StatusOK, login.Code)

	var status string
	err := s.pool.QueryRow(s.ctx, `SELECT status FROM users WHERE id = $1`, bobID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "active", status)
}

func TestIntegration_SecadminUnlock(t *testing.T) {
	s := newAuthStack(t)
	s.seedUserStatus(t, "secadmin", "secadmin", testPassword, "active")
	carolID := s.seedUserStatus(t, "carol", "user", testPassword, "active")
	secSID, csrf := secadminLogin(t, s)

	// Five failures lock the account; the right password still 401s
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusUnauthorized, loginRequest(s.engine, "carol", "wrong-password", "203.0.113.70").Code)
	}
	require.Equal(t, http.StatusUnauthorized, loginRequest(s.engine, "carol", testPassword, "203.0.113.71").Code)

	unlock := authedRequest(s.engine, http.MethodPost, "/api/v1/users/"+itoa(carolID)+"/unlock", secSID, csrf, "203.0.113.60")
	require.Equal(t, http.StatusNoContent, unlock.Code)

	login := loginRequest(s.engine, "carol", testPassword, "203.0.113.72")
	require.Equal(t, http.StatusOK, login.Code)

	var count int
	err := s.pool.QueryRow(s.ctx, `SELECT count(*) FROM audit_logs WHERE action = 'account.unlock'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestIntegration_SecadminSessionsListFilterForceLogout(t *testing.T) {
	s := newAuthStack(t)
	s.seedUserStatus(t, "secadmin", "secadmin", testPassword, "active")
	aliceID := s.seedUserStatus(t, "alice", "user", testPassword, "active")
	s.seedUserStatus(t, "bob", "user", testPassword, "active")
	secSID, csrf := secadminLogin(t, s)
	aliceSID := sidOf(t, loginRequest(s.engine, "alice", testPassword, "203.0.113.80"))
	bobSID := sidOf(t, loginRequest(s.engine, "bob", testPassword, "203.0.113.81"))

	// The listing shows all sessions with digests, never the raw bearer ids
	list := authedRequest(s.engine, http.MethodGet, "/api/v1/sessions", secSID, "", "203.0.113.60")
	require.Equal(t, http.StatusOK, list.Code)
	var body struct {
		Items []struct {
			SID        string    `json:"sid"`
			UserID     int64     `json:"user_id"`
			CreatedAt  time.Time `json:"created_at"`
			AbsExpires time.Time `json:"abs_expires"`
		} `json:"items"`
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
		Total    int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &body))
	require.Equal(t, 3, body.Total)
	require.Equal(t, 1, body.Page)
	require.Equal(t, 50, body.PageSize)
	seen := map[string]bool{}
	for i, item := range body.Items {
		require.True(t, hexSID.MatchString(item.SID), "sid must be a 16-hex digest")
		require.NotEqual(t, aliceSID, item.SID)
		require.NotEqual(t, bobSID, item.SID)
		require.False(t, seen[item.SID], "sid digests must not repeat")
		seen[item.SID] = true
		require.False(t, item.AbsExpires.Before(item.CreatedAt))
		if i > 0 {
			require.False(t, body.Items[i].CreatedAt.After(body.Items[i-1].CreatedAt), "items must be created_at desc")
		}
	}

	// user_id filter narrows to that user's sessions
	filtered := authedRequest(s.engine, http.MethodGet, "/api/v1/sessions?user_id="+itoa(aliceID), secSID, "", "203.0.113.60")
	require.Equal(t, http.StatusOK, filtered.Code)
	var fbody struct {
		Items []struct {
			SID    string `json:"sid"`
			UserID int64  `json:"user_id"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(filtered.Body.Bytes(), &fbody))
	require.Equal(t, 1, fbody.Total)
	require.Equal(t, aliceID, fbody.Items[0].UserID)

	// Force logout kills the raw session; the other session survives
	kill := authedRequest(s.engine, http.MethodDelete, "/api/v1/sessions/"+aliceSID, secSID, csrf, "203.0.113.60")
	require.Equal(t, http.StatusNoContent, kill.Code)
	require.Equal(t, http.StatusUnauthorized, authedRequest(s.engine, http.MethodGet, "/api/v1/auth/me", aliceSID, "", "203.0.113.80").Code)
	require.Equal(t, http.StatusOK, authedRequest(s.engine, http.MethodGet, "/api/v1/auth/me", bobSID, "", "203.0.113.81").Code)

	// Unknown sid → 404
	missing := authedRequest(s.engine, http.MethodDelete, "/api/v1/sessions/"+aliceSID, secSID, csrf, "203.0.113.60")
	require.Equal(t, http.StatusNotFound, missing.Code)
	require.JSONEq(t, `{"code":"NOT_FOUND","message":"资源不存在"}`, missing.Body.String())

	var count int
	err := s.pool.QueryRow(s.ctx, `SELECT count(*) FROM audit_logs WHERE action = 'session.force_logout' AND target = $1`, "sess:"+aliceSID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestIntegration_SecadminSessionsPagination(t *testing.T) {
	s := newAuthStack(t)
	s.seedUserStatus(t, "secadmin", "secadmin", testPassword, "active")
	aliceID := s.seedUserStatus(t, "alice", "user", testPassword, "active")
	secSID, _ := secadminLogin(t, s)

	sessions := apistore.NewSessionStore(s.rdb)
	for i := 0; i < 55; i++ {
		_, err := sessions.Create(s.ctx, aliceID)
		require.NoError(t, err)
	}

	page1 := authedRequest(s.engine, http.MethodGet, "/api/v1/sessions", secSID, "", "203.0.113.60")
	require.Equal(t, http.StatusOK, page1.Code)
	var p1 struct {
		Items    []map[string]any `json:"items"`
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
		Total    int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(page1.Body.Bytes(), &p1))
	require.Equal(t, 56, p1.Total, "55 seeded + the secadmin's own session")
	require.Equal(t, 50, len(p1.Items))

	page2 := authedRequest(s.engine, http.MethodGet, "/api/v1/sessions?page=2", secSID, "", "203.0.113.60")
	require.Equal(t, http.StatusOK, page2.Code)
	var p2 struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(page2.Body.Bytes(), &p2))
	require.Equal(t, 6, len(p2.Items))
	require.Equal(t, 56, p2.Total)

	// user_id filter on the same data still paginates by 50
	uf := authedRequest(s.engine, http.MethodGet, "/api/v1/sessions?user_id="+itoa(aliceID), secSID, "", "203.0.113.60")
	require.Equal(t, http.StatusOK, uf.Code)
	var ufbody struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(uf.Body.Bytes(), &ufbody))
	require.Equal(t, 55, ufbody.Total)
	require.Equal(t, 50, len(ufbody.Items))

	// Malformed params are 400 VALIDATION_ERROR
	bad := authedRequest(s.engine, http.MethodGet, "/api/v1/sessions?user_id=abc", secSID, "", "203.0.113.60")
	require.Equal(t, http.StatusBadRequest, bad.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"user_id","message":"必须是整数"}]}`, bad.Body.String())
	badPage := authedRequest(s.engine, http.MethodGet, "/api/v1/sessions?page=0", secSID, "", "203.0.113.60")
	require.Equal(t, http.StatusBadRequest, badPage.Code)
}

func TestIntegration_SecadminRoleEnforcement(t *testing.T) {
	s := newAuthStack(t)
	s.seedUserStatus(t, "secadmin", "secadmin", testPassword, "active")
	daveID := s.seedUserStatus(t, "dave", "user", testPassword, "active")
	erinID := s.seedUserStatus(t, "erin", "user", testPassword, "pending")

	// Unauthenticated → 401 SESSION_EXPIRED
	anon := authedRequest(s.engine, http.MethodPost, "/api/v1/users/"+itoa(erinID)+"/approve", "", "", "203.0.113.90")
	require.Equal(t, http.StatusUnauthorized, anon.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, anon.Body.String())

	// Plain user → 403 FORBIDDEN on every secadmin route
	daveSID, daveCSRF := func() (string, string) {
		rec := loginRequest(s.engine, "dave", testPassword, "203.0.113.91")
		require.Equal(t, http.StatusOK, rec.Code)
		return sidOf(t, rec), csrfToken(rec.Body.String())
	}()
	require.Equal(t, http.StatusForbidden, authedRequest(s.engine, http.MethodPost, "/api/v1/users/"+itoa(erinID)+"/approve", daveSID, daveCSRF, "203.0.113.91").Code)
	require.Equal(t, http.StatusForbidden, authedRequest(s.engine, http.MethodGet, "/api/v1/sessions", daveSID, "", "203.0.113.91").Code)
	require.Equal(t, http.StatusForbidden, authedRequest(s.engine, http.MethodDelete, "/api/v1/sessions/anything", daveSID, daveCSRF, "203.0.113.91").Code)
	require.JSONEq(t, `{"code":"FORBIDDEN","message":"无权限执行此操作"}`, authedRequest(s.engine, http.MethodGet, "/api/v1/sessions", daveSID, "", "203.0.113.91").Body.String())

	// secadmin can approve and unlock
	secSID, csrf := secadminLogin(t, s)
	require.Equal(t, http.StatusNoContent, authedRequest(s.engine, http.MethodPost, "/api/v1/users/"+itoa(erinID)+"/approve", secSID, csrf, "203.0.113.60").Code)
	require.Equal(t, http.StatusNoContent, authedRequest(s.engine, http.MethodPost, "/api/v1/users/"+itoa(daveID)+"/unlock", secSID, csrf, "203.0.113.60").Code)
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
