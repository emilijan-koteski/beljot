package user

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID                 uint   `gorm:"primaryKey" json:"id"`
	Email              string `gorm:"uniqueIndex;not null" json:"email"`
	Username           string `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash       string `gorm:"not null" json:"-"`
	LanguagePreference string `gorm:"default:en;not null" json:"languagePreference"`
	// Wallet fields (Story 9.1). State lives on the users table rather than a
	// dedicated wallet table; the wallet domain package owns the mutation logic.
	// WalletBalance default mirrors migration 000009 / wallet.StartingBalance.
	WalletBalance int `gorm:"not null;default:5000" json:"walletBalance"`
	// LastLoginAt is a pointer because it is nullable, and time.Time's zero
	// value would serialize as "0001-01-01T00:00:00Z" instead of null. DB column
	// is DATE; GORM reads/writes time.Time fine.
	LastLoginAt     *time.Time `gorm:"column:last_login_at" json:"lastLoginAt,omitempty"`
	LoginStreakDays int        `gorm:"column:login_streak_days;not null;default:0" json:"loginStreakDays"`
	// TotalXP is the player's lifetime experience-point total (Story 9.5). The
	// LEVEL is derived from it (user.LevelForXP), never stored. GORM default +
	// camelCase JSON tag. DB column total_xp is BIGINT (a lifetime accumulator,
	// width-matched to this 64-bit Go int) with a CHECK (total_xp >= 0); XP only
	// ever accrues. (Unlike WalletBalance, which is a bounded INTEGER balance.)
	TotalXP   int            `gorm:"not null;default:0" json:"totalXp"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
