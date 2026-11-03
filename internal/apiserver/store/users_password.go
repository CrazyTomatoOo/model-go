package store

import (
	"context"
	"fmt"
)

// ChangePassword swaps users.password_hash, records oldHash in
// password_history, trims the history to the newest 5 rows, and clears
// must_change_password, all in one transaction (spec §6).
func (s *UserStore) ChangePassword(ctx context.Context, userID int64, newHash, oldHash string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin change password tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful Commit

	if _, err := tx.Exec(ctx,
		`UPDATE users SET password_hash = $2, must_change_password = FALSE, updated_at = now() WHERE id = $1`,
		userID, newHash); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO password_history (user_id, password_hash) VALUES ($1, $2)`,
		userID, oldHash); err != nil {
		return fmt.Errorf("insert password history: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM password_history WHERE user_id = $1 AND id NOT IN (
		   SELECT id FROM password_history WHERE user_id = $1
		   ORDER BY created_at DESC, id DESC LIMIT 5)`,
		userID); err != nil {
		return fmt.Errorf("trim password history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit change password tx: %w", err)
	}
	return nil
}

// HistoryHashes returns the newest up-to-5 historical password hashes for
// userID (the current hash is never in this table), newest first.
func (s *UserStore) HistoryHashes(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT password_hash FROM password_history WHERE user_id = $1
		 ORDER BY created_at DESC, id DESC LIMIT 5`, userID)
	if err != nil {
		return nil, fmt.Errorf("query password history: %w", err)
	}
	defer rows.Close()

	hashes := make([]string, 0, 5)
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
