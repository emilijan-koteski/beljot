package identity

import "time"

// Identity links a local user to one external SSO account, keyed by the provider's stable subject.
// Email is a link-time snapshot for support only; login matches on (provider, provider_user_id).
type Identity struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"column:user_id;not null" json:"userId"`
	Provider       string    `gorm:"column:provider;not null" json:"provider"`
	ProviderUserID string    `gorm:"column:provider_user_id;not null" json:"providerUserId"`
	Email          string    `gorm:"column:email;not null" json:"email"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TableName maps Identity to the user_identities table.
func (Identity) TableName() string {
	return "user_identities"
}
