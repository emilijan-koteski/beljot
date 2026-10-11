package season

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Rollover is the nightly job that ensures the current season row exists even with zero traffic.
// Correctness never depends on it, because the lazy resolver in CurrentSeason self-heals on the next read or write.
type Rollover struct {
	repo     Repository
	interval time.Duration
	now      func() time.Time

	done     chan struct{}
	started  atomic.Bool
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewRollover builds the job. `interval` and `now` are injectable for tests:
// pass 0 and nil for the production defaults (24h, time.Now in UTC).
func NewRollover(repo Repository, interval time.Duration, now func() time.Time) *Rollover {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Rollover{
		repo:     repo,
		interval: interval,
		now:      now,
		done:     make(chan struct{}),
	}
}

// Start runs one pass immediately, then one per interval until Shutdown.
// A second Start, or a Start after Shutdown, is a no-op.
func (r *Rollover) Start() {
	if r.started.Swap(true) {
		return
	}
	select {
	case <-r.done:
		return
	default:
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.logPass(r.RunOnce())

		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-r.done:
				return
			case <-ticker.C:
				r.logPass(r.RunOnce())
			}
		}
	}()
}

// Shutdown stops the loop and waits for any in-flight pass to finish.
// Idempotent, and safe to call even if Start never ran.
func (r *Rollover) Shutdown() {
	r.stopOnce.Do(func() { close(r.done) })
	r.wg.Wait()
}

// RunOnce performs one pass: resolve (and, past a boundary, create) the window
// covering now. Exported so the integration test can prove idempotency —
// two runs, one row — without racing a ticker.
func (r *Rollover) RunOnce() error {
	current, err := r.repo.CurrentSeason(r.now())
	if err != nil {
		return fmt.Errorf("season rollover: resolving current season: %w", err)
	}
	if current == nil {
		return fmt.Errorf("season rollover: repository returned no season covering %s",
			r.now().Format(time.RFC3339))
	}
	slog.Info("season rollover: window ensured",
		"season", current.Name,
		"endsAt", current.EndsAt.UTC().Format(time.RFC3339),
		"createdAt", current.CreatedAt.UTC().Format(time.RFC3339))
	return nil
}

// logPass reports a failed pass without stopping the loop.
func (r *Rollover) logPass(err error) {
	if err != nil {
		slog.Error("season rollover: pass failed", "error", err)
	}
}
