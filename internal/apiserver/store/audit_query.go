package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// auditPageSize is the fixed page size for audit queries (login design
// spec §11.3: page_size=50).
const auditPageSize = 50

// AuditQuery carries the optional GET /audit-logs filters (spec §4.4).
// Nil ActorID/From/To and empty Action mean "no filter"; Page is 1-based.
type AuditQuery struct {
	ActorID *int64
	Action  string
	From    *time.Time
	To      *time.Time
	Page    int
}

// AuditItem is one audit_logs row as returned by AuditStore.Query.
// ActorID, ActorRole, Target, and IP are nil when the row's column is NULL.
type AuditItem struct {
	ID            int64
	ActorID       *int64
	ActorUsername string
	ActorRole     *string
	Action        string
	Target        *string
	Detail        map[string]any
	IP            *string
	CreatedAt     time.Time
}

// AuditPage is one page of query results: the rows, the 1-based page
// number, the fixed page size, and the total row count across all pages.
type AuditPage struct {
	Items    []AuditItem
	Page     int
	PageSize int
	Total    int64
}

// auditQuerySQL renders the WHERE clause and parameters for q plus the
// LIMIT/OFFSET pair. It is a pure function so the filter and pagination
// math is unit-testable without a database. A page below 1 is clamped to 1
// (callers validate already; the clamp keeps the OFFSET arithmetic safe).
func auditQuerySQL(q AuditQuery) (where string, args []any, limit, offset int) {
	var conds []string
	if q.ActorID != nil {
		args = append(args, *q.ActorID)
		conds = append(conds, fmt.Sprintf("actor_id = $%d", len(args)))
	}
	if q.Action != "" {
		args = append(args, q.Action)
		conds = append(conds, fmt.Sprintf("action = $%d", len(args)))
	}
	if q.From != nil {
		args = append(args, *q.From)
		conds = append(conds, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if q.To != nil {
		args = append(args, *q.To)
		conds = append(conds, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	return where, args, auditPageSize, (page - 1) * auditPageSize
}

// Query returns one page of audit rows matching q, newest first (spec
// §11.3: created_at DESC; id DESC breaks same-timestamp ties so pagination
// is stable). The page size is the fixed 50.
func (s *AuditStore) Query(ctx context.Context, q AuditQuery) (AuditPage, error) {
	where, args, limit, offset := auditQuerySQL(q)

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs`+where, args...).Scan(&total); err != nil {
		return AuditPage{}, fmt.Errorf("count audit rows: %w", err)
	}

	// ip::text is the INET column rendered as its canonical host text so a
	// NULL row scans into a nullable string like every other column.
	rows, err := s.pool.Query(ctx, fmt.Sprintf(
		`SELECT id, actor_id, actor_username, actor_role, action, target, detail, ip::text, created_at
		   FROM audit_logs%s
		  ORDER BY created_at DESC, id DESC
		  LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2),
		append(args, limit, offset)...)
	if err != nil {
		return AuditPage{}, fmt.Errorf("query audit rows: %w", err)
	}
	defer rows.Close()

	items := make([]AuditItem, 0)
	for rows.Next() {
		var it AuditItem
		var rawDetail []byte
		if err := rows.Scan(&it.ID, &it.ActorID, &it.ActorUsername, &it.ActorRole, &it.Action,
			&it.Target, &rawDetail, &it.IP, &it.CreatedAt); err != nil {
			return AuditPage{}, fmt.Errorf("scan audit row: %w", err)
		}
		if rawDetail != nil {
			if err := json.Unmarshal(rawDetail, &it.Detail); err != nil {
				return AuditPage{}, fmt.Errorf("unmarshal audit detail: %w", err)
			}
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return AuditPage{}, fmt.Errorf("iterate audit rows: %w", err)
	}
	return AuditPage{Items: items, Page: q.Page, PageSize: auditPageSize, Total: total}, nil
}
