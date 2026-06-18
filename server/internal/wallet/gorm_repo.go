package wallet

import (
	"errors"
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
