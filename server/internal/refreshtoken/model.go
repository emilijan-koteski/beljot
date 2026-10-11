package refreshtoken

import "time"

// RefreshToken is one link in the rotation chain of a login session ("family"), stored as a SHA-256 hash.
// ExpiresAt is the sliding idle deadline, capped by the fixed FamilyExpiresAt set at login.
type RefreshToken struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	UserID          uint       `gorm:"column:user_id;not null" json:"userId"`
	FamilyID        string     `gorm:"column:family_id;not null" json:"familyId"`
	TokenHash       string     `gorm:"column:token_hash;not null" json:"-"`
	ExpiresAt       time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	FamilyExpiresAt time.Time  `gorm:"column:family_expires_at;not null" json:"familyExpiresAt"`
	RotatedAt       *time.Time `gorm:"column:rotated_at" json:"rotatedAt,omitempty"`
	RevokedAt       *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}
