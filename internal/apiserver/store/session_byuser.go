package store

import (
	"context"
	"fmt"
	"strings"
)

// ListByUser returns the live session IDs owned by userID. It scans sess:*
// and filters on the user_id hash field; linear in total session count,
// which stays small at v1 scale, and it leaves the §3.2 key layout intact.
func (s *SessionStore) ListByUser(ctx context.Context, userID int64) ([]string, error) {
	var (
		cursor uint64
		sids   []string
	)
	want := fmt.Sprintf("%d", userID)
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, "sess:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("scan sessions: %w", err)
		}
		for _, key := range keys {
			uid, err := s.rdb.HGet(ctx, key, "user_id").Result()
			if err != nil {
				continue // key expired between SCAN and HGET
			}
			if uid == want {
				sids = append(sids, strings.TrimPrefix(key, "sess:"))
			}
		}
		cursor = next
		if cursor == 0 {
			return sids, nil
		}
	}
}

// DeleteByUserID removes every session owned by userID.
func (s *SessionStore) DeleteByUserID(ctx context.Context, userID int64) (int, error) {
	sids, err := s.ListByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, sid := range sids {
		if err := s.Delete(ctx, sid); err != nil {
			return removed, fmt.Errorf("delete session %q: %w", sid, err)
		}
		removed++
	}
	return removed, nil
}
