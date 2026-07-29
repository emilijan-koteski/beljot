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
	TotalXP int `gorm:"not null;default:0" json:"totalXp"`
	// UsernameChangedAt records when the user last changed their username. It is
	// a pointer because it is nullable — NULL (never changed) serializes as null,
	// not time.Time's "0001-01-01T00:00:00Z" zero value. Drives the 30-day change
	// cooldown (see UsernameChangeCooldownDays). DB column is TIMESTAMPTZ.
	UsernameChangedAt *time.Time `gorm:"column:username_changed_at" json:"usernameChangedAt,omitempty"`
	// Honor score storage (Story 9.7, migration 000017). Six columns, all
	// json:"-" — the API never serializes the raw storage, it serializes the
	// computed DTO fields in ProfileResponse / RegisterResponseData.
	//
	// HonorCompletedWeight / HonorAbandonedWeight are DECAYED running weights:
	// each finished match contributes 0.5^(age_days/90), so old events fade.
	// They are only meaningful together with HonorDecayedAt, the timestamp they
	// were last decayed to. DB type is NUMERIC(14,6), not a float, because
	// binary float drift in an access-gating trust signal is unacceptable.
	HonorCompletedWeight float64 `gorm:"column:honor_completed_weight;not null;default:0" json:"-"`
	HonorAbandonedWeight float64 `gorm:"column:honor_abandoned_weight;not null;default:0" json:"-"`
	// HonorDecayedAt is a pointer because the column is nullable: NULL means
	// "never decayed" and DecayFactor treats it as exactly 1.0. (Also avoids
	// time.Time's zero value serializing as "0001-01-01T00:00:00Z".)
	HonorDecayedAt *time.Time `gorm:"column:honor_decayed_at" json:"-"`
	// Raw, UNDECAYED lifetime counts. Their SUM drives the "New Player"
	// suppression: IsNewPlayer is (completed + abandoned) < 5, i.e. the floor
	// counts EXPERIENCE, not successes. Do NOT gate on HonorCompletedTotal alone
	// — that let a 0-completed / 20-abandoned account (real score 5,
	// "problematic") hide behind the newcomer chip forever, and Story 9.8's join
	// gate reads isNewPlayer off the same envelope. Always compare against these
	// raw totals, never the decayed weights, or a returning veteran gets
	// relabelled a newcomer. BIGINT columns, width-matched to these 64-bit Go
	// ints.
	//
	// HonorCompletedTotal deliberately SURVIVES ResetHonor — a pardon clears the
	// penalty, not the experience. See ResetHonor in gorm_repo.go.
	HonorCompletedTotal int64 `gorm:"column:honor_completed_total;not null;default:0" json:"-"`
	HonorAbandonedTotal int64 `gorm:"column:honor_abandoned_total;not null;default:0" json:"-"`
	// HonorScoreSnapshot is the DENORMALIZED honor_score column. It exists ONLY
	// so operators can filter/sort in SQL (WHERE honor_score < 50) and it is
	// ALLOWED TO LAG — decay means the true score moves as time passes even
	// when nothing is written.
	//
	// NEVER render this and NEVER gate on it. The authoritative value is always
	// HonorScore(HonorCompletedWeight, HonorAbandonedWeight, HonorDecayedAt,
	// now) — pure arithmetic on a row you have already loaded. The Go field is
	// deliberately named ...Snapshot so a misuse reads wrong at the call site.
	HonorScoreSnapshot int            `gorm:"column:honor_score;not null;default:80" json:"-"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}
