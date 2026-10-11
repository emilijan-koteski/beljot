package season_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/season"
)

// countingRepo is a mutex-guarded CurrentSeason counter safe for concurrent ticker tests.
// Any other Repository method panics through the embedded nil interface.
type countingRepo struct {
	season.Repository
	mu    sync.Mutex
	calls int
	ret   *season.Season
	err   error
}

func (c *countingRepo) CurrentSeason(time.Time) (*season.Season, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.ret, c.err
}

func (c *countingRepo) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestRollover_RunOnceResolvesTheWindow(t *testing.T) {
	repo := &countingRepo{ret: testWindow}
	job := season.NewRollover(repo, 0, nil)

	require.NoError(t, job.RunOnce())
	assert.Equal(t, 1, repo.callCount(), "one pass, one resolver call")
}

// The injectable clock is what the resolver receives — the job never reads a
// wall clock of its own once one is supplied.
func TestRollover_UsesTheInjectedClock(t *testing.T) {
	var got time.Time
	repo := &clockCaptureRepo{ret: testWindow, capture: &got}
	fixed := time.Date(2031, time.February, 3, 3, 0, 0, 0, time.UTC)

	job := season.NewRollover(repo, 0, func() time.Time { return fixed })
	require.NoError(t, job.RunOnce())
	assert.True(t, fixed.Equal(got), "the pass hands the injected clock to the resolver")
}

type clockCaptureRepo struct {
	season.Repository
	ret     *season.Season
	capture *time.Time
}

func (c *clockCaptureRepo) CurrentSeason(now time.Time) (*season.Season, error) {
	*c.capture = now
	return c.ret, nil
}

func TestRollover_RunOnceSurfacesRepositoryErrors(t *testing.T) {
	repo := &countingRepo{err: errors.New("db down")}
	job := season.NewRollover(repo, 0, nil)

	err := job.RunOnce()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

// The same never-(nil,nil) contract guard resolveSeason carries: a repository
// that violates it must produce an ERROR from the pass, not a nil dereference
// inside a long-lived goroutine.
func TestRollover_NilSeasonIsAnErrorNotAPanic(t *testing.T) {
	repo := &countingRepo{}
	job := season.NewRollover(repo, 0, nil)

	err := job.RunOnce()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no season")
}

// Start runs one immediate pass and then ticks; Shutdown stops it. Run with
// -race (make test does): the counter is mutex-guarded precisely so this proves
// the job's own synchronization, not the mock's absence of it.
func TestRollover_StartRunsImmediatelyThenTicks(t *testing.T) {
	repo := &countingRepo{ret: testWindow}
	job := season.NewRollover(repo, 2*time.Millisecond, nil)

	job.Start()
	require.Eventually(t, func() bool { return repo.callCount() >= 3 },
		2*time.Second, time.Millisecond,
		"the immediate pass plus at least two ticks")
	job.Shutdown()
}

// FAILURES DO NOT STOP THE TICKER — a transient 3am DB error must not kill the
// job for the rest of the quarter.
func TestRollover_KeepsTickingThroughFailures(t *testing.T) {
	repo := &countingRepo{err: errors.New("db down")}
	job := season.NewRollover(repo, 2*time.Millisecond, nil)

	job.Start()
	require.Eventually(t, func() bool { return repo.callCount() >= 3 },
		2*time.Second, time.Millisecond,
		"passes keep coming after failed ones")
	job.Shutdown()
}

// Shutdown waits for the loop to exit and no pass runs afterwards.
func TestRollover_ShutdownStopsThePasses(t *testing.T) {
	repo := &countingRepo{ret: testWindow}
	job := season.NewRollover(repo, time.Millisecond, nil)

	job.Start()
	require.Eventually(t, func() bool { return repo.callCount() >= 1 }, 2*time.Second, time.Millisecond)
	job.Shutdown()

	after := repo.callCount()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, after, repo.callCount(), "no pass runs after Shutdown returns")
}

// Shutdown is idempotent and safe without Start — main.go's shutdown path must
// never be able to panic on a double close.
func TestRollover_ShutdownIsIdempotentAndSafeWithoutStart(t *testing.T) {
	repo := &countingRepo{ret: testWindow}

	unstarted := season.NewRollover(repo, time.Millisecond, nil)
	unstarted.Shutdown()
	unstarted.Shutdown()

	started := season.NewRollover(repo, time.Millisecond, nil)
	started.Start()
	started.Shutdown()
	started.Shutdown()
}

// A second Start must not spawn a second ticker loop.
func TestRollover_StartIsIdempotent(t *testing.T) {
	repo := &countingRepo{ret: testWindow}
	job := season.NewRollover(repo, time.Hour, nil)

	job.Start()
	job.Start()
	job.Start()
	require.Eventually(t, func() bool { return repo.callCount() >= 1 },
		2*time.Second, time.Millisecond, "the first Start still runs its immediate pass")

	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, 1, repo.callCount(),
		"three Starts, ONE loop: a second immediate pass would mean a second goroutine")
	job.Shutdown()
}

// Start AFTER Shutdown must not run a pass — a late start during teardown would
// hit a repository whose DB handle is already closing.
func TestRollover_StartAfterShutdownRunsNothing(t *testing.T) {
	repo := &countingRepo{ret: testWindow}
	job := season.NewRollover(repo, time.Millisecond, nil)

	job.Shutdown()
	job.Start()

	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, 0, repo.callCount(), "no pass runs once the job is already shut down")
	job.Shutdown()
}
