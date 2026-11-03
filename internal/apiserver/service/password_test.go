package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/password"
)

func TestChangePasswordWrongOld(t *testing.T) {
	st := newAuthStubs()
	user := activeUser("alice")

	err := st.service().ChangePassword(context.Background(), "sid-current", user, "Wrong!Pass42", "New-Strong-Passw0rd-1!", "10.0.0.1")
	require.ErrorIs(t, err, ErrWrongOldPassword)
	require.Empty(t, st.users.history)
	require.Empty(t, st.users.changeCalls)
	require.Empty(t, st.sessions.deleted)
	require.Empty(t, st.audits.entries)
}

func TestChangePasswordPolicyRejects(t *testing.T) {
	user := activeUser("alice")
	cases := []struct {
		name    string
		newPass string
		wantMsg string
	}{
		{"too short", "short", "至少需要 16 位"},
		{"contains username", "alice-secret-1234", "不得与用户名相同或包含用户名"},
		{"too common", "password123456", "密码过于常见"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := newAuthStubs()
			err := st.service().ChangePassword(context.Background(), "sid-current", user, "Str0ng!Pass42", tc.newPass, "10.0.0.1")
			var serr *Error
			require.ErrorAs(t, err, &serr)
			require.Equal(t, KindValidationError, serr.Kind)
			require.Equal(t, "new_password", serr.Field)
			require.Contains(t, serr.Message, tc.wantMsg)
			require.Empty(t, st.users.history)
			require.Empty(t, st.users.changeCalls)
		})
	}
}

func TestChangePasswordReuseRejected(t *testing.T) {
	t.Run("same as current", func(t *testing.T) {
		st := newAuthStubs()
		user := activeUser("alice")
		err := st.service().ChangePassword(context.Background(), "sid-current", user, "Str0ng!Pass42", "Str0ng!Pass42", "10.0.0.1")
		require.ErrorIs(t, err, ErrPasswordReused)
	})
	t.Run("in history", func(t *testing.T) {
		st := newAuthStubs()
		user := activeUser("alice")
		oldHash, err := password.Hash("Old-Strong-Passw0rd-1!")
		require.NoError(t, err)
		st.users.history = []string{oldHash}

		err = st.service().ChangePassword(context.Background(), "sid-current", user, "Str0ng!Pass42", "Old-Strong-Passw0rd-1!", "10.0.0.1")
		require.ErrorIs(t, err, ErrPasswordReused)
		require.Empty(t, st.users.changeCalls)
		require.Empty(t, st.sessions.deleted)
	})
}

func TestChangePasswordSuccess(t *testing.T) {
	st := newAuthStubs()
	user := activeUser("alice")
	historyHash, err := password.Hash("Old-Strong-Passw0rd-1!")
	require.NoError(t, err)
	st.users.history = []string{historyHash}
	st.sessions.listByUser = func(_ context.Context, userID int64) ([]string, error) {
		require.Equal(t, int64(7), userID)
		return []string{"sid-other", "sid-current"}, nil
	}

	err = st.service().ChangePassword(context.Background(), "sid-current", user, "Str0ng!Pass42", "New-Strong-Passw0rd-2!", "10.0.0.1")
	require.NoError(t, err)

	require.Len(t, st.users.changeCalls, 1)
	call := st.users.changeCalls[0]
	require.Equal(t, int64(7), call.userID)
	require.Equal(t, testUserHash, call.oldHash)
	match, err := password.Verify("New-Strong-Passw0rd-2!", call.newHash)
	require.NoError(t, err)
	require.True(t, match)

	require.Equal(t, []string{"sid-other"}, st.sessions.deleted)

	require.Len(t, st.audits.entries, 1)
	entry := st.audits.entries[0]
	require.Equal(t, "auth.password_change", entry.Action)
	require.Equal(t, "users:7", entry.Target)
	require.Equal(t, int64(7), *entry.ActorID)
	require.Equal(t, "alice", entry.ActorUsername)
	require.Equal(t, "10.0.0.1", entry.IP)
}

func TestChangePasswordStoreErrorsPropagate(t *testing.T) {
	boom := errors.New("boom")
	user := activeUser("alice")

	t.Run("history read", func(t *testing.T) {
		st := newAuthStubs()
		st.users.historyErr = boom
		err := st.service().ChangePassword(context.Background(), "sid-current", user, "Str0ng!Pass42", "New-Strong-Passw0rd-2!", "10.0.0.1")
		require.ErrorIs(t, err, boom)
		require.Empty(t, st.users.changeCalls)
	})
	t.Run("change write", func(t *testing.T) {
		st := newAuthStubs()
		st.users.changeErr = boom
		err := st.service().ChangePassword(context.Background(), "sid-current", user, "Str0ng!Pass42", "New-Strong-Passw0rd-2!", "10.0.0.1")
		require.ErrorIs(t, err, boom)
		require.Empty(t, st.audits.entries)
	})
	t.Run("session list is best effort", func(t *testing.T) {
		st := newAuthStubs()
		st.sessions.listByUser = func(context.Context, int64) ([]string, error) { return nil, boom }
		err := st.service().ChangePassword(context.Background(), "sid-current", user, "Str0ng!Pass42", "New-Strong-Passw0rd-2!", "10.0.0.1")
		require.NoError(t, err)
		require.Len(t, st.audits.entries, 1)
	})
}
