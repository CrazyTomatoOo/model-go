package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
)

var testUserHash = func() string {
	h, err := password.Hash("Str0ng!Pass42")
	if err != nil {
		panic(err)
	}
	return h
}()

func activeUser(username string) store.User {
	return store.User{ID: 7, Username: username, PasswordHash: testUserHash, Role: store.RoleUser, Status: store.StatusActive}
}

type stubUsers struct {
	byUsername func(context.Context, string) (store.User, error)
	byID       func(context.Context, int64) (store.User, error)
	touched    []int64
	touchErr   error
}

func (s *stubUsers) ByUsername(ctx context.Context, username string) (store.User, error) {
	return s.byUsername(ctx, username)
}

func (s *stubUsers) ByID(ctx context.Context, id int64) (store.User, error) {
	return s.byID(ctx, id)
}

func (s *stubUsers) TouchLastLogin(ctx context.Context, id int64) error {
	s.touched = append(s.touched, id)
	return s.touchErr
}

type stubSessions struct {
	created   []int64
	create    func(context.Context, int64) (store.Session, error)
	deleted   []string
	getResult store.Session
	getErr    error
}

func (s *stubSessions) Create(ctx context.Context, userID int64) (store.Session, error) {
	s.created = append(s.created, userID)
	return s.create(ctx, userID)
}

func (s *stubSessions) Get(ctx context.Context, sid string) (store.Session, error) {
	return s.getResult, s.getErr
}

func (s *stubSessions) Delete(ctx context.Context, sid string) error {
	s.deleted = append(s.deleted, sid)
	return nil
}

type stubLocks struct {
	count     int
	countErr  error
	incrCalls []string
	incrErr   error
}

func (s *stubLocks) Count(ctx context.Context, username string) (int, error) {
	return s.count, s.countErr
}

func (s *stubLocks) Incr(ctx context.Context, username string) (int, error) {
	s.incrCalls = append(s.incrCalls, username)
	return len(s.incrCalls), s.incrErr
}

type stubRate struct {
	allowed bool
	err     error
}

func (s *stubRate) Allow(ctx context.Context, ip string) (bool, error) {
	return s.allowed, s.err
}

type stubAudit struct {
	entries []store.AuditEntry
	err     error
}

func (s *stubAudit) Log(ctx context.Context, e store.AuditEntry) error {
	s.entries = append(s.entries, e)
	return s.err
}

type authStubs struct {
	users    *stubUsers
	sessions *stubSessions
	locks    *stubLocks
	rate     *stubRate
	audits   *stubAudit
}

func newAuthStubs() *authStubs {
	s := &authStubs{
		users:    &stubUsers{byUsername: func(context.Context, string) (store.User, error) { return store.User{}, store.ErrUserNotFound }},
		sessions: &stubSessions{create: func(context.Context, int64) (store.Session, error) { return store.Session{}, errors.New("unexpected create") }},
		locks:    &stubLocks{},
		rate:     &stubRate{allowed: true},
		audits:   &stubAudit{},
	}
	s.users.byID = func(_ context.Context, id int64) (store.User, error) { return store.User{}, store.ErrUserNotFound }
	return s
}

