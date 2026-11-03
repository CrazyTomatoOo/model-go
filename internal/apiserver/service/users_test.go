package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
)

type stubUserAdmin struct {
	byID         func(context.Context, int64) (store.User, error)
	created      []createCall
	createErr    error
	history      []string
	historyErr   error
	historyCalls int
	resetCalls   []resetCall
	resetErr     error
	disableCalls []int64
	disableErr   error
}

type createCall struct {
	username     string
	passwordHash string
	role         string
	createdBy    int64
}

type resetCall struct {
	userID  int64
	newHash string
}

func (s *stubUserAdmin) ByID(ctx context.Context, id int64) (store.User, error) {
	return s.byID(ctx, id)
}

func (s *stubUserAdmin) Create(ctx context.Context, username, passwordHash, role string, createdBy int64) (int64, error) {
	s.created = append(s.created, createCall{username: username, passwordHash: passwordHash, role: role, createdBy: createdBy})
	return 42, s.createErr
}

func (s *stubUserAdmin) PasswordHistory(ctx context.Context, userID int64) ([]string, error) {
	s.historyCalls++
	return s.history, s.historyErr
}

func (s *stubUserAdmin) ResetPassword(ctx context.Context, userID int64, newHash string) error {
	s.resetCalls = append(s.resetCalls, resetCall{userID: userID, newHash: newHash})
	return s.resetErr
}

func (s *stubUserAdmin) Disable(ctx context.Context, userID int64) error {
	s.disableCalls = append(s.disableCalls, userID)
	return s.disableErr
}

type stubSessionKick struct {
	kicked []int64
	err    error
}

func (s *stubSessionKick) DeleteByUserID(ctx context.Context, userID int64) (int, error) {
	s.kicked = append(s.kicked, userID)
	return len(s.kicked), s.err
}

func sysadminActor() store.User {
	return store.User{ID: 1, Username: "bob", Role: store.RoleSysadmin, Status: store.StatusActive}
}

func newUsersStubs() (*stubUserAdmin, *stubSessionKick, *stubAudit) {
	users := &stubUserAdmin{byID: func(_ context.Context, id int64) (store.User, error) {
		return store.User{}, store.ErrUserNotFound
	}}
	return users, &stubSessionKick{}, &stubAudit{}
}

