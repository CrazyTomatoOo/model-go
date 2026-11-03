package store

import (
	"context"
	"errors"
	"fmt"
)

// ErrUserNotApprovable is returned by Approve when the user exists but its
// status is neither pending nor disabled (spec §4.3: approve transitions
// pending→active and disabled→active only).
var ErrUserNotApprovable = errors.New("user not approvable")

// Approve activates a pending or disabled user and records who approved it
// (spec §4.3: double-review path for re-enabling a disabled account). The
// status predicate makes the transition atomic; a missing user and an
// already-active user both yield zero rows and return ErrUserNotApprovable,
// so callers that need to tell them apart look the user up first.
func (s *UserStore) Approve(ctx context.Context, id, approvedBy int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET status = 'active', approved_by = $2
		  WHERE id = $1 AND status IN ('pending', 'disabled')`, id, approvedBy)
	if err != nil {
		return fmt.Errorf("approve user %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotApprovable
	}
	return nil
}
