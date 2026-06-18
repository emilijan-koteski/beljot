package auth

import (
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/text/unicode/norm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/user"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type RegisterRequest struct {
	Email              string `json:"email"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	LanguagePreference string `json:"languagePreference"`
}

// RegisterResponseData is shared by Register, Login, and Refresh. The wallet
// fields are read-only echoes for immediate header display — NO auth handler
// grants the daily bonus (that is the wallet endpoint's job). Register sets
// balance 5000 / streak 0; Login and Refresh echo the loaded user's values.
type RegisterResponseData struct {
	ID                 uint      `json:"id"`
	Username           string    `json:"username"`
	Email              string    `json:"email"`
	LanguagePreference string    `json:"languagePreference"`
	WalletBalance      int       `json:"walletBalance"`
	LoginStreakDays    int       `json:"loginStreakDays"`
	CreatedAt          time.Time `json:"createdAt"`
	Token              string    `json:"token"`
}

type AuthHandler struct {
	userRepo  user.UserRepository
	jwtSecret string
	env       string
}

func NewAuthHandler(userRepo user.UserRepository, jwtSecret string, env string) *AuthHandler {
	return &AuthHandler{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
		env:       env,
	}
}

func (h *AuthHandler) setRefreshCookie(c echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.env != "development",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 60 * 60,
	})
}

func (h *AuthHandler) clearRefreshCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.env != "development",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}

	if err := validateRegisterRequest(&req); err != nil {
		return err
	}

	existing, err := h.userRepo.FindByEmail(req.Email)
	if err != nil {
		return fmt.Errorf("checking email: %w", err)
	}
	if existing != nil {
		return apperr.ErrEmailTaken
	}

	existing, err = h.userRepo.FindByUsername(req.Username)
	if err != nil {
		return fmt.Errorf("checking username: %w", err)
	}
	if existing != nil {
		return apperr.ErrUsernameTaken
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	// languagePreference is a UX-tolerant field — bad/absent values fall
	// back to "en" rather than failing the registration. The /preferences
	// endpoint is the strict validator and exists for later correction.
	lang := "en"
	if user.IsSupportedLanguage(req.LanguagePreference) {
		lang = req.LanguagePreference
	}

	// Seed the wallet at registration (Story 9.1): balance 5000, streak 0, and
	// last_login stamped to today (UTC). Stamping today is what makes the
	// same-day bootstrap a no-grant and the FIRST grant land on the next
	// calendar day (day-1). No bonus is granted here — the grant is the wallet
	// endpoint's job. 5000 mirrors wallet.StartingBalance and migration 000009;
	// set explicitly so the response and the inserted row agree.
	//
	// Stamp the UTC calendar date (midnight), not the current instant: the
	// last_login_at column is a DATE and the wallet daily-login path writes a
	// date-truncated value too, so both write paths agree in intent (rather than
	// relying on the DATE column to silently truncate a full timestamp).
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	u := &user.User{
		Email:              req.Email,
		Username:           req.Username,
		PasswordHash:       hash,
		LanguagePreference: lang,
		WalletBalance:      5000,
		LoginStreakDays:    0,
		LastLoginAt:        &today,
	}

	if err := h.userRepo.Create(u); err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	accessToken, err := GenerateAccessToken(u.ID, h.jwtSecret)
	if err != nil {
		return fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := GenerateRefreshToken(u.ID, h.jwtSecret)
	if err != nil {
		return fmt.Errorf("generating refresh token: %w", err)
	}

	h.setRefreshCookie(c, refreshToken)

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"data": RegisterResponseData{
			ID:                 u.ID,
			Username:           u.Username,
			Email:              u.Email,
			LanguagePreference: u.LanguagePreference,
			WalletBalance:      u.WalletBalance,
			LoginStreakDays:    u.LoginStreakDays,
			CreatedAt:          u.CreatedAt,
			Token:              accessToken,
		},
	})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return apperr.ErrBadRequest
	}

	email := strings.ToLower(norm.NFC.String(strings.TrimSpace(req.Email)))
	if email == "" || req.Password == "" {
		return apperr.ErrInvalidCredentials
	}
	addr, err := mail.ParseAddress(email)
	if err == nil {
		email = strings.ToLower(norm.NFC.String(addr.Address))
	}

	u, err := h.userRepo.FindByEmail(email)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		return apperr.ErrInvalidCredentials
	}

	if err := CheckPassword(u.PasswordHash, req.Password); err != nil {
		return apperr.ErrInvalidCredentials
	}

	accessToken, err := GenerateAccessToken(u.ID, h.jwtSecret)
	if err != nil {
		return fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := GenerateRefreshToken(u.ID, h.jwtSecret)
	if err != nil {
		return fmt.Errorf("generating refresh token: %w", err)
	}

	h.setRefreshCookie(c, refreshToken)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": RegisterResponseData{
			ID:                 u.ID,
			Username:           u.Username,
			Email:              u.Email,
			LanguagePreference: u.LanguagePreference,
			WalletBalance:      u.WalletBalance,
			LoginStreakDays:    u.LoginStreakDays,
			CreatedAt:          u.CreatedAt,
			Token:              accessToken,
		},
	})
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	cookie, err := c.Cookie("refresh_token")
	if err != nil {
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	claims, err := ValidateToken(cookie.Value, h.jwtSecret)
	if err != nil {
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	if !slices.Contains([]string(claims.Audience), "refresh") {
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil {
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	u, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	accessToken, err := GenerateAccessToken(u.ID, h.jwtSecret)
	if err != nil {
		return fmt.Errorf("generating access token: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": RegisterResponseData{
			ID:                 u.ID,
			Username:           u.Username,
			Email:              u.Email,
			LanguagePreference: u.LanguagePreference,
			WalletBalance:      u.WalletBalance,
			LoginStreakDays:    u.LoginStreakDays,
			CreatedAt:          u.CreatedAt,
			Token:              accessToken,
		},
	})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	h.clearRefreshCookie(c)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{
			"message": "logged out",
		},
	})
}

func validateRegisterRequest(req *RegisterRequest) error {
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		return apperr.ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(req.Email)
	if err != nil {
		return apperr.ErrInvalidEmail
	}
	req.Email = strings.ToLower(norm.NFC.String(addr.Address))

	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 {
		return apperr.ErrUsernameTooShort
	}
	if len(req.Username) > 20 {
		return apperr.ErrUsernameTooLong
	}
	if !usernameRegex.MatchString(req.Username) {
		return apperr.ErrUsernameInvalidChars
	}

	if len(req.Password) < 8 {
		return apperr.ErrPasswordTooShort
	}
	if len(req.Password) > 72 {
		return apperr.ErrPasswordTooLong
	}

	return nil
}
