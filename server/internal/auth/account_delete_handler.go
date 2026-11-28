package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/account"
	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/user"
)

// DeleteAccountRequest confirms an erasure: the password for an account that has one, else the current username.
type DeleteAccountRequest struct {
	Password string `json:"password"`
	Username string `json:"username"`
}

// AccountEraser is the slice of account.Repository the delete handler needs.
type AccountEraser interface {
	InActiveRoom(userID uint) (bool, error)
	Erase(userID uint) (account.Erased, error)
}

// SessionCloser closes a user's live WebSocket connection.
type SessionCloser interface {
	DisconnectUser(userID uint)
}

// AvatarRemover deletes a stored avatar pair, best-effort, logging failures with the user id.
type AvatarRemover interface {
	DeleteStored(ctx context.Context, userID uint, prefix, reason string)
}

// AccountDeletionHandler serves self-service account erasure.
type AccountDeletionHandler struct {
	users    user.UserRepository
	eraser   AccountEraser
	sessions SessionCloser
	avatars  AvatarRemover
	revoked  *RevokedUsers
	env      string
}

// NewAccountDeletionHandler wires the erasure endpoint; avatars may be nil when avatar storage is off.
func NewAccountDeletionHandler(users user.UserRepository, eraser AccountEraser, sessions SessionCloser, avatars AvatarRemover, revoked *RevokedUsers, env string) *AccountDeletionHandler {
	return &AccountDeletionHandler{users: users, eraser: eraser, sessions: sessions, avatars: avatars, revoked: revoked, env: env}
}

// DeleteAccount handles DELETE /users/:id: it anonymises the caller's account, then ends every session it had.
// It is refused while the caller sits in a waiting or playing room, and needs the password or, without one, the username.
func (h *AccountDeletionHandler) DeleteAccount(c echo.Context) error {
	userID, err := authorizeSelf(c)
	if err != nil {
		return err
	}
	var req DeleteAccountRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}
	u, err := h.users.FindByID(userID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		return apperr.ErrUserNotFound
	}
	seated, err := h.eraser.InActiveRoom(userID)
	if err != nil {
		return err
	}
	if seated {
		return apperr.ErrAccountDeleteInRoom
	}
	if !deletionConfirmed(u, req) {
		return apperr.ErrAccountDeleteConfirmation
	}

	h.revoked.Revoke(userID)
	erased, err := h.eraser.Erase(userID)
	if err != nil {
		h.revoked.Unrevoke(userID)
		if errors.Is(err, apperr.ErrAccountDeleteInRoom) || errors.Is(err, apperr.ErrUserNotFound) {
			return err
		}
		return fmt.Errorf("erasing account: %w", err)
	}

	h.sessions.DisconnectUser(userID)
	avatarPrefix := ""
	if erased.AvatarKey != nil {
		avatarPrefix = *erased.AvatarKey
	}
	if avatarPrefix != "" && h.avatars != nil {
		h.avatars.DeleteStored(c.Request().Context(), userID, avatarPrefix, "account deleted")
	}
	clearRefreshCookie(c, h.env)
	slog.Info("account erased", "userID", userID, "avatarPrefix", avatarPrefix)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{"message": "account deleted"},
	})
}

// deletionConfirmed checks the password of an account that has one, or the exact username of a passwordless one.
func deletionConfirmed(u *user.User, req DeleteAccountRequest) bool {
	if u.PasswordHash != "" {
		return req.Password != "" && CheckPassword(u.PasswordHash, req.Password) == nil
	}
	typed := strings.TrimSpace(req.Username)
	return typed != "" && typed == u.Username
}
