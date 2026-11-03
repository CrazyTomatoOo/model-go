// Package service implements the login domain logic (login design spec
// §5/§9) against narrow store interfaces so the whole flow is unit-testable
// with stubs.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
)

// Store seams the login service consumes.
type (
	UserStore interface {
		ByUsername(ctx context.Context, username string) (store.User, error)
		ByID(ctx context.Context, id int64) (store.User, error)
		TouchLastLogin(ctx context.Context, id int64) error
	}
	SessionStore interface {
		Create(ctx context.Context, userID int64) (store.Session, error)
		Get(ctx context.Context, sid string) (store.Session, error)
		Delete(ctx context.Context, sid string) error
	}
	LockStore interface {
		Count(ctx context.Context, username string) (int, error)
		Incr(ctx context.Context, username string) (int, error)
	}
	RateLimiter interface {
		Allow(ctx context.Context, ip string) (bool, error)
	}
	AuditStore interface {
		Log(ctx context.Context, e store.AuditEntry) error
	}
)

// Error kinds from the spec §4.5 error table, carried by *Error.
const (
	KindInvalidCredentials = "INVALID_CREDENTIALS"
	KindAccountPending     = "ACCOUNT_PENDING"
	KindAccountDisabled    = "ACCOUNT_DISABLED"
	KindRateLimited        = "RATE_LIMITED"
	KindSessionExpired     = "SESSION_EXPIRED"
)

// Error is a typed login outcome; the handler maps Kind to the §4.5
// HTTP status and envelope.
type Error struct {
	Kind    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// The distinct login outcomes.
var (
	ErrInvalidCredentials = &Error{Kind: KindInvalidCredentials, Message: "用户名或密码错误"}
	ErrAccountPending     = &Error{Kind: KindAccountPending, Message: "账号待安全管理员审核启用"}
	ErrAccountDisabled    = &Error{Kind: KindAccountDisabled, Message: "账号已停用，请联系管理员"}
	ErrRateLimited        = &Error{Kind: KindRateLimited, Message: "登录尝试过于频繁，请稍后再试"}
	ErrSessionExpired     = &Error{Kind: KindSessionExpired, Message: "会话已过期，请重新登录"}
)

// LoginResult is a successful login: the authenticated user plus the fresh
// session id and its CSRF token.
type LoginResult struct {
	User store.User
	SID  string
	CSRF string
}

// AuthService implements login, logout, and me (the ticket #8 closed loop).
type AuthService struct {
	users    UserStore
	sessions SessionStore
	locks    LockStore
	rate     RateLimiter
	audits   AuditStore
	logger   *slog.Logger
}

func NewAuthService(users UserStore, sessions SessionStore, locks LockStore, rate RateLimiter, audits AuditStore, logger *slog.Logger) *AuthService {
	return &AuthService{users: users, sessions: sessions, locks: locks, rate: rate, audits: audits, logger: logger}
}

// dummyHash is verified against when the submitted username does not exist,
// so the response time matches the wrong-password path (spec §4.1: no
// account-existence oracle). The params are fixed and valid, so hashing
// cannot fail; the panic guards a future params change.
var dummyHash = func() string {
	h, err := password.Hash("timing-equivalence-dummy-password")
	if err != nil {
		panic(err)
	}
	return h
}()

// Login runs the spec §5 flow: rate limit, lock check, password verify,
// status check, session creation. Credential failures return the unified
// ErrInvalidCredentials; status denials return their specific kind;
// over-budget returns ErrRateLimited.
func (s *AuthService) Login(ctx context.Context, username, plaintext, ip string) (LoginResult, error) {
	allowed, err := s.rate.Allow(ctx, ip)
	if err != nil {
		return LoginResult{}, err
	}
	if !allowed {
		return LoginResult{}, ErrRateLimited
	}

	locked, err := s.locks.Count(ctx, username)
	if err != nil {
		return LoginResult{}, err
	}
	if locked >= store.LockThreshold {
		s.auditFailure(ctx, username, ip, "locked")
		return LoginResult{}, ErrInvalidCredentials
	}

	user, err := s.users.ByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			_, _ = password.Verify(plaintext, dummyHash)
			s.recordFailure(ctx, username, ip, "user_not_found")
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}

	match, err := password.Verify(plaintext, user.PasswordHash)
	if err != nil {
		return LoginResult{}, err
	}
	if !match {
		s.recordFailure(ctx, username, ip, "invalid_password")
		return LoginResult{}, ErrInvalidCredentials
	}

	switch user.Status {
	case store.StatusPending:
		return LoginResult{}, ErrAccountPending
	case store.StatusDisabled:
		return LoginResult{}, ErrAccountDisabled
	case store.StatusActive:
	default:
		return LoginResult{}, fmt.Errorf("user %d has unknown status %q", user.ID, user.Status)
	}

	sess, err := s.sessions.Create(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	if err := s.users.TouchLastLogin(ctx, user.ID); err != nil {
		return LoginResult{}, err
	}
	s.auditSuccess(ctx, user, sess.SID, ip)
	return LoginResult{User: user, SID: sess.SID, CSRF: sess.CSRF}, nil
}

// Logout deletes the session and writes the auth.logout audit row.
func (s *AuthService) Logout(ctx context.Context, sid string, user store.User, ip string) error {
	if err := s.sessions.Delete(ctx, sid); err != nil {
		return err
	}
	if err := s.audits.Log(ctx, store.AuditEntry{
		ActorID:       &user.ID,
		ActorUsername: user.Username,
		ActorRole:     user.Role,
		Action:        "auth.logout",
		Target:        "sess:" + sid,
		IP:            ip,
	}); err != nil {
		s.logger.Error("audit logout", "user_id", user.ID, "error", err)
	}
	return nil
}

// Me resolves the session's user, or ErrSessionExpired when the session or
// its user vanished after the middleware validated them.
func (s *AuthService) Me(ctx context.Context, sid string) (store.User, string, error) {
	sess, err := s.sessions.Get(ctx, sid)
	if err != nil {
		if errors.Is(err, store.ErrSessionNotFound) {
			return store.User{}, "", ErrSessionExpired
		}
		return store.User{}, "", err
	}
	user, err := s.users.ByID(ctx, sess.UserID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return store.User{}, "", ErrSessionExpired
		}
		return store.User{}, "", err
	}
	return user, sess.CSRF, nil
}

