package wallet

import "time"

// Economy constants for Story 9.1. These are placeholders tuned per story (see
// the 2026-06-18 change proposal); centralizing them here makes later tuning a
// single edit. StartingBalance duplicates the wallet_balance DEFAULT in
// migration 000009 — keep the two in sync.
const (
	StartingBalance = 5000 // new-player wallet seed (also the migration default)
	DailyBase       = 1000 // day-1 daily bonus
	DailyStep       = 162  // per-streak-day increment
	DailyCap        = 3100 // max daily bonus (first reached on day 14)
)

// Service computes the current UTC day and delegates the atomic write to the
// repository. The streak/bonus rules themselves are pure helpers below so they
// stay DB-free and table-testable.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ProcessDailyLogin grants the daily-login bonus at most once per UTC calendar
// day. It is idempotent — safe to call on every app bootstrap and to retry.
func (s *Service) ProcessDailyLogin(userID uint) (DailyLoginResult, error) {
	return s.repo.ProcessDailyLogin(userID, time.Now().UTC())
}

// ChargeStakes atomically debits `amount` from every (human) userID at match
// start. Returns the offending userID + apperr.ErrInsufficientCoins if any seat
// is insolvent, with the whole charge rolled back (Story 9.2 AC #4, #5).
func (s *Service) ChargeStakes(userIDs []uint, amount int) (uint, error) {
	return s.repo.ChargeStakes(userIDs, amount)
}

// ApplySettlement atomically credits the winning human seats at match end
// (Story 9.2 AC #6, #11). An empty map is a no-op (the no-human-winner sink).
func (s *Service) ApplySettlement(credits map[uint]int) error {
	return s.repo.ApplySettlement(credits)
}

// GetBalance reads a user's current wallet balance for the join affordability
// check (Story 9.2 AC #2).
func (s *Service) GetBalance(userID uint) (int, error) {
	return s.repo.GetBalance(userID)
}

// GetBalances reads balances for many users (userID → balance) to populate the
// per-human newBalance in event:coin_settlement.
func (s *Service) GetBalances(userIDs []uint) (map[uint]int, error) {
	return s.repo.GetBalances(userIDs)
}

// utcDate collapses a timestamp to its UTC calendar date (midnight UTC). Using
// calendar components — not Truncate(24h) or duration math — keeps the
// comparison correct across DST changes and non-UTC server time zones.
func utcDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// evaluateDailyLogin is the pure daily-login rule. Given the stored last-login
// date (nil if never), the stored streak, and today's time, it reports whether
// a bonus is due, the resulting streak, and the bonus amount. No DB, no clock.
//
// Rules (AC #2, #3):
//   - lastLoginAt == nil (legacy / never stamped) → streak 1, grant.
//   - today is the same UTC day as last login       → no grant (already today).
//   - today is exactly the day after last login      → streak++ , grant.
//   - otherwise (gap > 1 day, or clock skew today<last) → streak resets to 1, grant.
//
// Registration stamps streak 0 + last_login = today, so the first next-day call
// yields streak 0+1 = 1 (day-1).
func evaluateDailyLogin(lastLoginAt *time.Time, currentStreak int, today time.Time) (grant bool, newStreak int, amount int) {
	if lastLoginAt == nil {
		return true, 1, bonusForStreak(1)
	}

	td := utcDate(today)
	ld := utcDate(*lastLoginAt)

	switch {
	case td.Equal(ld):
		// Already counted today — grant nothing, keep the current streak.
		return false, currentStreak, 0
	case td.Equal(ld.AddDate(0, 0, 1)):
		s := currentStreak + 1
		return true, s, bonusForStreak(s)
	default:
		return true, 1, bonusForStreak(1)
	}
}

// bonusForStreak returns the daily bonus for a given streak day, capped at
// DailyCap. The streak counter itself keeps growing past the cap (for "Day N"
// display); only the amount is clamped. Curve: day1=1000, day2=1162, …,
// day13=2944, day14=3106→3100, day15+=3100.
func bonusForStreak(streak int) int {
	amount := DailyBase + (streak-1)*DailyStep
	if amount > DailyCap {
		return DailyCap
	}
	return amount
}
