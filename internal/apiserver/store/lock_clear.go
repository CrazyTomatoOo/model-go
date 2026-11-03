package store

import (
	"context"
	"fmt"
)

// Clear removes the lock:{username} failure counter (spec §4.3 unlock).
// Deleting an absent key is not an error: an unlocked account clears to 0.
func (s *LockStore) Clear(ctx context.Context, username string) error {
	if err := s.rdb.Del(ctx, lockKey(username)).Err(); err != nil {
		return fmt.Errorf("clear lock count: %w", err)
	}
	return nil
}
