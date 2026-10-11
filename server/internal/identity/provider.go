package identity

import "context"

// ExternalIdentity is the provider-agnostic claim set from a verified SSO credential.
// Trust Email only when EmailVerified is true.
type ExternalIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

// Provider fully validates one SSO provider's credentials server-side.
// Any failure returns apperr.ErrSSOInvalidCredential without leaking details to the client.
type Provider interface {
	Name() string
	Verify(ctx context.Context, credential string) (*ExternalIdentity, error)
}

// Registry maps a provider's route name (the :provider path param) to its implementation.
type Registry map[string]Provider
