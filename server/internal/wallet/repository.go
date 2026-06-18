package wallet

import "time"

// Repository is the persistence boundary for wallet mutations. The handler/
// service depend on this interface, never on GORM directly. Wallet state lives
// on the users table, so the implementation reads/writes user.User rows.
type Repository interface {
	// ProcessDailyLogin atomically evaluates and applies the daily-login bonus
	// for userID using today (UTC) as the reference calendar day. The whole
	// read-modify-write runs inside one transaction with the user row locked
	// FOR UPDATE, so two concurrent calls on a new day grant exactly once
	// (AC #3, #6). When the bonus was already claimed today it grants nothing
	// and returns the current balance/streak with Granted=false.
	ProcessDailyLogin(userID uint, today time.Time) (DailyLoginResult, error)
}
