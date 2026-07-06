package identity

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/api/idtoken"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// googleVerifyTimeout bounds a single token verification, JWKS fetch included.
// Without it a hung Google endpoint would wedge handler goroutines for as long
// as the client keeps the request open.
const googleVerifyTimeout = 10 * time.Second

// GoogleProvider verifies Google Identity Services ID tokens. Verification is
// fully server-side via idtoken.Validate (signature against Google's JWKS,
// audience == our OAuth client ID, expiry) plus an explicit issuer check —
// idtoken.Validate does not pin the issuer itself.
type GoogleProvider struct {
	clientID string
	// validate is idtoken.Validate, injectable so tests can exercise the
	// claim-mapping and issuer gate without real Google tokens.
	validate func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error)
}

func NewGoogleProvider(clientID string) *GoogleProvider {
	return &GoogleProvider{clientID: clientID, validate: idtoken.Validate}
}

func (p *GoogleProvider) Name() string {
	return "google"
}

// Verify validates the ID token and maps Google's claims onto the
// provider-agnostic ExternalIdentity. Every failure collapses to
// apperr.ErrSSOInvalidCredential — expired, forged, wrong-audience, and
// malformed tokens are deliberately indistinguishable to the client; the
// specifics are logged here.
func (p *GoogleProvider) Verify(ctx context.Context, credential string) (*ExternalIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, googleVerifyTimeout)
	defer cancel()
	payload, err := p.validate(ctx, credential, p.clientID)
	if err != nil {
		slog.Warn("google id-token validation failed", "error", err)
		return nil, apperr.ErrSSOInvalidCredential
	}
	if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
		slog.Warn("google id-token has unexpected issuer", "issuer", payload.Issuer)
		return nil, apperr.ErrSSOInvalidCredential
	}

	ext := &ExternalIdentity{
		Subject:       payload.Subject,
		Email:         stringClaim(payload.Claims, "email"),
		EmailVerified: boolClaim(payload.Claims, "email_verified"),
		DisplayName:   stringClaim(payload.Claims, "name"),
	}
	// A GIS sign-in token always carries sub + email; their absence means this
	// is not a token we can authenticate with.
	if ext.Subject == "" || ext.Email == "" {
		slog.Warn("google id-token missing subject or email claim")
		return nil, apperr.ErrSSOInvalidCredential
	}
	return ext, nil
}

func stringClaim(claims map[string]interface{}, key string) string {
	s, _ := claims[key].(string)
	return s
}

// boolClaim tolerates the string form ("true") some IdPs emit for boolean
// claims; anything else reads as false — the safe default for email_verified.
func boolClaim(claims map[string]interface{}, key string) bool {
	switch v := claims[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}
