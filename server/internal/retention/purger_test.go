package retention

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurger_RunOnceRunsEveryTaskPastAFailure(t *testing.T) {
	fixed := time.Date(2026, 11, 28, 3, 0, 0, 0, time.UTC)
	var seen []time.Time
	p := NewPurger([]Task{
		{Name: "broken", Purge: func(time.Time) (int64, error) { return 0, errors.New("boom") }},
		{Name: "tokens", Purge: func(now time.Time) (int64, error) {
			seen = append(seen, now)
			return 3, nil
		}},
	}, 0, func() time.Time { return fixed })

	got := p.RunOnce()

	assert.Equal(t, map[string]int64{"tokens": 3}, got)
	assert.Equal(t, []time.Time{fixed}, seen)
}

func TestPurger_StartRunsImmediatelyAndShutdownStops(t *testing.T) {
	var runs atomic.Int32
	p := NewPurger([]Task{{Name: "count", Purge: func(time.Time) (int64, error) {
		runs.Add(1)
		return 0, nil
	}}}, time.Hour, nil)

	p.Start()
	p.Start()
	require.Eventually(t, func() bool { return runs.Load() == 1 }, time.Second, 5*time.Millisecond)
	p.Shutdown()
	p.Shutdown()
	assert.Equal(t, int32(1), runs.Load())
}

func TestPurger_ShutdownWithoutStart(t *testing.T) {
	p := NewPurger(nil, 0, nil)
	p.Shutdown()
	p.Start()
	assert.Empty(t, p.RunOnce())
}
