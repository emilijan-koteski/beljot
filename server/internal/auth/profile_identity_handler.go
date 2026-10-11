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

// LinkedAccountsResponse is the body of GET /users/:id/identities.
// HasPassword tells the client whether the account can sign in without any linked identity.
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

// LinkIdentity links a verified SSO identity to the authenticated account, with no password gate.
// The provider email need not match the account email, unlike the login-collision merge.
func (h *AuthHandler) LinkIdentity(c echo.Context) error {
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

// UnlinkIdentity removes the user's identity for :provider, refusing to remove the last sign-in method.
// The provider is not checked against the registry, so stale links of removed providers can be unlinked.
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
	if u.PasswordHash == "" && len(idents) <= 1 {
		return apperr.ErrSSOCannotUnlinkLast
	}

	deleted, err := h.identityRepo.DeleteByUserProvider(authUserID, provider)
	if err != nil {
		return fmt.Errorf("unlinking identity: %w", err)
	}
	if deleted == 0 {
		return apperr.ErrSSOIdentityNotFound
	}
	return c.NoContent(http.StatusNoContent)
}
