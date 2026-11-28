package ratelimit

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time { return f.t }

func newTestLimiter(limit int, window time.Duration) (*Limiter, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	l := New(limit, window)
	l.now = clock.now
	return l, clock
}

func TestLimiter_AllowsUpToLimitThenRefuses(t *testing.T) {
	l, clock := newTestLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		_, ok := l.Take("a")
		require.True(t, ok, "hit %d", i+1)
		clock.t = clock.t.Add(10 * time.Second)
	}
	retry, ok := l.Take("a")
	assert.False(t, ok)
	assert.Equal(t, 30*time.Second, retry)
}

func TestLimiter_KeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(1, time.Minute)
	_, ok := l.Take("a")
	require.True(t, ok)
	_, ok = l.Take("a")
	assert.False(t, ok)
	_, ok = l.Take("b")
	assert.True(t, ok)
}

func TestLimiter_WindowSlides(t *testing.T) {
	l, clock := newTestLimiter(2, time.Minute)
	l.Take("a")
	clock.t = clock.t.Add(30 * time.Second)
	l.Take("a")
	_, ok := l.Take("a")
	require.False(t, ok)

	clock.t = clock.t.Add(31 * time.Second)
	_, ok = l.Take("a")
	assert.True(t, ok, "the first hit aged out")
	_, ok = l.Take("a")
	assert.False(t, ok)
}

func TestLimiter_SweepDropsIdleKeys(t *testing.T) {
	l, clock := newTestLimiter(5, time.Minute)
	l.Take("a")
	l.Take("b")
	require.Equal(t, 2, l.keys())

	clock.t = clock.t.Add(2 * time.Minute)
	l.Take("c")
	assert.Equal(t, 1, l.keys())
}

func serve(e *echo.Echo, remoteAddr, xff string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set(echo.HeaderXForwardedFor, xff)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func newEcho(l *Limiter, key KeyFunc) *echo.Echo {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPFromXFFHeader(echo.TrustPrivateNet(true))
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		var appErr *apperr.AppError
		if errors.As(err, &appErr) {
			_ = c.JSON(appErr.Status, map[string]any{"error": map[string]string{"code": appErr.Code}})
			return
		}
		_ = c.NoContent(http.StatusInternalServerError)
	}
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }, Middleware(l, key))
	return e
}

func TestMiddleware_KeysOnForwardedClientIPNotProxy(t *testing.T) {
	l := New(2, time.Minute)
	e := newEcho(l, ByClientIP)
	proxy := "10.0.0.2:5000"

	assert.Equal(t, http.StatusNoContent, serve(e, proxy, "203.0.113.7").Code)
	assert.Equal(t, http.StatusNoContent, serve(e, proxy, "203.0.113.7").Code)
	rec := serve(e, proxy, "203.0.113.7")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))

	var body map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "TOO_MANY_ATTEMPTS", body["error"]["code"])

	assert.Equal(t, http.StatusNoContent, serve(e, proxy, "198.51.100.9").Code,
		"another client behind the same proxy keeps its own budget")
}

func TestByUserOrIP(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodDelete, "/", nil), httptest.NewRecorder())
	c.Request().RemoteAddr = "203.0.113.7:1234"
	assert.Equal(t, "ip:203.0.113.7", ByUserOrIP(c))
	c.Set("userID", uint(42))
	assert.Equal(t, "user:42", ByUserOrIP(c))
}

func TestRetryAfterSeconds(t *testing.T) {
	assert.Equal(t, 1, retryAfterSeconds(0))
	assert.Equal(t, 1, retryAfterSeconds(200*time.Millisecond))
	assert.Equal(t, 30, retryAfterSeconds(30*time.Second))
	assert.Equal(t, 31, retryAfterSeconds(30*time.Second+time.Millisecond))
}
