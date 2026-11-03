package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// maxAuditPage caps the 1-based page number at 1e6 (50M rows at the fixed
// page size), far beyond the 180-day retention horizon, so the OFFSET
// arithmetic in the store cannot overflow.
const maxAuditPage = 1_000_000

// AuditQueryService is the read seam for GET /audit-logs (spec §4.4).
type AuditQueryService interface {
	Query(ctx context.Context, q store.AuditQuery) (store.AuditPage, error)
}

// AuditHandler serves the auditadmin-only read routes (spec §4.4).
type AuditHandler struct {
	audits AuditQueryService
}

// NewAuditHandler builds an AuditHandler over the given query service.
func NewAuditHandler(audits AuditQueryService) *AuditHandler {
	return &AuditHandler{audits: audits}
}

// List handles GET /api/v1/audit-logs (spec §4.4): parses the optional
// filters, queries one page (page_size fixed at 50, §11.3), and renders
// {items, page, page_size, total}. Only auditadmin may call it; the route
// carries middleware.RequireRole(store.RoleAuditadmin).
func (h *AuditHandler) List(c *gin.Context) {
	query, fields := auditQueryParams(c)
	if len(fields) > 0 {
		writeFieldErrors(c, fields)
		return
	}
	page, err := h.audits.Query(c.Request.Context(), query)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     auditItemJSONs(page.Items),
		"page":      page.Page,
		"page_size": page.PageSize,
		"total":     page.Total,
	})
}

// auditQueryParams parses the §4.4 query-string filters into an AuditQuery.
// The second return is non-empty when a parameter is invalid (each entry
// is one field-level VALIDATION_ERROR detail). page_size is deliberately
// rejected rather than ignored: the page size is fixed at 50 (spec §11.3),
// so a client that sends page_size expects a size it will not get, and
// failing loudly beats silently truncating its data.
func auditQueryParams(c *gin.Context) (store.AuditQuery, []gin.H) {
	q := store.AuditQuery{Page: 1}
	var fields []gin.H

	if raw := c.Query("page"); raw != "" {
		page, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || page < 1 || page > maxAuditPage {
			fields = append(fields, gin.H{"field": "page", "message": "page 必须是 1 到 1000000 之间的整数"})
		} else {
			q.Page = int(page)
		}
	}
	if c.Query("page_size") != "" {
		fields = append(fields, gin.H{"field": "page_size", "message": "page_size 不支持，固定为 50"})
	}
	if raw := c.Query("actor_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			fields = append(fields, gin.H{"field": "actor_id", "message": "actor_id 必须是正整数"})
		} else {
			q.ActorID = &id
		}
	}
	if raw := c.Query("action"); raw != "" {
		q.Action = raw
	}
	if raw := c.Query("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			fields = append(fields, gin.H{"field": "from", "message": "from 必须是 RFC3339 时间"})
		} else {
			q.From = &t
		}
	}
	if raw := c.Query("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			fields = append(fields, gin.H{"field": "to", "message": "to 必须是 RFC3339 时间"})
		} else {
			q.To = &t
		}
	}
	if q.From != nil && q.To != nil && q.From.After(*q.To) {
		fields = append(fields, gin.H{"field": "from", "message": "from 不能晚于 to"})
	}
	return q, fields
}

type auditItemJSON struct {
	ID            int64          `json:"id"`
	ActorID       *int64         `json:"actor_id"`
	ActorUsername string         `json:"actor_username"`
	ActorRole     *string        `json:"actor_role"`
	Action        string         `json:"action"`
	Target        *string        `json:"target"`
	Detail        map[string]any `json:"detail"`
	IP            *string        `json:"ip"`
	CreatedAt     time.Time      `json:"created_at"`
}

func auditItemJSONs(items []store.AuditItem) []auditItemJSON {
	out := make([]auditItemJSON, len(items))
	for i, it := range items {
		out[i] = auditItemJSON{
			ID:            it.ID,
			ActorID:       it.ActorID,
			ActorUsername: it.ActorUsername,
			ActorRole:     it.ActorRole,
			Action:        it.Action,
			Target:        it.Target,
			Detail:        it.Detail,
			IP:            it.IP,
			CreatedAt:     it.CreatedAt,
		}
	}
	return out
}
