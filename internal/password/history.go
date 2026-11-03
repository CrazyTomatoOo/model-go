package password

import (
	"context"
	"fmt"
)

// CheckReuse reports whether candidate matches the current password hash or
// any of the history hashes (login design spec §6: no reuse of the current
// or the last 5 passwords). Every hash is verified with its own stored PHC
// params. The context is checked between verifications and aborts on
// cancellation; an empty currentHash (account creation) skips that check.
func CheckReuse(ctx context.Context, candidate, currentHash string, history []string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if currentHash != "" {
		match, err := Verify(candidate, currentHash)
		if err != nil {
			return false, fmt.Errorf("verify current password hash: %w", err)
		}
		if match {
			return true, nil
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	for i, hash := range history {
		match, err := Verify(candidate, hash)
		if err != nil {
			return false, fmt.Errorf("verify history password hash %d: %w", i, err)
		}
		if match {
			return true, nil
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	return false, nil
}
