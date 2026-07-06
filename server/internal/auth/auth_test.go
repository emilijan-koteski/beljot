package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword_ProducesValidBcryptHash(t *testing.T) {
	hash, err := HashPassword("testpassword123")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "testpassword123", hash)
	// bcrypt hashes start with $2a$ or $2b$
	assert.Regexp(t, `^\$2[ab]\$`, hash)
}

func TestCheckPassword_SucceedsWithCorrectPassword(t *testing.T) {
	hash, err := HashPassword("correctpassword")
	require.NoError(t, err)

	err = CheckPassword(hash, "correctpassword")
	assert.NoError(t, err)
}

func TestCheckPassword_FailsWithWrongPassword(t *testing.T) {
	hash, err := HashPassword("correctpassword")
	require.NoError(t, err)

	err = CheckPassword(hash, "wrongpassword")
	assert.Error(t, err)
}

func TestGenerateAccessToken_AndValidateRoundTrip(t *testing.T) {
	secret := "test-secret-key"
	userID := uint(42)

	token, err := GenerateAccessToken(userID, secret)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := ValidateToken(token, secret)
	require.NoError(t, err)
	assert.Equal(t, "42", claims.Subject)
	assert.Contains(t, []string(claims.Audience), "access")
	assert.NotNil(t, claims.ExpiresAt)
	assert.NotNil(t, claims.IssuedAt)
}

func TestValidateToken_FailsWithWrongSecret(t *testing.T) {
	token, err := GenerateAccessToken(1, "correct-secret")
	require.NoError(t, err)

	_, err = ValidateToken(token, "wrong-secret")
	assert.Error(t, err)
}

func TestValidateToken_FailsWithInvalidTokenString(t *testing.T) {
	_, err := ValidateToken("not-a-valid-token", "secret")
	assert.Error(t, err)
}

func TestGenerateAccessToken_HasAccessAudience(t *testing.T) {
	secret := "test-secret"

	accessToken, err := GenerateAccessToken(1, secret)
	require.NoError(t, err)

	accessClaims, err := ValidateToken(accessToken, secret)
	require.NoError(t, err)
	assert.Contains(t, []string(accessClaims.Audience), "access")
	assert.NotContains(t, []string(accessClaims.Audience), "refresh")
}
