package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/text/unicode/norm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/mailer"
	"github.com/emilijan/beljot/server/internal/passwordreset"
	"github.com/emilijan/beljot/server/internal/user"
)

// PasswordResetHandler owns the public forgot-password / reset-password
// endpoints. It is intentionally separate from AuthHandler so wiring the
// mailer + reset-token repo doesn't disturb the existing auth constructor or
// its tests.
type PasswordResetHandler struct {
	userRepo   user.UserRepository
	resetRepo  passwordreset.Repository
	mailer     mailer.Mailer
	appBaseURL string
	resetTTL   time.Duration
}

func NewPasswordResetHandler(
	userRepo user.UserRepository,
	resetRepo passwordreset.Repository,
	m mailer.Mailer,
	appBaseURL string,
	resetTTL time.Duration,
) *PasswordResetHandler {
	return &PasswordResetHandler{
		userRepo:   userRepo,
		resetRepo:  resetRepo,
		mailer:     m,
		appBaseURL: appBaseURL,
		resetTTL:   resetTTL,
	}
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// genericForgotResponse is returned for every forgot-password call — existing
// email, unknown email, or malformed email alike — so the response never
// reveals whether an account exists.
func genericForgotResponse(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{
			"message": "if an account exists for that email, a password reset link has been sent",
		},
	})
}

func (h *PasswordResetHandler) ForgotPassword(c echo.Context) error {
	var req ForgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}

	email := normalizeEmail(req.Email)
	if email == "" {
		return genericForgotResponse(c)
	}

	u, err := h.userRepo.FindByEmail(email)
	if err != nil {
		return fmt.Errorf("finding user for password reset: %w", err)
	}
	// Anti-enumeration: identical response whether or not the user exists. Only
	// do the token+email work when a user is actually found.
	if u != nil {
		if err := h.issueAndSend(u); err != nil {
			// Token/DB errors are logged but never surfaced — the caller still
			// gets the generic success so existence cannot be probed.
			slog.Error("failed to issue password reset", "error", err, "userId", u.ID)
		}
	}

	return genericForgotResponse(c)
}

// issueAndSend invalidates any prior tokens, mints a fresh single-use token,
// persists its hash, and emails the reset link in the user's language. The raw
// token is only ever placed in the email link.
//
// The email is sent on a background goroutine so the HTTP response returns
// without waiting on SMTP — this keeps the forgot-password latency independent
// of whether the account exists (no timing-based enumeration) and never blocks
// the user on a slow mail server.
func (h *PasswordResetHandler) issueAndSend(u *user.User) error {
	if err := h.resetRepo.DeleteByUserID(u.ID); err != nil {
		return fmt.Errorf("clearing prior reset tokens: %w", err)
	}

	raw, hash, err := generateResetToken()
	if err != nil {
		return fmt.Errorf("generating reset token: %w", err)
	}

	token := &passwordreset.PasswordResetToken{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(h.resetTTL),
	}
	if err := h.resetRepo.Create(token); err != nil {
		return fmt.Errorf("persisting reset token: %w", err)
	}

	link := fmt.Sprintf("%s/reset-password?token=%s", h.appBaseURL, raw)
	email, lang := u.Email, u.LanguagePreference
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.mailer.SendPasswordReset(ctx, email, lang, link); err != nil {
			slog.Error("failed to send password reset email", "error", err, "email", email)
		}
	}()
	return nil
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *PasswordResetHandler) ResetPassword(c echo.Context) error {
	var req ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}

	// Token validity wins over password shape: an invalid / expired / used link
	// always reports INVALID_RESET_TOKEN regardless of the submitted password.
	if strings.TrimSpace(req.Token) == "" {
		return apperr.ErrInvalidResetToken
	}
	token, err := h.resetRepo.FindValidByHash(hashResetToken(req.Token))
	if err != nil {
		return fmt.Errorf("looking up reset token: %w", err)
	}
	if token == nil {
		return apperr.ErrInvalidResetToken
	}

	if len(req.Password) < 8 {
		return apperr.ErrPasswordTooShort
	}
	if len(req.Password) > 72 {
		return apperr.ErrPasswordTooLong
	}

	// Consume the token atomically BEFORE changing the password — the single-use
	// guard against two concurrent requests that both passed FindValidByHash.
	// The loser of the race gets consumed == false.
	consumed, err := h.resetRepo.MarkUsed(token.ID)
	if err != nil {
		return fmt.Errorf("consuming reset token: %w", err)
	}
	if !consumed {
		return apperr.ErrInvalidResetToken
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return fmt.Errorf("hashing new password: %w", err)
	}

	if err := h.userRepo.UpdatePasswordHash(token.UserID, hash); err != nil {
		// The token's user no longer exists (e.g. soft-deleted after the link
		// was issued) — treat the link as invalid rather than leaking a 404
		// USER_NOT_FOUND the client can't map.
		if errors.Is(err, apperr.ErrUserNotFound) {
			return apperr.ErrInvalidResetToken
		}
		return fmt.Errorf("updating password: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{"message": "password updated"},
	})
}

// normalizeEmail lower-cases, trims, NFC-normalizes, and best-effort parses the
// address — mirrors the Login handler so lookups match stored emails.
func normalizeEmail(raw string) string {
	email := strings.ToLower(norm.NFC.String(strings.TrimSpace(raw)))
	if email == "" {
		return ""
	}
	if addr, err := mail.ParseAddress(email); err == nil {
		email = strings.ToLower(norm.NFC.String(addr.Address))
	}
	return email
}

// generateResetToken returns a URL-safe raw token (for the email link) and its
// SHA-256 hex hash (for storage). The raw token is never persisted.
func generateResetToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("reading random bytes: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashResetToken(raw), nil
}

func hashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
