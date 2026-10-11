package identity

// Repository is the persistence boundary for external SSO identities.
// Lookups return (nil, nil) on no match; unique violations surface as apperr.ErrSSOIdentityInUse.
type Repository interface {
	Create(identity *Identity) error
	FindByProviderSubject(provider, subject string) (*Identity, error)
	FindByUserID(userID uint) ([]Identity, error)
	DeleteByUserProvider(userID uint, provider string) (int64, error)
}
