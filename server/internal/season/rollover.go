package season

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Rollover is the nightly season-rollover job (Story 13.3).
//
// A WRAPPER, NOT A SCHEDULER FRAMEWORK. Repository.CurrentSeason already
// computes the covering quarter, inserts it with ON CONFLICT (started_at)
// DO NOTHING, and re-reads — the idempotency recipe exists and is tested. This
// job exists so a ZERO-TRAFFIC deployment still gets its row shortly after a
// quarter boundary (and the logs prove it ran); correctness never depends on
// it, because the lazy resolver self-heals on the next read or write anyway.
//
// CADENCE IS BOOT-ANCHORED, NOT WALL-CLOCK: this is a plain 24h ticker started
// at process boot, so a server booted at 14:00 runs its passes at 14:00. There
// is no time-of-day scheduling and none is needed — the pass is idempotent, and
// the lazy resolver closes any gap before the next pass anyway.
//
// Lifecycle follows the hub idiom (ws/hub.go): a `done` channel closed by
// Shutdown, started from main.go, stopped during graceful shutdown. Failures
// are logged via slog and the ticker keeps ticking — a transient DB error on
// one pass must not kill the job for the rest of the quarter.
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

// Start launches the loop: one immediate pass (so a deploy that boots just
// after a boundary heals right away, not a day later), then one pass per
// interval until Shutdown.
//
// Guarded on BOTH sides: a second Start is a no-op rather than a second ticker
// loop, and a Start after Shutdown does not run a pass during teardown.
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
		// The same contract violation resolveSeason guards: "never (nil, nil)"
		// is documented, not type-enforced.
		return fmt.Errorf("season rollover: repository returned no season covering %s",
			r.now().Format(time.RFC3339))
	}
	// createdAt is what makes this line diagnostic rather than decorative: every
	// pass logs "window ensured", so the only way to tell the ONE pass that
	// actually created the new quarter from the ~90 no-op passes that follow it
	// is the row's own creation stamp landing near the pass's own timestamp.
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
