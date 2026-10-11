package passwordreset

import "time"

// PasswordResetToken is a single-use, time-limited forgot-password token.
// Only its SHA-256 hash is stored; rows are immutable and hard-deleted.
type PasswordResetToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"column:user_id;not null" json:"userId"`
	TokenHash string     `gorm:"column:token_hash;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	UsedAt    *time.Time `gorm:"column:used_at" json:"usedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}