func newUsersService(users *stubUserAdmin, sessions *stubSessionKick, audits *stubAudit) *UsersService {
	return NewUsersService(users, sessions, audits, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestUsersCreateSuccess(t *testing.T) {
	users, sessions, audits := newUsersStubs()
	svc := newUsersService(users, sessions, audits)

	got, err := svc.Create(context.Background(), sysadminActor(),
		CreateUserInput{Username: "carol", Password: "Str0ng!Pass42", Role: store.RoleUser}, "10.0.0.1")
	require.NoError(t, err)
	require.Equal(t, CreatedUser{ID: 42, Status: store.StatusPending}, got)
	require.Len(t, users.created, 1)
	call := users.created[0]
	require.Equal(t, "carol", call.username)
	require.Equal(t, store.RoleUser, call.role)
	require.Equal(t, int64(1), call.createdBy)
	ok, err := password.Verify("Str0ng!Pass42", call.passwordHash)
	require.NoError(t, err)
	require.True(t, ok, "service must hash the password before storing")
	require.Empty(t, sessions.kicked)

	require.Len(t, audits.entries, 1)
	e := audits.entries[0]
	require.Equal(t, "account.create", e.Action)
	require.Equal(t, int64(1), *e.ActorID)
	require.Equal(t, "bob", e.ActorUsername)
	require.Equal(t, store.RoleSysadmin, e.ActorRole)
	require.Equal(t, "users:42", e.Target)
	require.Equal(t, "10.0.0.1", e.IP)
}

func TestUsersCreateValidation(t *testing.T) {
	cases := []struct {
		name  string
		in    CreateUserInput
		field string
	}{
		{"empty username", CreateUserInput{Username: "", Password: "Str0ng!Pass42", Role: store.RoleUser}, "username"},
		{"invalid role", CreateUserInput{Username: "carol", Password: "Str0ng!Pass42", Role: "superuser"}, "role"},
		{"empty password", CreateUserInput{Username: "carol", Password: "", Role: store.RoleUser}, "password"},
		{"too short", CreateUserInput{Username: "carol", Password: "Short1!", Role: store.RoleUser}, "password"},
		{"contains username", CreateUserInput{Username: "carol", Password: "carolSecret12!a", Role: store.RoleUser}, "password"},
		{"common password", CreateUserInput{Username: "carol", Password: "password123456", Role: store.RoleUser}, "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users, sessions, audits := newUsersStubs()
			svc := newUsersService(users, sessions, audits)

			_, err := svc.Create(context.Background(), sysadminActor(), tc.in, "10.0.0.1")
			require.Error(t, err)
			var serr *Error
			require.ErrorAs(t, err, &serr)
			require.Equal(t, KindValidation, serr.Kind)
			require.Equal(t, tc.field, serr.Field)
			require.Empty(t, users.created)
			require.Empty(t, sessions.kicked)
			require.Empty(t, audits.entries)
		})
	}
}

func TestUsersCreateDuplicateUsername(t *testing.T) {
	users, sessions, audits := newUsersStubs()
	users.createErr = store.ErrUsernameTaken
	svc := newUsersService(users, sessions, audits)

	_, err := svc.Create(context.Background(), sysadminActor(),
		CreateUserInput{Username: "carol", Password: "Str0ng!Pass42", Role: store.RoleUser}, "10.0.0.1")
	require.ErrorIs(t, err, ErrUsernameTaken)
	require.Empty(t, audits.entries)
	require.Empty(t, sessions.kicked)
}

func TestUsersCreatePropagatesStoreErrors(t *testing.T) {
	boom := errors.New("boom")
	users, _, _ := newUsersStubs()
	users.createErr = boom

	_, err := newUsersService(users, &stubSessionKick{}, &stubAudit{}).Create(context.Background(), sysadminActor(),
		CreateUserInput{Username: "carol", Password: "Str0ng!Pass42", Role: store.RoleUser}, "10.0.0.1")
	require.ErrorIs(t, err, boom)
}

func TestUsersCreateAuditFailureIsSwallowed(t *testing.T) {
	users, sessions, audits := newUsersStubs()
	audits.err = errors.New("audit down")

	got, err := newUsersService(users, sessions, audits).Create(context.Background(), sysadminActor(),
		CreateUserInput{Username: "carol", Password: "Str0ng!Pass42", Role: store.RoleUser}, "10.0.0.1")
	require.NoError(t, err)
	require.Equal(t, int64(42), got.ID)
}

func TestUsersResetPasswordSuccess(t *testing.T) {
	users, sessions, audits := newUsersStubs()
	users.byID = func(_ context.Context, id int64) (store.User, error) { return activeUser("alice"), nil }
	svc := newUsersService(users, sessions, audits)

	err := svc.ResetPassword(context.Background(), sysadminActor(), 7, "Brand-New-Pass-2026!", "10.0.0.2")
	require.NoError(t, err)
	require.Equal(t, 1, users.historyCalls, "history must be read for the reuse check")
	require.Len(t, users.resetCalls, 1)
	require.Equal(t, int64(7), users.resetCalls[0].userID)
	ok, err := password.Verify("Brand-New-Pass-2026!", users.resetCalls[0].newHash)
	require.NoError(t, err)
	require.True(t, ok, "service must hash the new password before storing")
	require.Equal(t, []int64{7}, sessions.kicked, "all of the user's sessions must be kicked")

	require.Len(t, audits.entries, 1)
	e := audits.entries[0]
	require.Equal(t, "auth.password_reset", e.Action)
	require.Equal(t, int64(1), *e.ActorID)
	require.Equal(t, "bob", e.ActorUsername)
	require.Equal(t, "users:7", e.Target)
	require.Equal(t, "10.0.0.2", e.IP)
}

func TestUsersResetPasswordRejectsReuse(t *testing.T) {
	currentHash, err := password.Hash("Current-Pass-2026!")
	require.NoError(t, err)
	histHash, err := password.Hash("Old-Pass-2025!")
	require.NoError(t, err)

	cases := []struct {
		name      string
		candidate string
		history   []string
	}{
		{"same as current", "Current-Pass-2026!", nil},
		{"in history", "Old-Pass-2025!", []string{histHash}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users, sessions, audits := newUsersStubs()
			user := activeUser("alice")
			user.PasswordHash = currentHash
			users.byID = func(_ context.Context, id int64) (store.User, error) { return user, nil }
			users.history = tc.history
			svc := newUsersService(users, sessions, audits)

			err := svc.ResetPassword(context.Background(), sysadminActor(), 7, tc.candidate, "10.0.0.2")
			require.Error(t, err)
			var serr *Error
			require.ErrorAs(t, err, &serr)
			require.Equal(t, KindValidation, serr.Kind)
			require.Equal(t, "password", serr.Field)
			require.Empty(t, users.resetCalls)
			require.Empty(t, sessions.kicked)
			require.Empty(t, audits.entries)
		})
	}
}

func TestUsersResetPasswordPolicyFailure(t *testing.T) {
	users, sessions, audits := newUsersStubs()
	users.byID = func(_ context.Context, id int64) (store.User, error) { return activeUser("alice"), nil }
	svc := newUsersService(users, sessions, audits)

	err := svc.ResetPassword(context.Background(), sysadminActor(), 7, "short", "10.0.0.2")
	require.Error(t, err)
	var serr *Error
	require.ErrorAs(t, err, &serr)
	require.Equal(t, KindValidation, serr.Kind)
	require.Equal(t, "password", serr.Field)
	require.Empty(t, users.resetCalls)
	require.Empty(t, sessions.kicked)
	require.Empty(t, audits.entries)
}

