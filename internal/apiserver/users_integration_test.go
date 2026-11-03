package apiserver_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
)

func adminRequest(engine *gin.Engine, method, path, sid, csrf, ip, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: sid})
	req.Header.Set(middleware.CSRFHeader, csrf)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestIntegration_UserAdminCreateResetDisable(t *testing.T) {
	s := newAuthStack(t)
	s.seedUser(t, "bob", "sysadmin", testPassword, false)
	bobLogin := loginRequest(s.engine, "bob", testPassword, "203.0.113.100")
	require.Equal(t, http.StatusOK, bobLogin.Code)
	bobSid := sidOf(t, bobLogin)
	bobCSRF := csrfToken(bobLogin.Body.String())
	bobIP := "203.0.113.100"

	// Create — the sysadmin creates a user; server-controlled fields in the
	// body are ignored, the account starts pending
	create := adminRequest(s.engine, http.MethodPost, "/api/v1/users", bobSid, bobCSRF, bobIP,
		`{"username":"carol","password":"`+testPassword+`","role":"user","status":"active","must_change_password":false,"created_by":99}`)
	require.Equal(t, http.StatusCreated, create.Code)
	require.NotContains(t, create.Body.String(), "password_hash")
	var created struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &created))
	require.Equal(t, "pending", created.Status)

	var carolID int64
	var role, status string
	var mustChange bool
	var createdBy int64
	err := s.pool.QueryRow(s.ctx,
		`SELECT id, role, status, must_change_password, created_by FROM users WHERE username = 'carol'`).
		Scan(&carolID, &role, &status, &mustChange, &createdBy)
	require.NoError(t, err)
	require.Equal(t, created.ID, carolID)
	require.Equal(t, "user", role)
	require.Equal(t, "pending", status)
	require.True(t, mustChange)
	require.Equal(t, int64(1), createdBy, "created_by must be the acting sysadmin, not the forged 99")

	// A pending account cannot log in yet
	pendingLogin := loginRequest(s.engine, "carol", testPassword, "203.0.113.101")
	require.Equal(t, http.StatusForbidden, pendingLogin.Code)
	require.JSONEq(t, `{"code":"ACCOUNT_PENDING","message":"账号待安全管理员审核启用"}`, pendingLogin.Body.String())

	// Duplicate username → 409 with a uniform message that does not echo the username
	dup := adminRequest(s.engine, http.MethodPost, "/api/v1/users", bobSid, bobCSRF, bobIP,
		`{"username":"carol","password":"`+testPassword+`","role":"user"}`)
	require.Equal(t, http.StatusConflict, dup.Code)
	require.JSONEq(t, `{"code":"CONFLICT","message":"用户名已存在"}`, dup.Body.String())

	// Validation failures → 400 with field detail
	badRole := adminRequest(s.engine, http.MethodPost, "/api/v1/users", bobSid, bobCSRF, bobIP,
		`{"username":"eve","password":"`+testPassword+`","role":"superuser"}`)
	require.Equal(t, http.StatusBadRequest, badRole.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"role","message":"角色无效"}]}`, badRole.Body.String())
	weak := adminRequest(s.engine, http.MethodPost, "/api/v1/users", bobSid, bobCSRF, bobIP,
		`{"username":"eve","password":"password123456","role":"user"}`)
	require.Equal(t, http.StatusBadRequest, weak.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"password","message":"密码过于常见，请更换"}]}`, weak.Body.String())

	// The create was audited with actor/target/ip
	var actorID int64
	var actorName, actorRole, target, ipText string
	err = s.pool.QueryRow(s.ctx,
		`SELECT actor_id, actor_username, actor_role, target, ip::text FROM audit_logs WHERE action = 'account.create'`).
		Scan(&actorID, &actorName, &actorRole, &target, &ipText)
	require.NoError(t, err)
	require.Equal(t, int64(1), actorID)
	require.Equal(t, "bob", actorName)
	require.Equal(t, "sysadmin", actorRole)
	require.Equal(t, fmt.Sprintf("users:%d", carolID), target)
	prefix, err := netip.ParsePrefix(ipText)
	require.NoError(t, err)
	require.Equal(t, bobIP, prefix.Addr().String())

	// Reset — an active user's password is replaced, status unchanged
	s.seedUser(t, "alice", "user", testPassword, false)
	var aliceID int64
	require.NoError(t, s.pool.QueryRow(s.ctx, `SELECT id FROM users WHERE username = 'alice'`).Scan(&aliceID))
	aliceLogin := loginRequest(s.engine, "alice", testPassword, "203.0.113.102")
	require.Equal(t, http.StatusOK, aliceLogin.Code)
	aliceSid := sidOf(t, aliceLogin)

	reset := adminRequest(s.engine, http.MethodPatch, fmt.Sprintf("/api/v1/users/%d/password", aliceID), bobSid, bobCSRF, bobIP,
		`{"password":"New-Pass-2026!"}`)
	require.Equal(t, http.StatusNoContent, reset.Code)

	var aliceStatus string
	require.NoError(t, s.pool.QueryRow(s.ctx, `SELECT status FROM users WHERE id = $1`, aliceID).Scan(&aliceStatus))
	require.Equal(t, "active", aliceStatus, "reset must leave the status unchanged")

	// All of the user's sessions were kicked
	me := authedRequest(s.engine, http.MethodGet, "/api/v1/auth/me", aliceSid, "", "203.0.113.102")
	require.Equal(t, http.StatusUnauthorized, me.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, me.Body.String())

	// Old password rejected, new password accepted
	require.Equal(t, http.StatusUnauthorized, loginRequest(s.engine, "alice", testPassword, "203.0.113.103").Code)
	newLogin := loginRequest(s.engine, "alice", "New-Pass-2026!", "203.0.113.104")
	require.Equal(t, http.StatusOK, newLogin.Code)

	// History reuse rejected: resetting back to the just-replaced password
	reuse := adminRequest(s.engine, http.MethodPatch, fmt.Sprintf("/api/v1/users/%d/password", aliceID), bobSid, bobCSRF, bobIP,
		`{"password":"`+testPassword+`"}`)
	require.Equal(t, http.StatusBadRequest, reuse.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"password","message":"新密码不能与当前或最近使用的密码相同"}]}`, reuse.Body.String())

	// The reset was audited with the acting admin as actor
	var resetActorID int64
	var resetTarget string
	err = s.pool.QueryRow(s.ctx,
		`SELECT actor_id, target FROM audit_logs WHERE action = 'auth.password_reset'`).
		Scan(&resetActorID, &resetTarget)
	require.NoError(t, err)
	require.Equal(t, int64(1), resetActorID)
	require.Equal(t, fmt.Sprintf("users:%d", aliceID), resetTarget)

	// Disable — sessions are kicked and login is denied with ACCOUNT_DISABLED
	aliceSid2 := sidOf(t, loginRequest(s.engine, "alice", "New-Pass-2026!", "203.0.113.105"))
	disable := adminRequest(s.engine, http.MethodPost, fmt.Sprintf("/api/v1/users/%d/disable", aliceID), bobSid, bobCSRF, bobIP, "")
	require.Equal(t, http.StatusNoContent, disable.Code)

	me2 := authedRequest(s.engine, http.MethodGet, "/api/v1/auth/me", aliceSid2, "", "203.0.113.105")
	require.Equal(t, http.StatusUnauthorized, me2.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, me2.Body.String())
	disabledLogin := loginRequest(s.engine, "alice", "New-Pass-2026!", "203.0.113.106")
	require.Equal(t, http.StatusForbidden, disabledLogin.Code)
	require.JSONEq(t, `{"code":"ACCOUNT_DISABLED","message":"账号已停用，请联系管理员"}`, disabledLogin.Body.String())

	var disableActorID int64
	err = s.pool.QueryRow(s.ctx, `SELECT actor_id FROM audit_logs WHERE action = 'account.disable'`).Scan(&disableActorID)
	require.NoError(t, err)
	require.Equal(t, int64(1), disableActorID)

	// Unknown users → 404 with the same uniform message on both endpoints
	nfReset := adminRequest(s.engine, http.MethodPatch, "/api/v1/users/999999/password", bobSid, bobCSRF, bobIP,
		`{"password":"New-Pass-2026!"}`)
	require.Equal(t, http.StatusNotFound, nfReset.Code)
	require.JSONEq(t, `{"code":"NOT_FOUND","message":"用户不存在"}`, nfReset.Body.String())
	nfDisable := adminRequest(s.engine, http.MethodPost, "/api/v1/users/999999/disable", bobSid, bobCSRF, bobIP, "")
	require.Equal(t, http.StatusNotFound, nfDisable.Code)
	require.JSONEq(t, `{"code":"NOT_FOUND","message":"用户不存在"}`, nfDisable.Body.String())

	// Malformed user id → 400 with field detail
	badID := adminRequest(s.engine, http.MethodPatch, "/api/v1/users/abc/password", bobSid, bobCSRF, bobIP,
		`{"password":"New-Pass-2026!"}`)
	require.Equal(t, http.StatusBadRequest, badID.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"id","message":"用户 ID 无效"}]}`, badID.Body.String())

	// Non-sysadmin roles → 403; unauthenticated → 401
	s.seedUser(t, "dave", "user", testPassword, false)
	daveLogin := loginRequest(s.engine, "dave", testPassword, "203.0.113.107")
	require.Equal(t, http.StatusOK, daveLogin.Code)
	daveDenied := adminRequest(s.engine, http.MethodPost, "/api/v1/users", sidOf(t, daveLogin), csrfToken(daveLogin.Body.String()), "203.0.113.107",
		`{"username":"eve","password":"`+testPassword+`","role":"user"}`)
	require.Equal(t, http.StatusForbidden, daveDenied.Code)
	require.JSONEq(t, `{"code":"FORBIDDEN","message":"无权限执行此操作"}`, daveDenied.Body.String())

	anon := adminRequest(s.engine, http.MethodPost, "/api/v1/users", "", "", "203.0.113.108",
		`{"username":"eve","password":"`+testPassword+`","role":"user"}`)
	require.Equal(t, http.StatusUnauthorized, anon.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, anon.Body.String())

	// CSRF still enforced on state-changing admin routes
	noCSRF := adminRequest(s.engine, http.MethodPost, "/api/v1/users", bobSid, "", bobIP,
		`{"username":"eve","password":"`+testPassword+`","role":"user"}`)
	require.Equal(t, http.StatusForbidden, noCSRF.Code)
	require.JSONEq(t, `{"code":"CSRF_TOKEN_INVALID","message":"CSRF token 无效"}`, noCSRF.Body.String())
}
