package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/text/unicode/norm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/identity"
	"github.com/emilijan/beljot/server/internal/user"
)

// ssoUsernameStripRegex removes everything a username may not contain; the
// complement of the usernameRegex alphabet used by Register.
var ssoUsernameStripRegex = regexp.MustCompile(`[^a-zA-Z0-9_]`)

const (
	// ssoUsernameMaxSequentialAttempts caps the base, base1, base2, …
	// uniquification probes — each one is a FindByUsername round-trip, so the
	// sequential walk must stay short.
	ssoUsernameMaxSequentialAttempts = 20
	// ssoUsernameMaxRandomAttempts bounds the random-suffix fallback used once
	// the sequential space is crowded; collisions there are astronomically
	// unlikely, the bound just guarantees termination.
	ssoUsernameMaxRandomAttempts = 20
	// ssoRegisterMaxAttempts bounds create retries when the generated username
	// loses an insert race (unique violation despite the FindByUsername probe).
	ssoRegisterMaxAttempts = 3
	// ssoCredentialMaxLen is a cheap upper bound on the raw credential — real
	// Google ID tokens are well under this; anything larger is garbage we must
	// not ship to the verifier.
	ssoCredentialMaxLen = 4096
	// ssoEmailMaxLen matches the users.email column width.
	ssoEmailMaxLen = 255
)

type SSOLoginRequest struct {
	Credential string `json:"credential"`
}

type SSOLinkRequest struct {
	Credential string `json:"credential"`
	Password   string `json:"password"`
}

// SSOLogin handles POST /auth/sso/:provider — one endpoint for both SSO login
// and SSO registration, decided by whether the verified (provider, subject)
// identity already exists. An email collision with a local account is never
// auto-linked: it returns SSO_LINK_REQUIRED and the client confirms with the
// account password via SSOLink.
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
	// A verifier bug aside, a subject-less identity is nothing we can match or
	// register — refuse it before it touches any lookup.
	if ext.Subject == "" {
		return apperr.ErrSSOInvalidCredential
	}

	// Known identity → plain login, before any email gating: the email was
	// verified when the link was made, and Google-side flag churn must not
	// lock an already-linked player out.
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
			// Identity outlived its (soft-deleted) user — treat as a dead login,
			// not as a fresh registration slot for a taken email.
			return apperr.ErrInvalidCredentials
		}
		return h.issueSession(c, u, http.StatusOK)
	}

	// From here on we act on the provider-reported email, so it must be one
	// the provider itself has verified — and shaped like something the users
	// table can actually hold.
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
		// Never auto-link on email match alone — the owner must confirm with
		// the account's password (SSOLink). No identity row is created here.
		return apperr.ErrSSOLinkRequired
	}

	u, err := h.createSSOUser(email, ext.DisplayName)
	if err != nil {
		return err
	}
	if err := h.identityRepo.Create(&identity.Identity{
		UserID:         u.ID,
		Provider:       provider.Name(),
		ProviderUserID: ext.Subject,
		Email:          email,
	}); err != nil {
		// Compensate: without its identity row the passwordless user is
		// unreachable and its email is bricked (SSOLink rejects the empty
		// password-hash sentinel), so soft-delete the just-created user — the
		// partial unique indexes free the email/username again.
		if delErr := h.userRepo.Delete(u.ID); delErr != nil {
			slog.Error("failed to compensate orphaned sso user", "error", delErr, "userId", u.ID)
		}
		return fmt.Errorf("creating identity: %w", err)
	}
	return h.issueSession(c, u, http.StatusCreated)
}

