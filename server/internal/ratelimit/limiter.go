package ratelimit

import (
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// Limiter is a process-local sliding-window counter: at most limit hits per key
// in any window-long interval, the same shape as the avatar upload limiter.
type Limiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	hits      map[string][]time.Time
	lastSweep time.Time
	now       func() time.Time
}

// New returns a limiter allowing limit hits per key in each window.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, hits: make(map[string][]time.Time), now: time.Now}
}

// Take spends one hit from key's budget. When none is left it reports false
// and how long until the oldest hit ages out.
func (l *Limiter) Take(key string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	recent := l.recent(key, now)
	if len(recent) >= l.limit {
		return recent[0].Add(l.window).Sub(now), false
	}
	l.hits[key] = append(recent, now)
	return 0, true
}

// keys is the number of keys currently tracked (tests).
func (l *Limiter) keys() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.hits)
}

// sweep drops every key whose hits have all aged out, at most once per window,
// so addresses that never come back do not grow the map. Caller holds mu.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for key := range l.hits {
		l.recent(key, now)
	}
}

// recent prunes key's hits older than the window, returns the rest and deletes
// the entry once empty. Caller holds mu.
func (l *Limiter) recent(key string, now time.Time) []time.Time {
	hits := l.hits[key]
	cutoff := now.Add(-l.window)
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.hits, key)
		return nil
	}
	l.hits[key] = kept
	return kept
}

// KeyFunc picks the bucket a request is counted against.
type KeyFunc func(c echo.Context) string

// ByClientIP keys on c.RealIP, which the server's IP extractor resolves past
// trusted proxies (Traefik, Cloudflare) to the real client address.
func ByClientIP(c echo.Context) string {
	return "ip:" + c.RealIP()
}

// ByUserOrIP keys on the authenticated user id set by the auth middleware,
// falling back to the client IP when the request carries none.
func ByUserOrIP(c echo.Context) string {
	if id, ok := c.Get("userID").(uint); ok && id != 0 {
		return "user:" + strconv.FormatUint(uint64(id), 10)
	}
	return ByClientIP(c)
}

// Middleware rejects a request with 429 TOO_MANY_ATTEMPTS and a Retry-After
// header once its key has spent the limiter's budget.
func Middleware(l *Limiter, key KeyFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			retryAfter, ok := l.Take(key(c))
			if !ok {
				c.Response().Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
				return apperr.ErrTooManyAttempts
			}
			return next(c)
		}
	}
}

func retryAfterSeconds(d time.Duration) int {
	seconds := int((d + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}
