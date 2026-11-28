package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(raw), &line))
		lines = append(lines, line)
	}
	return lines
}

func findLog(lines []map[string]any, msg string) map[string]any {
	for _, l := range lines {
		if l["msg"] == msg {
			return l
		}
	}
	return nil
}

func TestCoreMiddleware_PanicBecomesLogged500(t *testing.T) {
	buf := captureLogs(t)
	e := echo.New()
	useCoreMiddleware(e, []string{"http://localhost:5173"})
	e.GET("/boom", func(echo.Context) error { panic("handler exploded") })

	req := httptest.NewRequest(http.MethodGet, "/boom?token=secret", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	var body map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "INTERNAL_ERROR", body["error"]["code"])

	lines := logLines(t, buf)
	panicLog := findLog(lines, "panic recovered")
	require.NotNil(t, panicLog, "the panic is logged")
	assert.Equal(t, "handler exploded", panicLog["panic"])
	assert.Contains(t, panicLog["stack"], "goroutine")

	requestLog := findLog(lines, "request")
	require.NotNil(t, requestLog, "the request logger still records the request")
	assert.EqualValues(t, http.StatusInternalServerError, requestLog["status"])
	assert.NotContains(t, buf.String(), "secret", "no query string reaches the log")
}

func TestCoreMiddleware_RequestLogOmitsQueryString(t *testing.T) {
	buf := captureLogs(t)
	e := echo.New()
	useCoreMiddleware(e, []string{"http://localhost:5173"})
	e.GET("/api/v1/users", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users?q=alice", nil)
	req.RemoteAddr = "198.51.100.7:4321"
	e.ServeHTTP(httptest.NewRecorder(), req)

	requestLog := findLog(logLines(t, buf), "request")
	require.NotNil(t, requestLog)
	assert.Equal(t, "/api/v1/users", requestLog["path"])
	assert.Equal(t, "GET", requestLog["method"])
	assert.EqualValues(t, http.StatusNoContent, requestLog["status"])
	assert.Equal(t, "198.51.100.7", requestLog["ip"])
	assert.NotContains(t, buf.String(), "alice")
}

func TestAuthRateLimits_LoginReturns429AfterBudget(t *testing.T) {
	captureLogs(t)
	e := echo.New()
	e.IPExtractor = clientIPExtractor()
	useCoreMiddleware(e, nil)
	limits := newAuthRateLimits()
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }, limits.login)

	post := func(xff string) int {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "10.0.1.5:4321"
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < credentialAttemptsPerMinute; i++ {
		require.Equal(t, http.StatusNoContent, post("203.0.113.9"), "attempt %d", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, post("203.0.113.9"))
	assert.Equal(t, http.StatusNoContent, post("203.0.113.10"), "a different client behind the same proxy is unaffected")
}
