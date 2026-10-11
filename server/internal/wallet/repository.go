package wallet

import "time"

// Repository is the persistence boundary for wallet mutations on user rows.
// Multi-user writes run in one transaction with rows locked FOR UPDATE in ascending userID order.
type Repository interface {
	ProcessDailyLogin(userID uint, today time.Time) (DailyLoginResult, error)
	ChargeStakes(userIDs []uint, amount int) (insolventUserID uint, err error)
	ApplySettlement(credits map[uint]int) error
	GetBalance(userID uint) (int, error)
	GetBalances(userIDs []uint) (map[uint]int, error)
}
