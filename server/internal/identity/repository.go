package identity

// Repository is the persistence boundary for external SSO identities.
// Handlers depend on this interface, never on GORM directly.
type Repository interface {
	// Create inserts a new identity link. A unique-constraint violation
	// (identity already linked, or the user already has an identity for this
	// provider) surfaces as apperr.ErrSSOIdentityInUse.
	Create(identity *Identity) error
	// FindByProviderSubject returns the identity for (provider, subject), or
	// (nil, nil) when no row matches.
	FindByProviderSubject(provider, subject string) (*Identity, error)
	// FindByUserID returns all identities linked to a user, oldest first. An
	// empty slice (not an error) means the user has no linked identities.
	FindByUserID(userID uint) ([]Identity, error)
	// DeleteByUserProvider hard-deletes the user's identity for the given
	// provider and returns the number of rows removed (0 when nothing was
	// linked). The row is hard-deleted — user_identities has no soft-delete —
	// so the freed (provider, provider_user_id) unique slot can be relinked.
	DeleteByUserProvider(userID uint, provider string) (int64, error)
}
