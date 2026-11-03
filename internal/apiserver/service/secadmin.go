package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// Secadmin error kinds from the spec §4.5 table. KindNotFound and
// KindConflict are declared in users.go (shared within the package).
const (
	KindForbidden = "FORBIDDEN"
)

// Secadmin outcomes that the handlers map to §4.5 statuses. ErrNotFound is
// redeclared here as ErrSecadminNotFound because the shared ErrNotFound in
// users.go carries the uniform 用户不存在 message (spec §4.2 no-oracle
// rule) while secadmin's 404 per §4.3 is the generic 资源不存在.
var (
	ErrSecadminNotFound     = &Error{Kind: KindNotFound, Message: "资源不存在"}
	ErrAccountAlreadyActive = &Error{Kind: KindConflict, Message: "账号已是启用状态"}
)

// secadminPageSize is the fixed sessions page size (spec §11.3).
const secadminPageSize = 50

// SecUserStore is the users surface the secadmin service needs.
type SecUserStore interface {
	ByID(ctx context.Context, id int64) (store.User, error)
	Approve(ctx context.Context, id, approvedBy int64) error
}

// SecSessionStore is the sessions surface the secadmin service needs.
type SecSessionStore interface {
	Get(ctx context.Context, sid string) (store.Session, error)
	Delete(ctx context.Context, sid string) error
	ListAll(ctx context.Context) ([]store.Session, error)
}

// SecLockStore clears login lock counters.
type SecLockStore interface {
	Clear(ctx context.Context, username string) error
}

// SecadminService implements the §4.3 security operations.
type SecadminService struct {
	users    SecUserStore
	sessions SecSessionStore
	locks    SecLockStore
	audits   AuditStore
	logger   *slog.Logger
}

func NewSecadminService(users SecUserStore, sessions SecSessionStore, locks SecLockStore, audits AuditStore, logger *slog.Logger) *SecadminService {
	return &SecadminService{users: users, sessions: sessions, locks: locks, audits: audits, logger: logger}
}

// SessionListItem is one entry in a sessions listing; SID is a digest of the
// raw session id so the id is never exposed (spec §4.3).
type SessionListItem struct {
	SID        string
	UserID     int64
	CreatedAt  time.Time
	AbsExpires time.Time
}

// SessionPage is one page of a sessions listing.
type SessionPage struct {
	Items    []SessionListItem
	Page     int
	PageSize int
	Total    int
}

// sidDigest hashes the raw session id to a short hex string.
func sidDigest(sid string) string {
	sum := sha256.Sum256([]byte(sid))
	return hex.EncodeToString(sum[:8])
}

// Approve activates a pending or disabled user, returning ErrNotFound for a
// missing user and ErrAccountAlreadyActive for an already-active one. The
// store owns the status transition (spec §4.3 double-review path); the
// service maps its outcomes and writes the account.approve audit row.
func (s *SecadminService) Approve(ctx context.Context, id int64, actor store.User, ip string) error {
	if _, err := s.users.ByID(ctx, id); err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return ErrSecadminNotFound
		}
		return fmt.Errorf("get user %d: %w", id, err)
	}
	if err := s.users.Approve(ctx, id, actor.ID); err != nil {
		if errors.Is(err, store.ErrUserNotApprovable) {
			return ErrAccountAlreadyActive
		}
		return fmt.Errorf("approve user %d: %w", id, err)
	}
	s.audit(ctx, store.AuditEntry{
		ActorID:       &actor.ID,
		ActorUsername: actor.Username,
		ActorRole:     actor.Role,
		Action:        "account.approve",
		Target:        fmt.Sprintf("users:%d", id),
		Detail:        map[string]any{"to_status": store.StatusActive},
		IP:            ip,
	})
	return nil
}

// Unlock clears the login lock counter for the user's username and writes
// the account.unlock audit row. A missing user returns ErrNotFound.
func (s *SecadminService) Unlock(ctx context.Context, id int64, actor store.User, ip string) error {
	user, err := s.users.ByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return ErrSecadminNotFound
		}
		return fmt.Errorf("get user %d: %w", id, err)
	}
	if err := s.locks.Clear(ctx, user.Username); err != nil {
		return fmt.Errorf("clear lock for %q: %w", user.Username, err)
	}
	s.audit(ctx, store.AuditEntry{
		ActorID:       &actor.ID,
		ActorUsername: actor.Username,
		ActorRole:     actor.Role,
		Action:        "account.unlock",
		Target:        fmt.Sprintf("users:%d", id),
		IP:            ip,
	})
	return nil
}

// ForceLogout deletes one session and writes the session.force_logout audit
// row. An unknown sid returns ErrNotFound.
func (s *SecadminService) ForceLogout(ctx context.Context, sid string, actor store.User, ip string) error {
	if _, err := s.sessions.Get(ctx, sid); err != nil {
		if errors.Is(err, store.ErrSessionNotFound) {
			return ErrSecadminNotFound
		}
		return fmt.Errorf("get session %q: %w", sid, err)
	}
	if err := s.sessions.Delete(ctx, sid); err != nil {
		return fmt.Errorf("delete session %q: %w", sid, err)
	}
	s.audit(ctx, store.AuditEntry{
		ActorID:       &actor.ID,
		ActorUsername: actor.Username,
		ActorRole:     actor.Role,
		Action:        "session.force_logout",
		Target:        "sess:" + sid,
		IP:            ip,
	})
	return nil
}

// ListSessions enumerates sessions (all, or filtered by user_id), ordered by
// created_at descending (spec §11.3) with the fixed page size. SIDs are
// digests; CSRF values are never returned.
func (s *SecadminService) ListSessions(ctx context.Context, userID *int64, page int) (SessionPage, error) {
	all, err := s.sessions.ListAll(ctx)
	if err != nil {
		return SessionPage{}, fmt.Errorf("list sessions: %w", err)
	}
	if userID != nil {
		filtered := all[:0]
		for _, sess := range all {
			if sess.UserID == *userID {
				filtered = append(filtered, sess)
			}
		}
		all = filtered
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].SID > all[j].SID
		}
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	items := make([]SessionListItem, 0, len(all))
	for _, sess := range all {
		items = append(items, SessionListItem{
			SID:        sidDigest(sess.SID),
			UserID:     sess.UserID,
			CreatedAt:  sess.CreatedAt,
			AbsExpires: sess.AbsExpires,
		})
	}
	total := len(items)
	if page < 1 {
		page = 1
	}
	start := (page - 1) * secadminPageSize
	if start >= total {
		return SessionPage{Items: []SessionListItem{}, Page: page, PageSize: secadminPageSize, Total: total}, nil
	}
	end := start + secadminPageSize
	if end > total {
		end = total
	}
	return SessionPage{Items: items[start:end], Page: page, PageSize: secadminPageSize, Total: total}, nil
}

// audit writes one audit row best-effort; a failed write is logged, never
// surfaced, so a broken audit trail cannot mask the operation's outcome.
func (s *SecadminService) audit(ctx context.Context, e store.AuditEntry) {
	if err := s.audits.Log(ctx, e); err != nil {
		s.logger.Error("secadmin audit", "action", e.Action, "target", e.Target, "error", err)
	}
}
