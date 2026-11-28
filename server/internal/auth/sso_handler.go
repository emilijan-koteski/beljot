package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/text/unicode/norm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/identity"
	"github.com/emilijan/beljot/server/internal/user"
)

const (
	ssoUsernamePrefix      = "player_"
	ssoUsernameAlphabet    = "abcdefghijklmnopqrstuvwxyz0123456789"
	ssoUsernameSuffixLen   = 6
	ssoUsernameMaxAttempts = 20
	ssoRegisterMaxAttempts = 3
	ssoCredentialMaxLen    = 4096
	ssoEmailMaxLen         = 255
)

type SSOLoginRequest struct {
	Credential string `json:"credential"`
}

type SSOLinkRequest struct {
	Credential string `json:"credential"`
	Password   string `json:"password"`
}

// SSOLogin handles POST /auth/sso/:provider, logging in a known identity or registering a new one.
// An email collision with a local account is never auto-linked; it returns SSO_LINK_REQUIRED for SSOLink.
func (h *AuthHandler) SSOLogin(c echo.Context) error {
	var req SSOLoginRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}
	provider, err := h.ssoProvider(c)
	if err != nil {
		return err
	}
	if err := validateSSOCredential(req.Credential); err != nil {
		return err
	}
	ext, err := provider.Verify(c.Request().Context(), req.Credential)
	if err != nil {
		return err
	}
	if ext.Subject == "" {
		return apperr.ErrSSOInvalidCredential
	}

	ident, err := h.identityRepo.FindByProviderSubject(provider.Name(), ext.Subject)
	if err != nil {
		return fmt.Errorf("finding identity: %w", err)
	}
	if ident != nil {
		u, err := h.userRepo.FindByID(ident.UserID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}
		if u == nil {
			return apperr.ErrInvalidCredentials
		}
		return h.issueSession(c, u, http.StatusOK)
	}

	if !ext.EmailVerified {
		return apperr.ErrSSOEmailUnverified
	}
	email := normalizeSSOEmail(ext.Email)
	if email == "" || len(email) > ssoEmailMaxLen {
		return apperr.ErrSSOInvalidCredential
	}

	existing, err := h.userRepo.FindByEmail(email)
	if err != nil {
		return fmt.Errorf("checking email: %w", err)
	}
	if existing != nil {
		return apperr.ErrSSOLinkRequired
	}

	u, err := h.createSSOUser(email)
	if err != nil {
		return err
	}
	if err := h.identityRepo.Create(&identity.Identity{
		UserID:         u.ID,
		Provider:       provider.Name(),
		ProviderUserID: ext.Subject,
		Email:          email,
	}); err != nil {
		if delErr := h.userRepo.Delete(u.ID); delErr != nil {
			slog.Error("failed to compensate orphaned sso user", "error", delErr, "userId", u.ID)
		}
		return fmt.Errorf("creating identity: %w", err)
	}
	return h.issueSession(c, u, http.StatusCreated)
}

// createSSOUser inserts a passwordless user seeded like Register, with a generated username.
// A lost email race returns ErrSSOLinkRequired; a lost username race regenerates and retries.
func (h *AuthHandler) createSSOUser(email string) (*user.User, error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var lastErr error
	for attempt := 0; attempt < ssoRegisterMaxAttempts; attempt++ {
		username, err := h.generateSSOUsername()
		if err != nil {
			return nil, err
		}
		u := &user.User{
			Email:                 email,
			Username:              username,
			PasswordHash:          "",
			LanguagePreference:    "en",
			CardDeckPreference:    user.CardDeckFrench,
			ThemePreference:       user.DefaultTheme,
			AllowNonFriendInvites: true,
			WalletBalance:         5000,
			LoginStreakDays:       0,
			LastLoginAt:           &today,
		}
		err = h.userRepo.Create(u)
		switch {
		case err == nil:
			return u, nil
		case errors.Is(err, apperr.ErrEmailTaken):
			return nil, apperr.ErrSSOLinkRequired
		case errors.Is(err, apperr.ErrUsernameTaken):
			lastErr = err
		default:
			return nil, fmt.Errorf("creating user: %w", err)
		}
	}
	return nil, fmt.Errorf("creating sso user: username collided %d times: %v", ssoRegisterMaxAttempts, lastErr)
}

