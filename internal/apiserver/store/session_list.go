package store

import (
	"context"
	"fmt"
	"strings"
)

// ListAll returns every live session. It scans sess:* and reads each hash,
// skipping keys that expire between SCAN and HGET (spec §11.3: session
// enumeration without a user filter).
func (s *SessionStore) ListAll(ctx context.Context) ([]Session, error) {
	var (
		cursor uint64
		out    []Session
	)
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, "sess:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("scan sessions: %w", err)
		}
		for _, key := range keys {
			sess, err := s.Get(ctx, strings.TrimPrefix(key, "sess:"))
			if err != nil {
				continue // key expired between SCAN and HGET
			}
			out = append(out, sess)
		}
		cursor = next
		if cursor == 0 {
			return out, nil
		}
	}
}
