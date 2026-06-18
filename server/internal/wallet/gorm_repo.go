package wallet

import (
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/user"
)

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// ProcessDailyLogin wraps the read-modify-write in a single transaction and
// locks the user row FOR UPDATE before reading it. The lock makes the
// once-per-day guard race-free: two concurrent bootstraps serialize on the
// row, so the second sees the just-written last_login_at and grants nothing
// (AC #3, #6). Precedent: passwordreset.MarkUsed's atomic single-use guard.
func (r *GormRepository) ProcessDailyLogin(userID uint, today time.Time) (DailyLoginResult, error) {
	var result DailyLoginResult

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var u user.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.ErrUserNotFound
			}
			return err
		}

		grant, newStreak, amount := evaluateDailyLogin(u.LastLoginAt, u.LoginStreakDays, today)
		if !grant {
			// Already claimed today — touch nothing, echo current state.
			result = DailyLoginResult{
				Granted:         false,
				Amount:          0,
				StreakDay:       u.LoginStreakDays,
				NewBalance:      u.WalletBalance,
				LoginStreakDays: u.LoginStreakDays,
			}
			return nil
		}

		newBalance := u.WalletBalance + amount
		if err := tx.Model(&user.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
			"wallet_balance":    newBalance,
			"login_streak_days": newStreak,
			"last_login_at":     utcDate(today),
		}).Error; err != nil {
			return err
		}

		result = DailyLoginResult{
			Granted:         true,
			Amount:          amount,
			StreakDay:       newStreak,
			NewBalance:      newBalance,
			LoginStreakDays: newStreak,
		}
		return nil
	})
	if err != nil {
		return DailyLoginResult{}, err
	}
	return result, nil
}

// ChargeStakes debits `amount` from every userID atomically. Reuses the
// ProcessDailyLogin FOR UPDATE pattern: each row is locked, balance-guarded,
// then written, all inside one db.Transaction. Locking in ascending userID
// order makes concurrent charges deadlock-free. On the first insolvent row the
// transaction returns (and rolls back) with ErrInsufficientCoins, so the
// caller observes all-or-nothing — no partial debits.
func (r *GormRepository) ChargeStakes(userIDs []uint, amount int) (uint, error) {
	if amount <= 0 || len(userIDs) == 0 {
		return 0, nil
	}
	ordered := sortedUniqueUserIDs(userIDs)

	var insolvent uint
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ordered {
			var u user.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return apperr.ErrUserNotFound
				}
				return err
			}
			if u.WalletBalance < amount {
				insolvent = id
				return apperr.ErrInsufficientCoins
			}
			if err := tx.Model(&user.User{}).Where("id = ?", id).
				Update("wallet_balance", u.WalletBalance-amount).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return insolvent, err
	}
	return 0, nil
}

// ApplySettlement credits each (userID → amount) atomically, mirroring the
// ChargeStakes lock discipline (FOR UPDATE, ascending userID order) so a credit
// running concurrently with a charge cannot deadlock. Zero-amount entries are
// skipped. Rolls back on any failure.
func (r *GormRepository) ApplySettlement(credits map[uint]int) error {
	if len(credits) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(credits))
	for id := range credits {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			amount := credits[id]
			if amount == 0 {
				continue
			}
			var u user.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return apperr.ErrUserNotFound
				}
				return err
			}
			if err := tx.Model(&user.User{}).Where("id = ?", id).
				Update("wallet_balance", u.WalletBalance+amount).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetBalance reads a single user's wallet balance without locking.
func (r *GormRepository) GetBalance(userID uint) (int, error) {
	var u user.User
	if err := r.db.Select("wallet_balance").First(&u, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, apperr.ErrUserNotFound
		}
		return 0, err
	}
	return u.WalletBalance, nil
}

// GetBalances reads balances for many users in one query (userID → balance).
func (r *GormRepository) GetBalances(userIDs []uint) (map[uint]int, error) {
	out := make(map[uint]int, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []user.User
	if err := r.db.Select("id", "wallet_balance").Find(&rows, userIDs).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		out[rows[i].ID] = rows[i].WalletBalance
	}
	return out, nil
}

// sortedUniqueUserIDs returns the ascending, de-duplicated userID set used to
// impose a stable lock order across ChargeStakes/ApplySettlement.
func sortedUniqueUserIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}