// recordFailure increments the lockout counter and audits the failure;
// recording errors are logged, never surfaced — the unified 401 outcome
// must not be masked by a failed counter or audit write.
func (s *AuthService) recordFailure(ctx context.Context, username, ip, reason string) {
	if _, err := s.locks.Incr(ctx, username); err != nil {
		s.logger.Error("record login failure: incr lock", "username", username, "error", err)
	}
	s.auditFailure(ctx, username, ip, reason)
}

// auditFailure writes one auth.login_failure row (best-effort).
func (s *AuthService) auditFailure(ctx context.Context, username, ip, reason string) {
	if err := s.audits.Log(ctx, store.AuditEntry{
		ActorUsername: username,
		Action:        "auth.login_failure",
		Detail:        map[string]any{"reason": reason},
		IP:            ip,
	}); err != nil {
		s.logger.Error("record login failure: audit", "username", username, "error", err)
	}
}

// auditSuccess writes one auth.login_success row (best-effort).
func (s *AuthService) auditSuccess(ctx context.Context, user store.User, sid, ip string) {
	if err := s.audits.Log(ctx, store.AuditEntry{
		ActorID:       &user.ID,
		ActorUsername: user.Username,
		ActorRole:     user.Role,
		Action:        "auth.login_success",
		Target:        "sess:" + sid,
		Detail:        map[string]any{"must_change_password": user.MustChangePassword},
		IP:            ip,
	}); err != nil {
		s.logger.Error("audit login success", "user_id", user.ID, "error", err)
	}
}
