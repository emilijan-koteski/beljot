package wallet

import "time"

const (
	StartingBalance = 5000
	DailyBase       = 1000
	DailyStep       = 162
	DailyCap        = 3100
)

// Service computes the current UTC day and delegates the atomic wallet writes to the repository.
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

// ChargeStakes atomically debits amount from every human userID at match start.
// An insolvent seat rolls the charge back and returns its userID with apperr.ErrInsufficientCoins.
func (s *Service) ChargeStakes(userIDs []uint, amount int) (uint, error) {
	return s.repo.ChargeStakes(userIDs, amount)
}

// ApplySettlement atomically credits the winning human seats at match end.
func (s *Service) ApplySettlement(credits map[uint]int) error {
	return s.repo.ApplySettlement(credits)
}

// GetBalance reads a user's current wallet balance for the join affordability check.
func (s *Service) GetBalance(userID uint) (int, error) {
	return s.repo.GetBalance(userID)
}

// GetBalances reads balances for many users (userID → balance) to populate the
// per-human newBalance in event:coin_settlement.
func (s *Service) GetBalances(userIDs []uint) (map[uint]int, error) {
	return s.repo.GetBalances(userIDs)
}

// utcDate collapses a timestamp to midnight UTC of its calendar date.
// Calendar components, unlike Truncate(24h), stay correct across DST and non-UTC zones.
func utcDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// evaluateDailyLogin reports whether a bonus is due, the resulting streak, and the amount.
// A same-day login grants nothing, a next-day login extends the streak, and anything else restarts it at 1.
func evaluateDailyLogin(lastLoginAt *time.Time, currentStreak int, today time.Time) (grant bool, newStreak int, amount int) {
	if lastLoginAt == nil {
		return true, 1, bonusForStreak(1)
	}

	td := utcDate(today)
	ld := utcDate(*lastLoginAt)

	switch {
	case td.Equal(ld):
		return false, currentStreak, 0
	case td.Equal(ld.AddDate(0, 0, 1)):
		s := currentStreak + 1
		return true, s, bonusForStreak(s)
	default:
		return true, 1, bonusForStreak(1)
	}
}

// bonusForStreak returns the daily bonus for a given streak day, capped at DailyCap.
func bonusForStreak(streak int) int {
	amount := DailyBase + (streak-1)*DailyStep
	if amount > DailyCap {
		return DailyCap
	}
	return amount
}
