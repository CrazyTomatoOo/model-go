package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Login lockout and rate-limit policy (login design spec §3.2/§4.1):
// lock:{username} counts consecutive failures with a 15-minute TTL
// refreshed on every failure; a count of LockThreshold locks the account.
// Login attempts from one IP are limited to RateLimitPerMinute per minute.
const (
	LockThreshold      = 5
	LockTTL            = 15 * time.Minute
	RateLimitPerMinute = 20
	RateLimitWindow    = time.Minute
)

func lockKey(username string) string     { return "lock:" + username }
func rateLimitKey(ip string) string      { return "rl:" + ip }

// LockStore manages the lock:{username} consecutive-failure counter.
type LockStore struct {
	rdb *redis.Client
}

func NewLockStore(rdb *redis.Client) *LockStore { return &LockStore{rdb: rdb} }

// Count returns the current failure count; 0 when none are recorded.
func (s *LockStore) Count(ctx context.Context, username string) (int, error) {
	value, err := s.rdb.Get(ctx, lockKey(username)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get lock count: %w", err)
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("malformed lock count %q: %w", value, err)
	}
	return n, nil
}

// Incr bumps the failure counter, refreshes its TTL, and returns the new
// count. The lock releases itself when the TTL expires.
func (s *LockStore) Incr(ctx context.Context, username string) (int, error) {
	n, err := s.rdb.Incr(ctx, lockKey(username)).Result()
	if err != nil {
		return 0, fmt.Errorf("incr lock count: %w", err)
	}
	if err := s.rdb.Expire(ctx, lockKey(username), LockTTL).Err(); err != nil {
		return 0, fmt.Errorf("refresh lock ttl: %w", err)
	}
	return int(n), nil
}

// RateLimiter enforces the per-IP login budget with an rl:{ip} counter.
type RateLimiter struct {
	rdb *redis.Client
}

func NewRateLimiter(rdb *redis.Client) *RateLimiter { return &RateLimiter{rdb: rdb} }

// Allow records one login attempt from ip and reports whether it is within
// the per-minute budget. The counter window starts at the first attempt.
func (r *RateLimiter) Allow(ctx context.Context, ip string) (bool, error) {
	n, err := r.rdb.Incr(ctx, rateLimitKey(ip)).Result()
	if err != nil {
		return false, fmt.Errorf("incr rate limit: %w", err)
	}
	if n == 1 {
		if err := r.rdb.Expire(ctx, rateLimitKey(ip), RateLimitWindow).Err(); err != nil {
			return false, fmt.Errorf("set rate limit ttl: %w", err)
		}
	}
	return n <= RateLimitPerMinute, nil
}
