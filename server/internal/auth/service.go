package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// defaultAccessTokenTTL is the fallback access-token lifetime used by the
// convenience GenerateAccessToken wrapper (and thus by tests). Production wiring
// passes the configured value through GenerateAccessTokenWithTTL.
const defaultAccessTokenTTL = 15 * time.Minute

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(bytes), nil
}

func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateAccessToken mints a 15-minute access JWT. It is a thin wrapper over
// GenerateAccessTokenWithTTL kept so the many test call sites (and any caller
// that doesn't thread config) stay unchanged.
func GenerateAccessToken(userID uint, secret string) (string, error) {
	return GenerateAccessTokenWithTTL(userID, secret, defaultAccessTokenTTL)
}

// GenerateAccessTokenWithTTL mints an access JWT with an explicit lifetime.
// Claims and signing are identical to before — only the expiry is configurable.
func GenerateAccessTokenWithTTL(userID uint, secret string, ttl time.Duration) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		Audience:  jwt.ClaimStrings{"access"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateToken(tokenString, secret string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("validating token: %w", err)
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// mintRefreshToken returns a URL-safe raw refresh token (for the httpOnly
// cookie) and its SHA-256 hex hash (for storage). The raw token is never
// persisted — only the hash is, so a DB read cannot recover a usable token.
func mintRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("reading random bytes: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashRefreshToken(raw), nil
}

// hashRefreshToken maps a raw refresh token to the value stored in the DB.
func hashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// newFamilyID returns a random opaque identifier for a refresh-token family
// (one login/session lineage). 16 random bytes hex-encode to 32 chars, matching
// the family_id column width.
func newFamilyID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
