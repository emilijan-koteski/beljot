package identity

import "context"

// ExternalIdentity is the provider-agnostic claim set extracted from a
// verified SSO credential. Subject is the provider's stable user id; Email is
// as reported by the provider (trust it only when EmailVerified is true);
// DisplayName seeds username generation and may be empty.
type ExternalIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

// Provider verifies one SSO provider's credentials. Implementations must do
// full server-side validation (signature, audience, issuer, expiry) and return
// apperr.ErrSSOInvalidCredential for anything that fails — logging details but
// never leaking them to the client. Adding a provider to the platform is a new
// implementation plus a Registry entry (plus an i18n label); no schema or
// endpoint changes.
type Provider interface {
	Name() string
	Verify(ctx context.Context, credential string) (*ExternalIdentity, error)
}

// Registry maps a provider's route name (the :provider path param) to its
// implementation. Built once in main.go from config; providers with missing
// config are simply not registered.
type Registry map[string]Provider
