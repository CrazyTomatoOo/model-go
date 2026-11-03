package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditEntry is one append-only audit_logs row (login design spec §3.1/§7).
// actor_id is nil for unauthenticated actions (login attempts, bootstrap).
type AuditEntry struct {
	ActorID       *int64
	ActorUsername string
	ActorRole     string
	Action        string
	Target        string
	Detail        map[string]any
	IP            string
}

// AuditStore writes append-only audit rows. audit_logs is INSERT/SELECT
// only (no UPDATE/DELETE grants for the application role, spec §3.1).
type AuditStore struct {
	pool *pgxpool.Pool
}

func NewAuditStore(pool *pgxpool.Pool) *AuditStore { return &AuditStore{pool: pool} }

// Log inserts one audit row. A nil Detail is stored as NULL; an IP that
// does not parse as an inet is stored as NULL.
func (s *AuditStore) Log(ctx context.Context, e AuditEntry) error {
	var ip net.IP
	if parsed := net.ParseIP(e.IP); parsed != nil {
		// pgx sends 16-byte net.IP as IPv6; encode plain IPv4 as 4 bytes
		// so the stored inet stays in the IPv4 address family.
		if v4 := parsed.To4(); v4 != nil {
			ip = v4
		} else {
			ip = parsed
		}
	}
	var detail any
	if e.Detail != nil {
		raw, err := json.Marshal(e.Detail)
		if err != nil {
			return fmt.Errorf("marshal audit detail: %w", err)
		}
		detail = json.RawMessage(raw)
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_logs (actor_id, actor_username, actor_role, action, target, detail, ip)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ActorID, e.ActorUsername, e.ActorRole, e.Action, e.Target, detail, ip)
	if err != nil {
		return fmt.Errorf("insert audit row: %w", err)
	}
	return nil
}
