package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/idtoken"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// withValidate returns a GoogleProvider whose idtoken.Validate is stubbed —
// claim mapping and the issuer gate are testable without real Google tokens.
func withValidate(fn func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error)) *GoogleProvider {
	p := NewGoogleProvider("test-client-id")
	p.validate = fn
	return p
}

func googlePayload(issuer string, claims map[string]interface{}) *idtoken.Payload {
	return &idtoken.Payload{
		Issuer:  issuer,
		Subject: "google-sub-1",
		Claims:  claims,
	}
}

func TestGoogleVerify_MapsClaims(t *testing.T) {
	p := withValidate(func(_ context.Context, _, audience string) (*idtoken.Payload, error) {
		assert.Equal(t, "test-client-id", audience, "audience must be the configured client id")
		return googlePayload("https://accounts.google.com", map[string]interface{}{
			"email":          "player@example.com",
			"email_verified": true,
			"name":           "Player One",
		}), nil
	})

	ext, err := p.Verify(context.Background(), "credential")
	require.NoError(t, err)
	assert.Equal(t, "google-sub-1", ext.Subject)
	assert.Equal(t, "player@example.com", ext.Email)
	assert.True(t, ext.EmailVerified)
	assert.Equal(t, "Player One", ext.DisplayName)
}

func TestGoogleVerify_AcceptsBothIssuerForms(t *testing.T) {
	for _, issuer := range []string{"accounts.google.com", "https://accounts.google.com"} {
		p := withValidate(func(_ context.Context, _, _ string) (*idtoken.Payload, error) {
			return googlePayload(issuer, map[string]interface{}{
				"email":          "player@example.com",
				"email_verified": true,
			}), nil
		})
		_, err := p.Verify(context.Background(), "credential")
		assert.NoError(t, err, "issuer %q must be accepted", issuer)
	}
}

func TestGoogleVerify_RejectsWrongIssuer(t *testing.T) {
	p := withValidate(func(_ context.Context, _, _ string) (*idtoken.Payload, error) {
		return googlePayload("https://evil.example.com", map[string]interface{}{
			"email":          "player@example.com",
			"email_verified": true,
		}), nil
	})

	_, err := p.Verify(context.Background(), "credential")
	assert.ErrorIs(t, err, apperr.ErrSSOInvalidCredential)
}

func TestGoogleVerify_RejectsValidationFailure(t *testing.T) {
	p := withValidate(func(_ context.Context, _, _ string) (*idtoken.Payload, error) {
		return nil, errors.New("token expired")
	})

	_, err := p.Verify(context.Background(), "credential")
	assert.ErrorIs(t, err, apperr.ErrSSOInvalidCredential)
}

func TestGoogleVerify_RejectsMissingEmail(t *testing.T) {
	p := withValidate(func(_ context.Context, _, _ string) (*idtoken.Payload, error) {
		return googlePayload("https://accounts.google.com", map[string]interface{}{
			"email_verified": true,
		}), nil
	})

	_, err := p.Verify(context.Background(), "credential")
	assert.ErrorIs(t, err, apperr.ErrSSOInvalidCredential)
}

func TestGoogleVerify_StringEmailVerifiedClaim(t *testing.T) {
	p := withValidate(func(_ context.Context, _, _ string) (*idtoken.Payload, error) {
		return googlePayload("https://accounts.google.com", map[string]interface{}{
			"email":          "player@example.com",
			"email_verified": "true",
		}), nil
	})

	ext, err := p.Verify(context.Background(), "credential")
	require.NoError(t, err)
	assert.True(t, ext.EmailVerified, "string-typed email_verified must still count as verified")
}
