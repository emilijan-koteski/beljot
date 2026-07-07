package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/identity"
)

// LinkIdentityRequest is the body of POST /users/:id/identities/:provider — the
// raw GIS credential, verified server-side. Unlike the login-time link path
// (SSOLink), no password is required: the JWT already proves account ownership.
type LinkIdentityRequest struct {
	Credential string `json:"credential"`
}

// IdentityView is the safe per-identity projection for the profile — never the
// internal row id, user id, or provider subject (Google `sub`).
type IdentityView struct {
	Provider  string    `json:"provider"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

// LinkedAccountsResponse is the body of GET /users/:id/identities. HasPassword
// tells the client whether the account can still be signed into without any
// linked identity — it drives the "cannot unlink your last sign-in method" UX
// and mirrors the guard the server enforces on unlink.
type LinkedAccountsResponse struct {
	HasPassword bool           `json:"hasPassword"`
	Identities  []IdentityView `json:"identities"`
}

// authorizeSelf runs the self-only guard shared by the profile identity
// endpoints: the caller must be authenticated and the :id path param must equal
// the authenticated user's id. Mirrors user.GetProfile exactly (401 / 400 / 403).
func authorizeSelf(c echo.Context) (uint, error) {
	authUserID, err := GetUserID(c)
	if err != nil {
		return 0, apperr.ErrUnauthorized
	}
	paramID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paramID == 0 {
		return 0, apperr.ErrBadRequest
	}
	if paramID != uint64(authUserID) {
		return 0, apperr.ErrForbidden
	}
	return authUserID, nil
}

// ListIdentities handles GET /users/:id/identities — returns the authenticated
// user's linked SSO identities plus whether the account has a password.
func (h *AuthHandler) ListIdentities(c echo.Context) error {
	authUserID, err := authorizeSelf(c)
	if err != nil {
		return err
	}
	u, err := h.userRepo.FindByID(authUserID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		return apperr.ErrUserNotFound
	}
	idents, err := h.identityRepo.FindByUserID(authUserID)
	if err != nil {
		return fmt.Errorf("listing identities: %w", err)
	}
	views := make([]IdentityView, 0, len(idents))
	for _, id := range idents {
		views = append(views, IdentityView{
			Provider:  id.Provider,
			Email:     id.Email,
			CreatedAt: id.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": LinkedAccountsResponse{
			HasPassword: u.PasswordHash != "",
			Identities:  views,
		},
	})
}

// LinkIdentity handles POST /users/:id/identities/:provider — links a verified
// SSO identity to the already-authenticated account. No password gate (unlike
// SSOLink): the JWT proves account ownership and the verified credential proves
// provider ownership. The provider email is deliberately NOT required to match
// the account email — this is the "connected accounts" flow, not the
// login-collision merge.
func (h *AuthHandler) LinkIdentity(c echo.Context) error {
	authUserID, err := authorizeSelf(c)
	if err != nil {
		return err
	}
	// Match the sibling endpoints' existence check: never create an identity
	// row for a soft-deleted (or otherwise absent) user — it would orphan the
	// row and squat the (provider, subject) unique slot for an unreachable
	// account.
	u, err := h.userRepo.FindByID(authUserID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		return apperr.ErrUserNotFound
	}
	var req LinkIdentityRequest
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

	newIdentity := &identity.Identity{
		UserID:         authUserID,
		Provider:       provider.Name(),
		ProviderUserID: ext.Subject,
		Email:          email,
	}
	if err := h.identityRepo.Create(newIdentity); err != nil {
		if errors.Is(err, apperr.ErrSSOIdentityInUse) {
			// Idempotency: a retried/double-tapped link conflicts on a row that
			// already says what we were about to write. If the (provider,
			// subject) identity already belongs to THIS user, the link exists —
			// succeed. Any other conflict (subject linked to another account, or
			// this user already linked a different account for this provider) is
			// a real 409.
			existing, findErr := h.identityRepo.FindByProviderSubject(provider.Name(), ext.Subject)
			if findErr != nil {
				return fmt.Errorf("re-checking identity: %w", findErr)
			}
			if existing != nil && existing.UserID == authUserID {
				return c.JSON(http.StatusOK, map[string]interface{}{
					"data": IdentityView{
						Provider:  existing.Provider,
						Email:     existing.Email,
						CreatedAt: existing.CreatedAt,
					},
				})
			}
		}
		return fmt.Errorf("linking identity: %w", err)
	}
	return c.JSON(http.StatusCreated, map[string]interface{}{
		"data": IdentityView{
			Provider:  newIdentity.Provider,
			Email:     newIdentity.Email,
			CreatedAt: newIdentity.CreatedAt,
		},
	})
}

// UnlinkIdentity handles DELETE /users/:id/identities/:provider — removes the
// authenticated user's linked identity for the given provider. It refuses to
// remove the account's only remaining sign-in method: a passwordless (SSO-only)
// account with a single identity would otherwise become unreachable. The
// :provider is matched directly (not resolved against the registry) so a
// de-registered provider's stale link can still be removed.
func (h *AuthHandler) UnlinkIdentity(c echo.Context) error {
	authUserID, err := authorizeSelf(c)
	if err != nil {
		return err
	}
	provider := c.Param("provider")

	u, err := h.userRepo.FindByID(authUserID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		return apperr.ErrUserNotFound
	}
	idents, err := h.identityRepo.FindByUserID(authUserID)
	if err != nil {
		return fmt.Errorf("listing identities: %w", err)
	}

	linked := false
	for _, id := range idents {
		if id.Provider == provider {
			linked = true
			break
		}
	}
	if !linked {
		return apperr.ErrSSOIdentityNotFound
	}
	// An account must retain at least one way to sign in. With a unique
	// (user_id, provider) index there is at most one identity per provider, so
	// a passwordless account holding a single identity would be locked out by
	// removing it.
	if u.PasswordHash == "" && len(idents) <= 1 {
		return apperr.ErrSSOCannotUnlinkLast
	}

	deleted, err := h.identityRepo.DeleteByUserProvider(authUserID, provider)
	if err != nil {
		return fmt.Errorf("unlinking identity: %w", err)
	}
	if deleted == 0 {
		// Lost a race with a concurrent unlink of the same provider — the end
		// state (no such link) matches the request, so report it as not-found.
		return apperr.ErrSSOIdentityNotFound
	}
	return c.NoContent(http.StatusNoContent)
}
