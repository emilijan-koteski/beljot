package refreshtoken

import "time"

// RefreshToken is one link in a rotation chain that backs a single logged-in
// session ("family"). Only the SHA-256 hash of the opaque token is persisted;
// the raw value lives exclusively in the httpOnly refresh cookie. The table
// name resolves to "refresh_tokens" via GORM's default pluralization.
//
// A family is created at login and shares one FamilyID across every rotation.
// Exactly one token per family is "live" (RotatedAt == nil, RevokedAt == nil)
// at a time; refreshing rotates the live token and mints its successor.
//
// The struct intentionally omits UpdatedAt/DeletedAt: tokens are immutable once
// issued (other than the one-time RotatedAt/RevokedAt stamps) and are never
// soft-deleted, so there is no DeletedAt column for GORM to manage. Rows are
// retained (append-only) — periodic pruning of long-rotated/revoked rows is
// deferred (see _bmad-output/implementation-artifacts/deferred-work.md).
type RefreshToken struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UserID    uint   `gorm:"column:user_id;not null" json:"userId"`
	FamilyID  string `gorm:"column:family_id;not null" json:"familyId"`
	TokenHash string `gorm:"column:token_hash;not null" json:"-"`
	// ExpiresAt is the sliding idle deadline: each rotation pushes it out by the
	// idle TTL (capped at FamilyExpiresAt). FamilyExpiresAt is the absolute cap,
	// fixed at login and copied unchanged onto every successor.
	ExpiresAt       time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	FamilyExpiresAt time.Time  `gorm:"column:family_expires_at;not null" json:"familyExpiresAt"`
	RotatedAt       *time.Time `gorm:"column:rotated_at" json:"rotatedAt,omitempty"`
	RevokedAt       *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}