func (s *authStubs) service() *AuthService {
	return NewAuthService(s.users, s.sessions, s.locks, s.rate, s.audits, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func loginSessions() *stubSessions {
	return &stubSessions{create: func(_ context.Context, userID int64) (store.Session, error) {
		return store.Session{SID: "sid-abc", UserID: userID, CSRF: "csrf-xyz"}, nil
	}}
}

func TestLoginSuccess(t *testing.T) {
	st := newAuthStubs()
	user := activeUser("alice")
	st.users.byUsername = func(_ context.Context, username string) (store.User, error) { return user, nil }
	st.sessions = loginSessions()

	res, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
	require.NoError(t, err)
	require.Equal(t, LoginResult{User: user, SID: "sid-abc", CSRF: "csrf-xyz"}, res)
	require.Equal(t, []int64{7}, st.users.touched)
	require.Equal(t, []int64{7}, st.sessions.created)
	require.Empty(t, st.locks.incrCalls)
	require.Len(t, st.audits.entries, 1)
	require.Equal(t, "auth.login_success", st.audits.entries[0].Action)
	require.Equal(t, "sess:sid-abc", st.audits.entries[0].Target)
}

func TestLoginWrongPassword(t *testing.T) {
	st := newAuthStubs()
	st.users.byUsername = func(_ context.Context, username string) (store.User, error) { return activeUser("alice"), nil }

	_, err := st.service().Login(context.Background(), "alice", "Wrong!Pass42", "10.0.0.1")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	require.Equal(t, []string{"alice"}, st.locks.incrCalls)
	require.Len(t, st.audits.entries, 1)
	require.Equal(t, "auth.login_failure", st.audits.entries[0].Action)
	require.Equal(t, map[string]any{"reason": "invalid_password"}, st.audits.entries[0].Detail)
	require.Empty(t, st.sessions.created)
}

func TestLoginUnknownUser(t *testing.T) {
	st := newAuthStubs()

	_, err := st.service().Login(context.Background(), "ghost", "Str0ng!Pass42", "10.0.0.1")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	require.Equal(t, []string{"ghost"}, st.locks.incrCalls)
	require.Len(t, st.audits.entries, 1)
	require.Equal(t, map[string]any{"reason": "user_not_found"}, st.audits.entries[0].Detail)
}

func TestLoginLockedAccount(t *testing.T) {
	st := newAuthStubs()
	st.locks.count = store.LockThreshold

	_, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	require.Empty(t, st.locks.incrCalls)
	require.Len(t, st.audits.entries, 1)
	require.Equal(t, map[string]any{"reason": "locked"}, st.audits.entries[0].Detail)
	require.Empty(t, st.sessions.created)
}

func TestLoginRateLimited(t *testing.T) {
	st := newAuthStubs()
	st.rate.allowed = false

	_, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
	require.ErrorIs(t, err, ErrRateLimited)
	require.Empty(t, st.locks.incrCalls)
	require.Empty(t, st.users.touched)
	require.Empty(t, st.audits.entries)
}

func TestLoginStatusDenials(t *testing.T) {
	cases := []struct {
		status string
		want   error
	}{
		{store.StatusPending, ErrAccountPending},
		{store.StatusDisabled, ErrAccountDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			st := newAuthStubs()
			user := activeUser("alice")
			user.Status = tc.status
			st.users.byUsername = func(_ context.Context, username string) (store.User, error) { return user, nil }

			_, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
			require.ErrorIs(t, err, tc.want)
			require.Empty(t, st.sessions.created)
			require.Empty(t, st.locks.incrCalls)
		})
	}
}

func TestLoginUnknownStatus(t *testing.T) {
	st := newAuthStubs()
	user := activeUser("alice")
	user.Status = "unknown"
	st.users.byUsername = func(_ context.Context, username string) (store.User, error) { return user, nil }

	_, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
	require.ErrorContains(t, err, `unknown status "unknown"`)
}

func TestLoginPropagatesStoreErrors(t *testing.T) {
	boom := errors.New("boom")
	t.Run("rate", func(t *testing.T) {
		st := newAuthStubs()
		st.rate.err = boom
		_, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})
	t.Run("lock count", func(t *testing.T) {
		st := newAuthStubs()
		st.locks.countErr = boom
		_, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})
	t.Run("user lookup", func(t *testing.T) {
		st := newAuthStubs()
		st.users.byUsername = func(_ context.Context, username string) (store.User, error) { return store.User{}, boom }
		_, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})
	t.Run("session create", func(t *testing.T) {
		st := newAuthStubs()
		st.users.byUsername = func(_ context.Context, username string) (store.User, error) { return activeUser("alice"), nil }
		st.sessions.create = func(context.Context, int64) (store.Session, error) { return store.Session{}, boom }
		_, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})
	t.Run("touch", func(t *testing.T) {
		st := newAuthStubs()
		st.users.byUsername = func(_ context.Context, username string) (store.User, error) { return activeUser("alice"), nil }
		st.sessions = loginSessions()
		st.users.touchErr = boom
		_, err := st.service().Login(context.Background(), "alice", "Str0ng!Pass42", "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})
}

func TestLoginAuditFailureErrorsAreSwallowed(t *testing.T) {
	st := newAuthStubs()
	st.audits.err = errors.New("audit down")
	st.users.byUsername = func(_ context.Context, username string) (store.User, error) { return store.User{}, store.ErrUserNotFound }

	_, err := st.service().Login(context.Background(), "ghost", "Str0ng!Pass42", "10.0.0.1")
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogout(t *testing.T) {
	st := newAuthStubs()
	user := activeUser("alice")
	err := st.service().Logout(context.Background(), "sid-abc", user, "10.0.0.1")
	require.NoError(t, err)
	require.Equal(t, []string{"sid-abc"}, st.sessions.deleted)
	require.Len(t, st.audits.entries, 1)
	entry := st.audits.entries[0]
	require.Equal(t, "auth.logout", entry.Action)
	require.Equal(t, "sess:sid-abc", entry.Target)
	require.Equal(t, int64(7), *entry.ActorID)
	require.Equal(t, "alice", entry.ActorUsername)
}

func TestMe(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		st := newAuthStubs()
		user := activeUser("alice")
		st.sessions.getResult = store.Session{SID: "sid-abc", UserID: 7, CSRF: "csrf-xyz"}
		st.users.byID = func(_ context.Context, id int64) (store.User, error) { return user, nil }

		got, csrf, err := st.service().Me(context.Background(), "sid-abc")
		require.NoError(t, err)
		require.Equal(t, user, got)
		require.Equal(t, "csrf-xyz", csrf)
	})
	t.Run("session gone", func(t *testing.T) {
		st := newAuthStubs()
		st.sessions.getErr = store.ErrSessionNotFound
		_, _, err := st.service().Me(context.Background(), "sid-abc")
		require.ErrorIs(t, err, ErrSessionExpired)
	})
	t.Run("user gone", func(t *testing.T) {
		st := newAuthStubs()
		st.sessions.getResult = store.Session{SID: "sid-abc", UserID: 7, CSRF: "csrf-xyz"}
		_, _, err := st.service().Me(context.Background(), "sid-abc")
		require.ErrorIs(t, err, ErrSessionExpired)
	})
	t.Run("store error", func(t *testing.T) {
		st := newAuthStubs()
		st.sessions.getErr = fmt.Errorf("redis down: %w", errors.New("boom"))
		_, _, err := st.service().Me(context.Background(), "sid-abc")
		require.ErrorContains(t, err, "redis down")
	})
}
