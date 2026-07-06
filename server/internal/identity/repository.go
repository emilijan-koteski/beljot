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
}
