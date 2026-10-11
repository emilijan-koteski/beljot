package wallet

// DailyLoginResult is the outcome of a daily-login evaluation, serialized as the daily-login response body.
type DailyLoginResult struct {
	Granted         bool `json:"granted"`
	Amount          int  `json:"amount"`
	StreakDay       int  `json:"streakDay"`
	NewBalance      int  `json:"newBalance"`
	LoginStreakDays int  `json:"loginStreakDays"`
}