// SSOLink handles POST /auth/sso/:provider/link, linking a verified identity to the same-email account after a password check.
// Missing accounts, passwordless accounts, and wrong passwords all answer 401 so it is no better an oracle than login.
func (h *AuthHandler) SSOLink(c echo.Context) error {
	var req SSOLinkRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}
	provider, err := h.ssoProvider(c)
	if err != nil {
		return err
	}
	if err := validateSSOCredential(req.Credential); err != nil {
		return err
	}
	ext, err := provider.Verify(c.Request().Context(), req.Credential)
	if err != nil {
		return err
	}
	if ext.Subject == "" {
		return apperr.ErrSSOInvalidCredential
	}
	if !ext.EmailVerified {
		return apperr.ErrSSOEmailUnverified
	}
	email := normalizeSSOEmail(ext.Email)
	if email == "" || len(email) > ssoEmailMaxLen {
		return apperr.ErrSSOInvalidCredential
	}

	u, err := h.userRepo.FindByEmail(email)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil || u.PasswordHash == "" || CheckPassword(u.PasswordHash, req.Password) != nil {
		return apperr.ErrInvalidCredentials
	}

	if err := h.identityRepo.Create(&identity.Identity{
		UserID:         u.ID,
		Provider:       provider.Name(),
		ProviderUserID: ext.Subject,
		Email:          email,
	}); err != nil {
		if errors.Is(err, apperr.ErrSSOIdentityInUse) {
			existing, findErr := h.identityRepo.FindByProviderSubject(provider.Name(), ext.Subject)
			if findErr != nil {
				return fmt.Errorf("re-checking identity: %w", findErr)
			}
			if existing != nil && existing.UserID == u.ID {
				return h.issueSession(c, u, http.StatusOK)
			}
		}
		return fmt.Errorf("linking identity: %w", err)
	}
	return h.issueSession(c, u, http.StatusOK)
}

// validateSSOCredential rejects blank or oversized credentials before any verifier round-trip.
func validateSSOCredential(credential string) error {
	if strings.TrimSpace(credential) == "" || len(credential) > ssoCredentialMaxLen {
		return apperr.ErrSSOInvalidCredential
	}
	return nil
}

// ssoProvider resolves the :provider path param against the registry.
func (h *AuthHandler) ssoProvider(c echo.Context) (identity.Provider, error) {
	p, ok := h.providers[c.Param("provider")]
	if !ok {
		return nil, apperr.ErrSSOUnknownProvider
	}
	return p, nil
}

// issueSession mints an access token, opens a fresh refresh-token family, and
// writes the standard auth envelope — the shared tail of Login and Register,
// so SSO sessions are indistinguishable from password sessions.
func (h *AuthHandler) issueSession(c echo.Context, u *user.User, status int) error {
	accessToken, err := GenerateAccessTokenWithTTL(u.ID, h.jwtSecret, h.accessTTL)
	if err != nil {
		return fmt.Errorf("generating access token: %w", err)
	}
	if err := h.startSession(c, u.ID); err != nil {
		return fmt.Errorf("starting session: %w", err)
	}
	return c.JSON(status, map[string]interface{}{
		"data": authResponseData(u, accessToken),
	})
}

// normalizeSSOEmail applies the same normalization Login applies to typed
// emails (trim, NFC, lowercase, RFC-5322 address extraction) so a provider
// email always compares equal to its locally-registered form.
func normalizeSSOEmail(raw string) string {
	email := strings.ToLower(norm.NFC.String(strings.TrimSpace(raw)))
	if addr, err := mail.ParseAddress(email); err == nil {
		email = strings.ToLower(norm.NFC.String(addr.Address))
	}
	return email
}

// generateSSOUsername picks a free neutral username, never one derived from the Google name or email.
// The first rename stays free because username_changed_at is left unset.
func (h *AuthHandler) generateSSOUsername() (string, error) {
	for i := 0; i < ssoUsernameMaxAttempts; i++ {
		candidate := ssoUsernamePrefix + h.usernameSuffix()
		existing, err := h.userRepo.FindByUsername(candidate)
		if err != nil {
			return "", fmt.Errorf("checking username: %w", err)
		}
		if existing == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free generated username after %d attempts", ssoUsernameMaxAttempts)
}

// randomUsernameSuffix returns ssoUsernameSuffixLen lowercase letters and digits.
func randomUsernameSuffix() string {
	b := make([]byte, ssoUsernameSuffixLen)
	for i := range b {
		b[i] = ssoUsernameAlphabet[rand.IntN(len(ssoUsernameAlphabet))]
	}
	return string(b)
}
