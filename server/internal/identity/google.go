package identity

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/api/idtoken"

	"github.com/emilijan/beljot/server/internal/apperr"
)

const googleVerifyTimeout = 10 * time.Second

// GoogleProvider verifies Google Identity Services ID tokens server-side.
// idtoken.Validate checks signature, audience and expiry; the issuer is checked explicitly.
type GoogleProvider struct {
	clientID string
	validate func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error)
}

func NewGoogleProvider(clientID string) *GoogleProvider {
	return &GoogleProvider{clientID: clientID, validate: idtoken.Validate}
}

func (p *GoogleProvider) Name() string {
	return "google"
}

// Verify validates the ID token and maps Google's claims onto ExternalIdentity.
// Every failure returns apperr.ErrSSOInvalidCredential so the client cannot tell causes apart.
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

// boolClaim reads a boolean claim, accepting the string form "true"; anything else is false.
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
