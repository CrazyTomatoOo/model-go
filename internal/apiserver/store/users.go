// Package store implements the persistence layer for the login feature
// (login design spec §3): Postgres users/audit_logs via pgx and Redis
// sessions/locks via go-redis. The four files mirror the spec's stores:
// users.go + audit.go are the Postgres half, session.go + lock.go the
// Redis half.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User roles and statuses (login design spec §3.1 CHECK constraints).
const (
	RoleUser       = "user"
	RoleSysadmin   = "sysadmin"
	RoleSecadmin   = "secadmin"
	RoleAuditadmin = "auditadmin"

	StatusPending  = "pending"
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// ErrUserNotFound is returned when a user lookup finds no row.
var ErrUserNotFound = errors.New("user not found")

// User is a users row. PasswordHash is populated by every lookup (it is the
// server-side credential record); handlers must never serialize it.
type User struct {
	ID                 int64
	Username           string
	PasswordHash       string
	Role               string
	Status             string
	MustChangePassword bool
	LastLoginAt        *time.Time
}

// UserStore reads users and stamps login activity.
type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore { return &UserStore{pool: pool} }

// ByUsername returns the user row for username, or ErrUserNotFound.
func (s *UserStore) ByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, role, status, must_change_password, last_login_at
		   FROM users WHERE username = $1`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.MustChangePassword, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user by username: %w", err)
	}
	return u, nil
}

// ByID returns the user row for id, or ErrUserNotFound.
func (s *UserStore) ByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, role, status, must_change_password, last_login_at
		   FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.MustChangePassword, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user by id: %w", err)
	}
	return u, nil
}

// TouchLastLogin stamps last_login_at = now() for id.
func (s *UserStore) TouchLastLogin(ctx context.Context, id int64) error {
	if _, err := s.pool.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("touch last_login_at: %w", err)
	}
	return nil
}

// HasAnyAdmin reports whether any admin-role user (sysadmin/secadmin/
// auditadmin) exists. It gates the admin bootstrap run (spec §9).
func (s *UserStore) HasAnyAdmin(ctx context.Context) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE role IN ($1, $2, $3))`,
		RoleSysadmin, RoleSecadmin, RoleAuditadmin).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check admin existence: %w", err)
	}
	return exists, nil
}

// NewAdmin carries the seed data for one bootstrap admin account.
type NewAdmin struct {
	Username     string
	PasswordHash string
	Role         string
}

// CreateAdmins creates the bootstrap admin accounts and their
// admin.bootstrap audit rows in one transaction (spec §9: direct active
// with must_change_password=true so the first admins can log in). The
// audit row's actor_id is NULL (no admin existed to act) and the actor
// username/role are the created account's own.
func (s *UserStore) CreateAdmins(ctx context.Context, admins []NewAdmin) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin admin bootstrap tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful Commit

	for _, a := range admins {
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (username, password_hash, role, status, must_change_password)
			 VALUES ($1, $2, $3, 'active', TRUE) RETURNING id`,
			a.Username, a.PasswordHash, a.Role).Scan(&id); err != nil {
			return fmt.Errorf("create admin %s: %w", a.Username, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO audit_logs (actor_id, actor_username, actor_role, action, target, detail)
			 VALUES (NULL, $1, $2, 'admin.bootstrap', $3, $4)`,
			a.Username, a.Role, fmt.Sprintf("users:%d", id),
			map[string]any{"must_change_password": true}); err != nil {
			return fmt.Errorf("audit admin.bootstrap for %s: %w", a.Username, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit admin bootstrap tx: %w", err)
	}
	return nil
}