// createSSOUser inserts the passwordless user row for an SSO registration,
// mirroring Register's seeding exactly (wallet 5000, streak 0, last_login
// stamped today UTC — see Register for the rationale) with a generated
// username and the empty-string password-hash sentinel meaning "no password
// set". Insert-time unique violations — which the pre-checks cannot rule out
// under concurrency — are handled here: a lost email race answers exactly like
// the pre-check (ErrSSOLinkRequired), and a lost username race regenerates and
// retries a bounded number of times.
func (h *AuthHandler) createSSOUser(email, displayName string) (*user.User, error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var lastErr error
	for attempt := 0; attempt < ssoRegisterMaxAttempts; attempt++ {
		username, err := h.generateSSOUsername(displayName, email)
		if err != nil {
			return nil, err
		}
		u := &user.User{
			Email:              email,
			Username:           username,
			PasswordHash:       "",
			LanguagePreference: "en",
			WalletBalance:      5000,
			LoginStreakDays:    0,
			LastLoginAt:        &today,
		}
		err = h.userRepo.Create(u)
		switch {
		case err == nil:
			return u, nil
		case errors.Is(err, apperr.ErrEmailTaken):
			// A concurrent password registration (or a duplicate SSO first
			// login) won the email — same answer as the pre-check: link.
			return nil, apperr.ErrSSOLinkRequired
		case errors.Is(err, apperr.ErrUsernameTaken):
			lastErr = err // regenerate and retry
		default:
			return nil, fmt.Errorf("creating user: %w", err)
		}
	}
	// Deliberately %v, not %w: exhausting the retries is an internal failure
	// (500), and preserving ErrUsernameTaken in the chain would misreport it
	// as a 409 for a username the player never chose.
	return nil, fmt.Errorf("creating sso user: username collided %d times: %v", ssoRegisterMaxAttempts, lastErr)
}

// SSOLink handles POST /auth/sso/:provider/link — links a verified SSO
// identity to the existing local account with the same email, gated on that
// account's password. A missing account, a passwordless (SSO-only) account,
// and a wrong password are deliberately indistinguishable (all 401
// INVALID_CREDENTIALS) so the endpoint is no better an oracle than login.
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

	// A concurrent link/registration race surfaces here as the repo's mapped
	// unique-violation (ErrSSOIdentityInUse) — nothing to unwind, the identity
	// row is the only write.
	if err := h.identityRepo.Create(&identity.Identity{
		UserID:         u.ID,
		Provider:       provider.Name(),
		ProviderUserID: ext.Subject,
		Email:          email,
	}); err != nil {
		if errors.Is(err, apperr.ErrSSOIdentityInUse) {
			// Idempotency: a retry after a lost response (or a concurrent
			// duplicate of the same link) conflicts on a row that already says
			// exactly what we were about to write. If the (provider, subject)
			// identity belongs to THIS user, the link exists — succeed.
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

// validateSSOCredential rejects obviously unusable credentials before any
// verifier round-trip: an empty/whitespace body is a client bug (and would
// otherwise slog.Warn per request), an oversized one is garbage no real ID
// token ever reaches.
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

// generateSSOUsername derives a unique username for an SSO registration:
// sanitize the display name to the username alphabet, fall back to the email
// local-part, then to "player"; clamp to the 3-20 length window and uniquify
// with a numeric suffix.
func (h *AuthHandler) generateSSOUsername(displayName, email string) (string, error) {
	base := ssoUsernameStripRegex.ReplaceAllString(displayName, "")
	if len(base) < 3 {
		local := email
		if at := strings.Index(email, "@"); at >= 0 {
			local = email[:at]
		}
		base = ssoUsernameStripRegex.ReplaceAllString(local, "")
	}
	if len(base) < 3 {
		base = "player"
	}
	if len(base) > 20 {
		base = base[:20]
	}

	candidate := base
	for i := 1; i <= ssoUsernameMaxSequentialAttempts; i++ {
		existing, err := h.userRepo.FindByUsername(candidate)
		if err != nil {
			return "", fmt.Errorf("checking username: %w", err)
		}
		if existing == nil {
			return candidate, nil
		}
		candidate = suffixUsername(base, strconv.Itoa(i))
	}

	// The sequential space is crowded — switch to random numeric suffixes so a
	// popular base costs O(1) probes instead of a linear scan.
	for i := 0; i < ssoUsernameMaxRandomAttempts; i++ {
		candidate = suffixUsername(base, strconv.Itoa(rand.IntN(100_000_000)))
		existing, err := h.userRepo.FindByUsername(candidate)
		if err != nil {
			return "", fmt.Errorf("checking username: %w", err)
		}
		if existing == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free username after %d attempts for base %q",
		ssoUsernameMaxSequentialAttempts+ssoUsernameMaxRandomAttempts, base)
}

// suffixUsername appends a numeric suffix to base, trimming base so the result
// stays within the 20-char username cap. base is always >= 3 chars and the
// suffix at most 8 digits, so the result stays inside the 3-20 window.
func suffixUsername(base, suffix string) string {
	if len(base)+len(suffix) > 20 {
		base = base[:20-len(suffix)]
	}
	return base + suffix
}
