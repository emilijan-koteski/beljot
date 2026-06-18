// Package wallet owns the player coin-wallet logic introduced in Story 9.1.
//
// Unlike the other domain packages (user, room, passwordreset), wallet has no
// table of its own: its persistent state — balance, last-login date, and login
// streak — lives on the users table (see migration 000009). The package still
// exists for cohesion: it concentrates the atomic mutation, the streak/bonus
// math, and the daily-login handler in one place (AC #6).
//
// Import direction: wallet may import user (for user.User); user must NOT import
// wallet — that would create an import cycle.
package wallet

// DailyLoginResult is the outcome of a daily-login evaluation. It is returned by
// the service/repo and serialized verbatim by the handler as the body of the
// POST /api/v1/wallet/daily-login response (inside the { "data": ... } envelope).
//
// On a grant, StreakDay and LoginStreakDays are equal (the newly-incremented
// streak); on a no-grant they both echo the current stored streak. Both are
// included because the wire contract names both fields.
type DailyLoginResult struct {
	Granted         bool `json:"granted"`
	Amount          int  `json:"amount"`
	StreakDay       int  `json:"streakDay"`
	NewBalance      int  `json:"newBalance"`
	LoginStreakDays int  `json:"loginStreakDays"`
}
