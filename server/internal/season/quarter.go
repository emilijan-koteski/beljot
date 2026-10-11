package season

import (
	"fmt"
	"time"
)

// QuarterBounds returns the [start, end) bounds of the calendar quarter
// containing now, normalised to UTC.
func QuarterBounds(now time.Time) (start, end time.Time) {
	u := now.UTC()
	startMonth := time.Month(((int(u.Month())-1)/3)*3 + 1)
	start = time.Date(u.Year(), startMonth, 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 3, 0)
	return start, end
}

// QuarterName returns the machine-stable season token for a quarter start, such as "2026 Q3".
// It is never translated and matches the 000024 seed's to_char expression.
func QuarterName(start time.Time) string {
	u := start.UTC()
	return fmt.Sprintf("%d Q%d", u.Year(), (int(u.Month())-1)/3+1)
}