func TestUsersResetPasswordUnknownUser(t *testing.T) {
	users, sessions, audits := newUsersStubs()
	svc := newUsersService(users, sessions, audits)

	err := svc.ResetPassword(context.Background(), sysadminActor(), 999, "Brand-New-Pass-2026!", "10.0.0.2")
	require.ErrorIs(t, err, ErrNotFound)
	require.Empty(t, sessions.kicked)
	require.Empty(t, audits.entries)
}

func TestUsersResetPasswordUserVanishedBetweenLookupAndReset(t *testing.T) {
	users, sessions, audits := newUsersStubs()
	users.byID = func(_ context.Context, id int64) (store.User, error) { return activeUser("alice"), nil }
	users.resetErr = store.ErrUserNotFound
	svc := newUsersService(users, sessions, audits)

	err := svc.ResetPassword(context.Background(), sysadminActor(), 7, "Brand-New-Pass-2026!", "10.0.0.2")
	require.ErrorIs(t, err, ErrNotFound)
	require.Empty(t, sessions.kicked)
}

func TestUsersResetPasswordPropagatesStoreErrors(t *testing.T) {
	boom := errors.New("boom")
	t.Run("lookup", func(t *testing.T) {
		users, sessions, audits := newUsersStubs()
		users.byID = func(context.Context, int64) (store.User, error) { return store.User{}, boom }
		err := newUsersService(users, sessions, audits).ResetPassword(context.Background(), sysadminActor(), 7, "Brand-New-Pass-2026!", "10.0.0.2")
		require.ErrorIs(t, err, boom)
	})
	t.Run("history", func(t *testing.T) {
		users, sessions, audits := newUsersStubs()
		users.byID = func(_ context.Context, id int64) (store.User, error) { return activeUser("alice"), nil }
		users.historyErr = boom
		err := newUsersService(users, sessions, audits).ResetPassword(context.Background(), sysadminActor(), 7, "Brand-New-Pass-2026!", "10.0.0.2")
		require.ErrorIs(t, err, boom)
		require.Empty(t, users.resetCalls)
	})
	t.Run("reset", func(t *testing.T) {
		users, sessions, audits := newUsersStubs()
		users.byID = func(_ context.Context, id int64) (store.User, error) { return activeUser("alice"), nil }
		users.resetErr = boom
		err := newUsersService(users, sessions, audits).ResetPassword(context.Background(), sysadminActor(), 7, "Brand-New-Pass-2026!", "10.0.0.2")
		require.ErrorIs(t, err, boom)
	})
	t.Run("kick", func(t *testing.T) {
		users, sessions, audits := newUsersStubs()
		users.byID = func(_ context.Context, id int64) (store.User, error) { return activeUser("alice"), nil }
		sessions.err = boom
		err := newUsersService(users, sessions, audits).ResetPassword(context.Background(), sysadminActor(), 7, "Brand-New-Pass-2026!", "10.0.0.2")
		require.ErrorIs(t, err, boom)
	})
}

func TestUsersDisableSuccess(t *testing.T) {
	users, sessions, audits := newUsersStubs()
	svc := newUsersService(users, sessions, audits)

	err := svc.Disable(context.Background(), sysadminActor(), 7, "10.0.0.3")
	require.NoError(t, err)
	require.Equal(t, []int64{7}, users.disableCalls)
	require.Equal(t, []int64{7}, sessions.kicked, "all of the user's sessions must be kicked")

	require.Len(t, audits.entries, 1)
	e := audits.entries[0]
	require.Equal(t, "account.disable", e.Action)
	require.Equal(t, int64(1), *e.ActorID)
	require.Equal(t, "bob", e.ActorUsername)
	require.Equal(t, "users:7", e.Target)
	require.Equal(t, "10.0.0.3", e.IP)
}

func TestUsersDisableUnknownUser(t *testing.T) {
	users, sessions, audits := newUsersStubs()
	users.disableErr = store.ErrUserNotFound
	svc := newUsersService(users, sessions, audits)

	err := svc.Disable(context.Background(), sysadminActor(), 999, "10.0.0.3")
	require.ErrorIs(t, err, ErrNotFound)
	require.Empty(t, sessions.kicked)
	require.Empty(t, audits.entries)
}

func TestUsersDisablePropagatesStoreErrors(t *testing.T) {
	boom := errors.New("boom")
	t.Run("disable", func(t *testing.T) {
		users, sessions, audits := newUsersStubs()
		users.disableErr = boom
		err := newUsersService(users, sessions, audits).Disable(context.Background(), sysadminActor(), 7, "10.0.0.3")
		require.ErrorIs(t, err, boom)
		require.Empty(t, sessions.kicked)
	})
	t.Run("kick", func(t *testing.T) {
		users, sessions, audits := newUsersStubs()
		sessions.err = boom
		err := newUsersService(users, sessions, audits).Disable(context.Background(), sysadminActor(), 7, "10.0.0.3")
		require.ErrorIs(t, err, boom)
	})
}
