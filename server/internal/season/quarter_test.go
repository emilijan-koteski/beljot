package season_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/emilijan/beljot/server/internal/season"
)

func TestQuarterBounds(t *testing.T) {
	utc := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}

	cases := []struct {
		name      string
		now       time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{"Jan 1 opens Q1", utc(2026, time.January, 1), utc(2026, time.January, 1), utc(2026, time.April, 1)},
		{"mid Q1", utc(2026, time.February, 14), utc(2026, time.January, 1), utc(2026, time.April, 1)},
		{"Mar 31 still Q1", utc(2026, time.March, 31), utc(2026, time.January, 1), utc(2026, time.April, 1)},
		{"Apr 1 opens Q2", utc(2026, time.April, 1), utc(2026, time.April, 1), utc(2026, time.July, 1)},
		{"Jun 30 still Q2", utc(2026, time.June, 30), utc(2026, time.April, 1), utc(2026, time.July, 1)},
		{"Jul 1 opens Q3", utc(2026, time.July, 1), utc(2026, time.July, 1), utc(2026, time.October, 1)},
		{"Sep 30 still Q3", utc(2026, time.September, 30), utc(2026, time.July, 1), utc(2026, time.October, 1)},
		{"Oct 1 opens Q4", utc(2026, time.October, 1), utc(2026, time.October, 1), utc(2027, time.January, 1)},
		// Q4 is the one window that crosses a year, which is where naive
		// month-arithmetic breaks.
		{"Dec 31 still Q4, ending next year", utc(2026, time.December, 31), utc(2026, time.October, 1), utc(2027, time.January, 1)},
		{"leap-year Feb 29", utc(2028, time.February, 29), utc(2028, time.January, 1), utc(2028, time.April, 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := season.QuarterBounds(tc.now)
			assert.True(t, tc.wantStart.Equal(start), "start: want %s got %s", tc.wantStart, start)
			assert.True(t, tc.wantEnd.Equal(end), "end: want %s got %s", tc.wantEnd, end)
		})
	}
}

// A non-UTC instant must resolve by its UTC calendar date, not by its local one.
// 2026-04-01 00:30 in UTC+2 is still 2026-03-31 in UTC, so it belongs to Q1.
func TestQuarterBounds_NormalisesToUTC(t *testing.T) {
	east := time.FixedZone("UTC+2", 2*60*60)
	start, end := season.QuarterBounds(time.Date(2026, time.April, 1, 0, 30, 0, 0, east))
	assert.True(t, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Equal(start))
	assert.True(t, time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC).Equal(end))
	assert.Equal(t, time.UTC, start.Location(), "bounds are always UTC")
}

// started_at inclusive / ends_at exclusive means consecutive windows must ABUT
// exactly: one quarter's end is the next quarter's start, with no gap and no
// overlap anywhere in a multi-year sweep.
func TestQuarterBounds_WindowsAbutWithNoGapOrOverlap(t *testing.T) {
	cursor := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		start, end := season.QuarterBounds(cursor)
		assert.True(t, start.Equal(cursor), "window %d must start where the previous one ended", i)
		assert.True(t, end.After(start), "window %d must be non-empty", i)
		// The instant before the end still belongs to this window.
		s2, _ := season.QuarterBounds(end.Add(-time.Nanosecond))
		assert.True(t, s2.Equal(start), "the instant before ends_at is still inside the window")
		cursor = end
	}
}

func TestQuarterName(t *testing.T) {
	cases := []struct {
		start time.Time
		want  string
	}{
		{time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), "2026 Q1"},
		{time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC), "2026 Q2"},
		{time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), "2026 Q3"},
		{time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), "2026 Q4"},
		{time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC), "2027 Q1"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, season.QuarterName(tc.start))
		})
	}
}
