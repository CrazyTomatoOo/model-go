package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/handler"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

type stubAuditQuery struct {
	queryFn func(ctx context.Context, q store.AuditQuery) (store.AuditPage, error)
}

func (s *stubAuditQuery) Query(ctx context.Context, q store.AuditQuery) (store.AuditPage, error) {
	return s.queryFn(ctx, q)
}

func newAuditEngine(audits handler.AuditQueryService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := handler.NewAuditHandler(audits)
	engine.GET("/audit-logs", h.List)
	return engine
}

func getAudit(engine *gin.Engine, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/audit-logs"+query, nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestAuditListHappyPath(t *testing.T) {
	actorID := int64(42)
	role := "sysadmin"
	target := "users:42"
	ip := "203.0.113.9"
	created := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	audits := &stubAuditQuery{queryFn: func(_ context.Context, q store.AuditQuery) (store.AuditPage, error) {
		require.Equal(t, 1, q.Page, "absent page must default to 1")
		require.Nil(t, q.ActorID)
		require.Equal(t, "", q.Action)
		require.Nil(t, q.From)
		require.Nil(t, q.To)
		return store.AuditPage{
			Items: []store.AuditItem{{
				ID:            7,
				ActorID:       &actorID,
				ActorUsername: "alice",
				ActorRole:     &role,
				Action:        "auth.login_success",
				Target:        &target,
				Detail:        map[string]any{"attempt": float64(3)},
				IP:            &ip,
				CreatedAt:     created,
			}},
			Page: 1, PageSize: 50, Total: 1,
		}, nil
	}}
	rec := getAudit(newAuditEngine(audits), "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"items":[{
			"id":7,"actor_id":42,"actor_username":"alice","actor_role":"sysadmin",
			"action":"auth.login_success","target":"users:42","detail":{"attempt":3},
			"ip":"203.0.113.9","created_at":"2026-11-01T12:00:00Z"
		}],
		"page":1,"page_size":50,"total":1
	}`, rec.Body.String())
}

func TestAuditListAllFiltersForwarded(t *testing.T) {
	audits := &stubAuditQuery{queryFn: func(_ context.Context, q store.AuditQuery) (store.AuditPage, error) {
		require.Equal(t, 3, q.Page)
		require.NotNil(t, q.ActorID)
		require.Equal(t, int64(42), *q.ActorID)
		require.Equal(t, "account.disable", q.Action)
		require.NotNil(t, q.From)
		require.True(t, q.From.Equal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)))
		require.NotNil(t, q.To)
		require.True(t, q.To.Equal(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)))
		return store.AuditPage{Items: []store.AuditItem{}, Page: 3, PageSize: 50, Total: 0}, nil
	}}
	rec := getAudit(newAuditEngine(audits),
		"?actor_id=42&action=account.disable&from=2026-01-02T00:00:00Z&to=2026-01-03T00:00:00Z&page=3")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"items":[],"page":3,"page_size":50,"total":0}`, rec.Body.String())
}

func TestAuditListBadParams(t *testing.T) {
	tests := []struct {
		name  string
		query string
		field string
	}{
		{"page zero", "?page=0", "page"},
		{"page non-numeric", "?page=abc", "page"},
		{"page above cap", "?page=1000001", "page"},
		{"page_size rejected", "?page_size=10", "page_size"},
		{"actor_id non-numeric", "?actor_id=abc", "actor_id"},
		{"actor_id negative", "?actor_id=-1", "actor_id"},
		{"from malformed", "?from=2026-01-02", "from"},
		{"to malformed", "?to=not-a-time", "to"},
		{"from after to", "?from=2026-01-03T00:00:00Z&to=2026-01-02T00:00:00Z", "from"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			audits := &stubAuditQuery{queryFn: func(context.Context, store.AuditQuery) (store.AuditPage, error) {
				t.Fatal("store must not be called on validation failure")
				return store.AuditPage{}, nil
			}}
			rec := getAudit(newAuditEngine(audits), tc.query)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			var body struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Errors  []struct {
					Field   string `json:"field"`
					Message string `json:"message"`
				} `json:"errors"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, "VALIDATION_ERROR", body.Code)
			require.Equal(t, "请求参数校验失败", body.Message)
			require.Len(t, body.Errors, 1)
			require.Equal(t, tc.field, body.Errors[0].Field)
			require.NotEmpty(t, body.Errors[0].Message)
		})
	}
}

func TestAuditListMultipleBadParams(t *testing.T) {
	audits := &stubAuditQuery{queryFn: func(context.Context, store.AuditQuery) (store.AuditPage, error) {
		t.Fatal("store must not be called on validation failure")
		return store.AuditPage{}, nil
	}}
	rec := getAudit(newAuditEngine(audits), "?page=0&page_size=10&from=bad")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body struct {
		Errors []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, []string{"page", "page_size", "from"}, []string{body.Errors[0].Field, body.Errors[1].Field, body.Errors[2].Field})
}

func TestAuditListNilColumnsRenderAsNull(t *testing.T) {
	audits := &stubAuditQuery{queryFn: func(context.Context, store.AuditQuery) (store.AuditPage, error) {
		return store.AuditPage{
			Items: []store.AuditItem{{
				ID: 9, ActorUsername: "ghost", Action: "auth.login_failure",
			}},
			Page: 1, PageSize: 50, Total: 1,
		}, nil
	}}
	rec := getAudit(newAuditEngine(audits), "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"items":[{"id":9,"actor_id":null,"actor_username":"ghost","actor_role":null,
			"action":"auth.login_failure","target":null,"detail":null,"ip":null,
			"created_at":"0001-01-01T00:00:00Z"}],
		"page":1,"page_size":50,"total":1
	}`, rec.Body.String())
}

func TestAuditListStoreErrorIsOpaque(t *testing.T) {
	audits := &stubAuditQuery{queryFn: func(context.Context, store.AuditQuery) (store.AuditPage, error) {
		return store.AuditPage{}, context.DeadlineExceeded
	}}
	rec := getAudit(newAuditEngine(audits), "")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"code":"INTERNAL","message":"服务器内部错误"}`, rec.Body.String())
}
