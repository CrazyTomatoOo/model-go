package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrUsernameTaken is returned when creating a user whose username already
// exists (login design spec §4.2: the unique violation maps to a 409 that
// must not echo the conflicting username).
var ErrUsernameTaken = errors.New("username taken")

// Create inserts a pending user on behalf of the acting admin (spec §4.2).
// status/must_change_password/created_by are server-forced; the created
// account always starts pending with a password change required. It returns
// ErrUsernameTaken on a username collision.
func (s *UserStore) Create(ctx context.Context, username, passwordHash, role string, createdBy int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash, role, status, must_change_password, created_by)
		 VALUES ($1, $2, $3, 'pending', TRUE, $4) RETURNING id`,
		username, passwordHash, role, createdBy).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, ErrUsernameTaken
		}
		return 0, fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

// PasswordHistory returns the user's stored password hashes, newest first
// (spec §6: the reuse check covers the last 5 passwords).
func (s *UserStore) PasswordHistory(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT password_hash FROM password_history
		   WHERE user_id = $1 ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query password history: %w", err)
	}
	defer rows.Close()
	var hashes []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, fmt.Errorf("scan password history: %w", err)
		}
		hashes = append(hashes, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate password history: %w", err)
	}
	return hashes, nil
}

// ResetPassword replaces the user's password in one transaction (spec §4.2):
// the old hash moves into password_history and the history is trimmed to the
// newest 5 rows. must_change_password is forced true; status is untouched.
// It returns ErrUserNotFound when the user does not exist.
func (s *UserStore) ResetPassword(ctx context.Context, userID int64, newHash string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password reset tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful Commit

	var oldHash string
	err = tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&oldHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("lock user for password reset: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE users SET password_hash = $1, must_change_password = TRUE, updated_at = now() WHERE id = $2`,
		newHash, userID); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO password_history (user_id, password_hash) VALUES ($1, $2)`, userID, oldHash); err != nil {
		return fmt.Errorf("insert password history: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM password_history WHERE user_id = $1 AND id NOT IN (
		   SELECT id FROM password_history WHERE user_id = $1 ORDER BY created_at DESC, id DESC LIMIT 5)`,
		userID); err != nil {
		return fmt.Errorf("trim password history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset tx: %w", err)
	}
	return nil
}

// Disable sets the user's status to disabled (spec §4.2). It returns
// ErrUserNotFound when the user does not exist; disabling an already
// disabled user is a no-op success.
func (s *UserStore) Disable(ctx context.Context, userID int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET status = 'disabled', updated_at = now() WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("disable user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
