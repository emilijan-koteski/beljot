package passwordreset

import "time"

// PasswordResetToken is a single-use, time-limited token backing the
// forgot-password flow. Only the SHA-256 hash of the token is persisted; the
// raw token lives exclusively in the emailed reset link. The table name
// resolves to "password_reset_tokens" via GORM's default pluralization.
//
// The struct intentionally omits UpdatedAt/DeletedAt: tokens are immutable once
// issued (other than the one-time UsedAt stamp) and are hard-deleted, so there
// is no soft-delete column for GORM to manage.
type PasswordResetToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"column:user_id;not null" json:"userId"`
	TokenHash string     `gorm:"column:token_hash;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	UsedAt    *time.Time `gorm:"column:used_at" json:"usedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}
