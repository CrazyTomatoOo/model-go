package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Session TTL policy (login design spec §3.2): 30-minute sliding TTL per
// request, 12-hour absolute lifetime enforced by the session middleware.
const (
	SessionTTL         = 30 * time.Minute
	SessionAbsLifetime = 12 * time.Hour
)

// ErrSessionNotFound is returned when sess:{sid} holds no fields.
var ErrSessionNotFound = errors.New("session not found")

// Session is the server-side session record stored as a sess:{sid} hash.
type Session struct {
	SID        string
	UserID     int64
	CSRF       string
	CreatedAt  time.Time
	AbsExpires time.Time
}

// SessionStore manages sess:{sid} hashes in Redis.
type SessionStore struct {
	rdb *redis.Client
}

func NewSessionStore(rdb *redis.Client) *SessionStore { return &SessionStore{rdb: rdb} }

func sessionKey(sid string) string { return "sess:" + sid }

// Create draws a fresh 256-bit sid and csrf (no reuse: a new sid is drawn
// for every session), stores the hash, and sets the sliding TTL. Times are
// truncated to seconds so a Get round-trip returns an identical Session.
func (s *SessionStore) Create(ctx context.Context, userID int64) (Session, error) {
	sid, err := newToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate session id: %w", err)
	}
	csrf, err := newToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate csrf token: %w", err)
	}
	now := time.Now()
	sess := Session{
		SID:        sid,
		UserID:     userID,
		CSRF:       csrf,
		CreatedAt:  time.Unix(now.Unix(), 0),
		AbsExpires: time.Unix(now.Add(SessionAbsLifetime).Unix(), 0),
	}
	if err := s.rdb.HSet(ctx, sessionKey(sid), map[string]any{
		"user_id":     sess.UserID,
		"csrf":        sess.CSRF,
		"created_at":  sess.CreatedAt.Unix(),
		"abs_expires": sess.AbsExpires.Unix(),
	}).Err(); err != nil {
		return Session{}, fmt.Errorf("store session: %w", err)
	}
	if err := s.rdb.Expire(ctx, sessionKey(sid), SessionTTL).Err(); err != nil {
		return Session{}, fmt.Errorf("set session ttl: %w", err)
	}
	return sess, nil
}

// Get returns the session for sid, or ErrSessionNotFound when absent.
// Malformed or missing fields are treated as not-found (fail-closed).
func (s *SessionStore) Get(ctx context.Context, sid string) (Session, error) {
	fields, err := s.rdb.HGetAll(ctx, sessionKey(sid)).Result()
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	if len(fields) == 0 {
		return Session{}, ErrSessionNotFound
	}
	userID, err := strconv.ParseInt(fields["user_id"], 10, 64)
	if err != nil {
		return Session{}, fmt.Errorf("session %s: malformed user_id", sid)
	}
	created, err := strconv.ParseInt(fields["created_at"], 10, 64)
	if err != nil {
		return Session{}, fmt.Errorf("session %s: malformed created_at", sid)
	}
	abs, err := strconv.ParseInt(fields["abs_expires"], 10, 64)
	if err != nil {
		return Session{}, fmt.Errorf("session %s: malformed abs_expires", sid)
	}
	if fields["csrf"] == "" {
		return Session{}, fmt.Errorf("session %s: missing csrf", sid)
	}
	return Session{
		SID:        sid,
		UserID:     userID,
		CSRF:       fields["csrf"],
		CreatedAt:  time.Unix(created, 0),
		AbsExpires: time.Unix(abs, 0),
	}, nil
}

// Delete removes the session.
func (s *SessionStore) Delete(ctx context.Context, sid string) error {
	if err := s.rdb.Del(ctx, sessionKey(sid)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Touch refreshes the sliding TTL (per-request activity touch, §3.2).
func (s *SessionStore) Touch(ctx context.Context, sid string) error {
	if err := s.rdb.Expire(ctx, sessionKey(sid), SessionTTL).Err(); err != nil {
		return fmt.Errorf("touch session ttl: %w", err)
	}
	return nil
}

// newToken returns 32 random bytes in unpadded base64url form (43 chars,
// within the spec's 64-char session id budget).
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
