package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

type secUserStub struct {
	byID    func(context.Context, int64) (store.User, error)
	approve func(context.Context, int64, int64) error
}

func (s *secUserStub) ByID(ctx context.Context, id int64) (store.User, error) { return s.byID(ctx, id) }
func (s *secUserStub) Approve(ctx context.Context, id, approvedBy int64) error {
	return s.approve(ctx, id, approvedBy)
}

type secSessionStub struct {
	get     func(context.Context, string) (store.Session, error)
	delete  func(context.Context, string) error
	listAll func(context.Context) ([]store.Session, error)
}

func (s *secSessionStub) Get(ctx context.Context, sid string) (store.Session, error) {
	return s.get(ctx, sid)
}
func (s *secSessionStub) Delete(ctx context.Context, sid string) error { return s.delete(ctx, sid) }
func (s *secSessionStub) ListAll(ctx context.Context) ([]store.Session, error) {
	return s.listAll(ctx)
}

type secLockStub struct {
	cleared []string
	err     error
}

func (s *secLockStub) Clear(_ context.Context, username string) error {
	s.cleared = append(s.cleared, username)
	return s.err
}

type secAuditStub struct {
	entries []store.AuditEntry
	err     error
}

func (s *secAuditStub) Log(_ context.Context, e store.AuditEntry) error {
	s.entries = append(s.entries, e)
	return s.err
}

type secStubs struct {
	users    *secUserStub
	sessions *secSessionStub
	locks    *secLockStub
	audits   *secAuditStub
}

func newSecStubs() *secStubs {
	return &secStubs{
		users: &secUserStub{
			byID: func(context.Context, int64) (store.User, error) {
				return store.User{}, store.ErrUserNotFound
			},
			approve: func(context.Context, int64, int64) error { return nil },
		},
		sessions: &secSessionStub{
			get: func(context.Context, string) (store.Session, error) {
				return store.Session{}, store.ErrSessionNotFound
			},
			delete:  func(context.Context, string) error { return nil },
			listAll: func(context.Context) ([]store.Session, error) { return nil, nil },
		},
		locks:  &secLockStub{},
		audits: &secAuditStub{},
	}
}

