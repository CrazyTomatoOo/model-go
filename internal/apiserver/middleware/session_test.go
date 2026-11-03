package middleware_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fmt"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

type stubUserStore struct {
	byID func(int64) (store.User, error)
}

func (s *stubUserStore) ByID(_ context.Context, id int64) (store.User, error) {
	return s.byID(id)
}

func (s *stubUserStore) found(u store.User) *stubUserStore {
	s.byID = func(int64) (store.User, error) { return u, nil }
	return s
}

func testSessionEngine(t *testing.T, users *stubUserStore) (*gin.Engine, *store.SessionStore, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mr, rdb := newTestRedis(t)
	sessions := store.NewSessionStore(rdb)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := gin.New()
	engine.Use(middleware.Session(sessions, users, logger))
	engine.GET("/protected", func(c *gin.Context) {
		user := c.MustGet(middleware.UserKey).(store.User)
		c.JSON(http.StatusOK, gin.H{"username": user.Username})
	})
	return engine, sessions, mr
}

func doRequest(engine *gin.Engine, method, path, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestSessionNoCookie(t *testing.T) {
	engine, _, _ := testSessionEngine(t, &stubUserStore{})
	rec := doRequest(engine, http.MethodGet, "/protected", "")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, rec.Body.String())
}

func TestSessionUnknownSid(t *testing.T) {
	engine, _, _ := testSessionEngine(t, &stubUserStore{})
	rec := doRequest(engine, http.MethodGet, "/protected", "forged-sid")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionValid(t *testing.T) {
	engine, sessions, _ := testSessionEngine(t, (&stubUserStore{}).found(store.User{ID: 7, Username: "alice"}))
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)

	rec := doRequest(engine, http.MethodGet, "/protected", sess.SID)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"username":"alice"}`, rec.Body.String())
}

func TestSessionTouchRenewsTTL(t *testing.T) {
	engine, sessions, mr := testSessionEngine(t, (&stubUserStore{}).found(store.User{ID: 7, Username: "alice"}))
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)
	mr.SetTTL("sess:"+sess.SID, time.Minute)

	require.Equal(t, http.StatusOK, doRequest(engine, http.MethodGet, "/protected", sess.SID).Code)
	require.Equal(t, store.SessionTTL, mr.TTL("sess:"+sess.SID))
}

func TestSessionAbsExpired(t *testing.T) {
	engine, sessions, mr := testSessionEngine(t, (&stubUserStore{}).found(store.User{ID: 7, Username: "alice"}))
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)
	past := fmt.Sprintf("%d", time.Now().Add(-time.Hour).Unix())
	mr.HSet("sess:"+sess.SID, "abs_expires", past)

	rec := doRequest(engine, http.MethodGet, "/protected", sess.SID)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionRedisDownFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	sessions := store.NewSessionStore(rdb)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := gin.New()
	engine.Use(middleware.Session(sessions, (&stubUserStore{}).found(store.User{ID: 7, Username: "alice"}), logger))
	engine.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doRequest(engine, http.MethodGet, "/protected", "any-sid")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionUserVanished(t *testing.T) {
	engine, sessions, _ := testSessionEngine(t, &stubUserStore{byID: func(int64) (store.User, error) { return store.User{}, store.ErrUserNotFound }})
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)

	rec := doRequest(engine, http.MethodGet, "/protected", sess.SID)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
