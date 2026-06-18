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

	// ChargeStakes atomically debits `amount` from every userID in ONE
	// transaction, locking rows FOR UPDATE in ascending userID order to avoid
	// deadlocks between concurrent charges (Story 9.2 match-start charge). If
	// any user is insolvent (wallet_balance < amount) the whole transaction
	// rolls back and (offendingUserID, apperr.ErrInsufficientCoins) is returned
	// — never a partial debit. amount <= 0 or an empty slice is a no-op success.
	// Callers pass HUMAN userIDs only (bot seats carry UserID 0).
	ChargeStakes(userIDs []uint, amount int) (insolventUserID uint, err error)

	// ApplySettlement atomically credits each (userID → amount) in ONE
	// transaction, rows locked FOR UPDATE in ascending userID order (Story 9.2
	// match-end settlement). Rolls back + returns on any failure. An empty map
	// is a no-op success (the no-human-winner sink, or a no-economy match).
	ApplySettlement(credits map[uint]int) error

	// GetBalance reads a user's current wallet balance (plain read, no lock).
	// Used for the cosmetic join-time affordability check; the authoritative
	// guard is ChargeStakes' FOR UPDATE re-validation at match start.
	GetBalance(userID uint) (int, error)

	// GetBalances reads current balances for the given userIDs in one query,
	// returning a userID → balance map (missing IDs are absent). Used after
	// settlement to populate each human's newBalance in event:coin_settlement.
	GetBalances(userIDs []uint) (map[uint]int, error)
}
