package identity

import "time"

// Identity links a local user account to one external SSO provider account
// (e.g. a Google account). Provider is the registry name ("google"), and
// ProviderUserID is the provider's stable subject for that account (Google's
// `sub` claim) — never the email, which can change or be recycled.
//
// Email is a point-in-time snapshot of the provider email at link time, kept
// for support/debugging only; login matches exclusively on
// (provider, provider_user_id).
type Identity struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"column:user_id;not null" json:"userId"`
	Provider       string    `gorm:"column:provider;not null" json:"provider"`
	ProviderUserID string    `gorm:"column:provider_user_id;not null" json:"providerUserId"`
	Email          string    `gorm:"column:email;not null" json:"email"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TableName overrides GORM's default pluralization ("identities") — the table
// is user_identities (migration 000014).
func (Identity) TableName() string {
	return "user_identities"
}