func (s *secStubs) service() *SecadminService {
	return NewSecadminService(s.users, s.sessions, s.locks, s.audits,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func secActor() store.User {
	return store.User{ID: 11, Username: "secadmin", Role: store.RoleSecadmin, Status: store.StatusActive}
}

func secAuditEntry(t *testing.T, st *secStubs) store.AuditEntry {
	t.Helper()
	require.Len(t, st.audits.entries, 1)
	return st.audits.entries[0]
}

func TestSecadminApprove(t *testing.T) {
	t.Run("pending user approved", func(t *testing.T) {
		st := newSecStubs()
		var gotID, gotBy int64
		st.users.byID = func(_ context.Context, id int64) (store.User, error) {
			return store.User{ID: id, Username: "alice", Role: store.RoleUser, Status: store.StatusPending}, nil
		}
		st.users.approve = func(_ context.Context, id, approvedBy int64) error {
			gotID, gotBy = id, approvedBy
			return nil
		}
		err := st.service().Approve(context.Background(), 42, secActor(), "10.0.0.1")
		require.NoError(t, err)
		require.Equal(t, int64(42), gotID)
		require.Equal(t, int64(11), gotBy)
		e := secAuditEntry(t, st)
		require.Equal(t, "account.approve", e.Action)
		require.Equal(t, "users:42", e.Target)
		require.NotNil(t, e.ActorID)
		require.Equal(t, int64(11), *e.ActorID)
		require.Equal(t, "secadmin", e.ActorUsername)
		require.Equal(t, "10.0.0.1", e.IP)
	})

	t.Run("disabled user re-enabled", func(t *testing.T) {
		st := newSecStubs()
		st.users.byID = func(_ context.Context, id int64) (store.User, error) {
			return store.User{ID: id, Username: "alice", Role: store.RoleUser, Status: store.StatusDisabled}, nil
		}
		st.users.approve = func(_ context.Context, id, approvedBy int64) error { return nil }
		err := st.service().Approve(context.Background(), 42, secActor(), "10.0.0.1")
		require.NoError(t, err)
		require.Equal(t, "account.approve", secAuditEntry(t, st).Action)
	})

	t.Run("already active conflicts", func(t *testing.T) {
		st := newSecStubs()
		st.users.byID = func(_ context.Context, id int64) (store.User, error) {
			return store.User{ID: id, Username: "alice", Role: store.RoleUser, Status: store.StatusActive}, nil
		}
		st.users.approve = func(context.Context, int64, int64) error { return store.ErrUserNotApprovable }
		err := st.service().Approve(context.Background(), 42, secActor(), "10.0.0.1")
		require.ErrorIs(t, err, ErrAccountAlreadyActive)
		require.Empty(t, st.audits.entries)
	})

	t.Run("unknown user not found", func(t *testing.T) {
		st := newSecStubs()
		err := st.service().Approve(context.Background(), 999, secActor(), "10.0.0.1")
		require.ErrorIs(t, err, ErrSecadminNotFound)
		require.Empty(t, st.audits.entries)
	})

	t.Run("store error propagates", func(t *testing.T) {
		st := newSecStubs()
		st.users.byID = func(_ context.Context, id int64) (store.User, error) {
			return store.User{ID: id, Username: "alice"}, nil
		}
		boom := errors.New("boom")
		st.users.approve = func(context.Context, int64, int64) error { return boom }
		err := st.service().Approve(context.Background(), 42, secActor(), "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})

	t.Run("byID error propagates", func(t *testing.T) {
		st := newSecStubs()
		boom := errors.New("db down")
		st.users.byID = func(context.Context, int64) (store.User, error) { return store.User{}, boom }
		err := st.service().Approve(context.Background(), 42, secActor(), "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})

	t.Run("audit failure does not fail request", func(t *testing.T) {
		st := newSecStubs()
		st.users.byID = func(_ context.Context, id int64) (store.User, error) {
			return store.User{ID: id, Username: "alice", Status: store.StatusPending}, nil
		}
		st.audits.err = errors.New("audit down")
		err := st.service().Approve(context.Background(), 42, secActor(), "10.0.0.1")
		require.NoError(t, err)
	})
}

func TestSecadminUnlock(t *testing.T) {
	t.Run("clears lock and audits", func(t *testing.T) {
		st := newSecStubs()
		st.users.byID = func(_ context.Context, id int64) (store.User, error) {
			return store.User{ID: id, Username: "alice", Role: store.RoleUser, Status: store.StatusActive}, nil
		}
		err := st.service().Unlock(context.Background(), 42, secActor(), "10.0.0.1")
		require.NoError(t, err)
		require.Equal(t, []string{"alice"}, st.locks.cleared)
		e := secAuditEntry(t, st)
		require.Equal(t, "account.unlock", e.Action)
		require.Equal(t, "users:42", e.Target)
		require.Equal(t, int64(11), *e.ActorID)
	})

	t.Run("unknown user not found", func(t *testing.T) {
		st := newSecStubs()
		err := st.service().Unlock(context.Background(), 999, secActor(), "10.0.0.1")
		require.ErrorIs(t, err, ErrSecadminNotFound)
		require.Empty(t, st.locks.cleared)
		require.Empty(t, st.audits.entries)
	})

	t.Run("byID error propagates", func(t *testing.T) {
		st := newSecStubs()
		boom := errors.New("boom")
		st.users.byID = func(context.Context, int64) (store.User, error) { return store.User{}, boom }
		err := st.service().Unlock(context.Background(), 42, secActor(), "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})

	t.Run("clear error propagates", func(t *testing.T) {
		st := newSecStubs()
		st.users.byID = func(_ context.Context, id int64) (store.User, error) {
			return store.User{ID: id, Username: "alice"}, nil
		}
		boom := errors.New("redis down")
		st.locks.err = boom
		err := st.service().Unlock(context.Background(), 42, secActor(), "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})
}

func TestSecadminForceLogout(t *testing.T) {
	t.Run("deletes session and audits", func(t *testing.T) {
		st := newSecStubs()
		st.sessions.get = func(_ context.Context, sid string) (store.Session, error) {
			return store.Session{SID: sid, UserID: 7, CSRF: "csrf", CreatedAt: time.Now(), AbsExpires: time.Now().Add(time.Hour)}, nil
		}
		var deleted []string
		st.sessions.delete = func(_ context.Context, sid string) error { deleted = append(deleted, sid); return nil }
		err := st.service().ForceLogout(context.Background(), "sid-abc", secActor(), "10.0.0.1")
		require.NoError(t, err)
		require.Equal(t, []string{"sid-abc"}, deleted)
		e := secAuditEntry(t, st)
		require.Equal(t, "session.force_logout", e.Action)
		require.Equal(t, "sess:sid-abc", e.Target)
		require.Equal(t, int64(11), *e.ActorID)
	})

	t.Run("unknown sid not found", func(t *testing.T) {
		st := newSecStubs()
		err := st.service().ForceLogout(context.Background(), "sid-xyz", secActor(), "10.0.0.1")
		require.ErrorIs(t, err, ErrSecadminNotFound)
		require.Empty(t, st.audits.entries)
	})

	t.Run("get error propagates", func(t *testing.T) {
		st := newSecStubs()
		boom := errors.New("redis down")
		st.sessions.get = func(context.Context, string) (store.Session, error) { return store.Session{}, boom }
		err := st.service().ForceLogout(context.Background(), "sid-abc", secActor(), "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})

	t.Run("delete error propagates", func(t *testing.T) {
		st := newSecStubs()
		st.sessions.get = func(_ context.Context, sid string) (store.Session, error) {
			return store.Session{SID: sid}, nil
		}
		boom := errors.New("redis down")
		st.sessions.delete = func(context.Context, string) error { return boom }
		err := st.service().ForceLogout(context.Background(), "sid-abc", secActor(), "10.0.0.1")
		require.ErrorIs(t, err, boom)
	})

	t.Run("audit failure does not fail request", func(t *testing.T) {
		st := newSecStubs()
		st.sessions.get = func(_ context.Context, sid string) (store.Session, error) {
			return store.Session{SID: sid}, nil
		}
		st.sessions.delete = func(context.Context, string) error { return nil }
		st.audits.err = errors.New("audit down")
		err := st.service().ForceLogout(context.Background(), "sid-abc", secActor(), "10.0.0.1")
		require.NoError(t, err)
	})
}

func secSessions(n int) []store.Session {
	base := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	out := make([]store.Session, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, store.Session{
			SID:        "sid-" + string(rune('a'+i%26)) + string(rune('0'+i%10)),
			UserID:     int64(1 + i%3),
			CSRF:       "csrf",
			CreatedAt:  base.Add(-time.Duration(i) * time.Minute),
			AbsExpires: base.Add(-time.Duration(i)*time.Minute + time.Hour),
		})
	}
	return out
}

func TestSecadminListSessions(t *testing.T) {
	hexSID := func(t *testing.T, s string) {
		t.Helper()
		require.Regexp(t, `^[0-9a-f]{16}$`, s, "sid must be a 16-hex digest, never the raw id")
	}

	t.Run("filters by user_id", func(t *testing.T) {
		st := newSecStubs()
		st.sessions.listAll = func(context.Context) ([]store.Session, error) { return secSessions(6), nil }
		uid := int64(2)
		page, err := st.service().ListSessions(context.Background(), &uid, 1)
		require.NoError(t, err)
		require.Equal(t, 2, page.Total)
		require.Len(t, page.Items, 2)
		for _, it := range page.Items {
			require.Equal(t, int64(2), it.UserID)
			hexSID(t, it.SID)
		}
	})

	t.Run("sorts by created_at desc and never leaks raw sid", func(t *testing.T) {
		st := newSecStubs()
		now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
		st.sessions.listAll = func(context.Context) ([]store.Session, error) {
			return []store.Session{
				{SID: "old", UserID: 1, CreatedAt: now.Add(-2 * time.Hour), AbsExpires: now},
				{SID: "new", UserID: 1, CreatedAt: now, AbsExpires: now.Add(time.Hour)},
				{SID: "mid", UserID: 1, CreatedAt: now.Add(-time.Hour), AbsExpires: now.Add(time.Hour)},
			}, nil
		}
		page, err := st.service().ListSessions(context.Background(), nil, 1)
		require.NoError(t, err)
		require.Len(t, page.Items, 3)
		for i := 1; i < len(page.Items); i++ {
			require.False(t, page.Items[i].CreatedAt.After(page.Items[i-1].CreatedAt),
				"items must be ordered by created_at desc")
			hexSID(t, page.Items[i].SID)
		}
		hexSID(t, page.Items[0].SID)
	})

	t.Run("paginates 55 sessions", func(t *testing.T) {
		st := newSecStubs()
		st.sessions.listAll = func(context.Context) ([]store.Session, error) { return secSessions(55), nil }
		p1, err := st.service().ListSessions(context.Background(), nil, 1)
		require.NoError(t, err)
		require.Equal(t, 1, p1.Page)
		require.Equal(t, 50, p1.PageSize)
		require.Equal(t, 55, p1.Total)
		require.Len(t, p1.Items, 50)
		p2, err := st.service().ListSessions(context.Background(), nil, 2)
		require.NoError(t, err)
		require.Len(t, p2.Items, 5)
		require.Equal(t, 55, p2.Total)
	})

	t.Run("page beyond range returns empty", func(t *testing.T) {
		st := newSecStubs()
		st.sessions.listAll = func(context.Context) ([]store.Session, error) { return secSessions(55), nil }
		p3, err := st.service().ListSessions(context.Background(), nil, 3)
		require.NoError(t, err)
		require.Empty(t, p3.Items)
		require.Equal(t, 55, p3.Total)
	})

	t.Run("no filter returns all", func(t *testing.T) {
		st := newSecStubs()
		st.sessions.listAll = func(context.Context) ([]store.Session, error) { return secSessions(3), nil }
		page, err := st.service().ListSessions(context.Background(), nil, 1)
		require.NoError(t, err)
		require.Equal(t, 3, page.Total)
		require.Len(t, page.Items, 3)
	})

	t.Run("listAll error propagates", func(t *testing.T) {
		st := newSecStubs()
		boom := errors.New("redis down")
		st.sessions.listAll = func(context.Context) ([]store.Session, error) { return nil, boom }
		_, err := st.service().ListSessions(context.Background(), nil, 1)
		require.ErrorIs(t, err, boom)
	})
}
