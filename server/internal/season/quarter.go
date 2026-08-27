package season

import (
	"fmt"
	"time"
)

// Season windows are CALENDAR QUARTERS IN UTC (Story 13.1 D1):
//
//	Q1 = Jan 1 - Apr 1   Q2 = Apr 1 - Jul 1
//	Q3 = Jul 1 - Oct 1   Q4 = Oct 1 - Jan 1
//
// started_at is INCLUSIVE and ends_at is EXCLUSIVE, so consecutive windows leave
// no gap and no overlap: the instant one quarter ends is the instant the next
// begins, and exactly one window covers any given moment.
//
// Pure, like tier.go: the clock is always a parameter. The only caller that
// reads a real clock is the repository's CurrentSeason, and it passes `now` in.

// QuarterBounds returns the [start, end) bounds of the calendar quarter
// containing now, normalised to UTC.
func QuarterBounds(now time.Time) (start, end time.Time) {
	u := now.UTC()
	// Integer division puts Jan/Feb/Mar in quarter 0, Apr/May/Jun in 1, etc.
	startMonth := time.Month(((int(u.Month())-1)/3)*3 + 1)
	start = time.Date(u.Year(), startMonth, 1, 0, 0, 0, 0, time.UTC)
	// AddDate, not a fixed hour count: quarters are 90-92 days long and only
	// calendar arithmetic lands on the 1st of the next quarter every time.
	end = start.AddDate(0, 3, 0)
	return start, end
}

// QuarterName returns the machine-stable season token for a quarter start,
// e.g. "2026 Q3".
//
// NOT a display string. It crosses the wire on event:season_points_awarded and
// GET /api/v1/seasons/current and the client renders it VERBATIM as an
// identifier -- it is never translated and never localised. It matches the
// `to_char(started_at, 'YYYY "Q"Q')` expression the 000024 seed uses, so a row
// created by the migration and one created by the lazy resolver are named
// identically.
func QuarterName(start time.Time) string {
	u := start.UTC()
	return fmt.Sprintf("%d Q%d", u.Year(), (int(u.Month())-1)/3+1)
}
