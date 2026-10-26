package auth

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/text/unicode/norm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/identity"
	"github.com/emilijan/beljot/server/internal/refreshtoken"
	"github.com/emilijan/beljot/server/internal/user"
)

const reuseGracePeriod = 20 * time.Second

type RegisterRequest struct {
	Email              string `json:"email"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	LanguagePreference string `json:"languagePreference"`
}

// RegisterResponseData is the auth envelope shared by Register, Login, and Refresh.
// Wallet and honor fields are read-only echoes; no auth handler grants the daily bonus.
type RegisterResponseData struct {
	ID                    uint      `json:"id"`
	Username              string    `json:"username"`
	Email                 string    `json:"email"`
	LanguagePreference    string    `json:"languagePreference"`
	CardDeckPreference    string    `json:"cardDeckPreference"`
	SoundEnabled          bool      `json:"soundEnabled"`
	MusicEnabled          bool      `json:"musicEnabled"`
	SoundVolume           int       `json:"soundVolume"`
	MusicVolume           int       `json:"musicVolume"`
	AllowNonFriendInvites bool      `json:"allowNonFriendInvites"`
	WalletBalance         int       `json:"walletBalance"`
	LoginStreakDays       int       `json:"loginStreakDays"`
	TotalXP               int       `json:"totalXp"`
	Level                 int       `json:"level"`
	HonorScore            int       `json:"honorScore"`
	HonorTier             string    `json:"honorTier"`
	IsNewPlayer           bool      `json:"isNewPlayer"`
	AvatarURL             *string   `json:"avatarUrl"`
	CreatedAt             time.Time `json:"createdAt"`
	Token                 string    `json:"token"`
}

type AuthHandler struct {
	userRepo     user.UserRepository
	refreshRepo  refreshtoken.Repository
	identityRepo identity.Repository
	providers    identity.Registry
	jwtSecret    string
	env          string
	accessTTL    time.Duration
	idleTTL      time.Duration
	absoluteTTL  time.Duration
}

func NewAuthHandler(
	userRepo user.UserRepository,
	refreshRepo refreshtoken.Repository,
	identityRepo identity.Repository,
	providers identity.Registry,
	jwtSecret string,
	env string,
	accessTTL time.Duration,
	idleTTL time.Duration,
	absoluteTTL time.Duration,
) *AuthHandler {
	return &AuthHandler{
		userRepo:     userRepo,
		refreshRepo:  refreshRepo,
		identityRepo: identityRepo,
		providers:    providers,
		jwtSecret:    jwtSecret,
		env:          env,
		accessTTL:    accessTTL,
		idleTTL:      idleTTL,
		absoluteTTL:  absoluteTTL,
	}
}

func (h *AuthHandler) setRefreshCookie(c echo.Context, token string, maxAge int) {
	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.env != "development",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
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

// startSession opens a brand-new refresh-token family for a login/registration:
// it mints the first opaque token, persists its hash with a sliding idle
// deadline and a fixed absolute cap, and sets the httpOnly cookie.
func (h *AuthHandler) startSession(c echo.Context, userID uint) error {
	familyID, err := newFamilyID()
	if err != nil {
		return fmt.Errorf("generating family id: %w", err)
	}
	raw, hash, err := mintRefreshToken()
	if err != nil {
		return fmt.Errorf("minting refresh token: %w", err)
	}
	now := time.Now()
	familyExpiresAt := now.Add(h.absoluteTTL)
	exp := now.Add(h.idleTTL)
	if exp.After(familyExpiresAt) {
		exp = familyExpiresAt
	}
	rt := &refreshtoken.RefreshToken{
		UserID:          userID,
		FamilyID:        familyID,
		TokenHash:       hash,
		ExpiresAt:       exp,
		FamilyExpiresAt: familyExpiresAt,
	}
	if err := h.refreshRepo.Create(rt); err != nil {
		return fmt.Errorf("persisting refresh token: %w", err)
	}
	h.setRefreshCookie(c, raw, refreshCookieMaxAge(exp))
	return nil
}

// revokeFamily terminates a whole session, logging (but not surfacing) failures
// so a revoke error never masks the security decision the caller already made.
func (h *AuthHandler) revokeFamily(familyID, reason string) {
	if err := h.refreshRepo.RevokeFamily(familyID); err != nil {
		slog.Error("failed to revoke refresh token family", "error", err, "familyId", familyID, "reason", reason)
	}
}

func authResponseData(u *user.User, accessToken string) RegisterResponseData {
	honor := user.NewHonorSnapshot(
		u.HonorCompletedWeight, u.HonorAbandonedWeight, u.HonorDecayedAt,
		u.HonorCompletedTotal, u.HonorAbandonedTotal, time.Now().UTC(),
	)
	return RegisterResponseData{
		ID:                    u.ID,
		Username:              u.Username,
		Email:                 u.Email,
		LanguagePreference:    u.LanguagePreference,
		CardDeckPreference:    u.CardDeckPreference,
		SoundEnabled:          u.SoundEnabled,
		MusicEnabled:          u.MusicEnabled,
		SoundVolume:           u.SoundVolume,
		MusicVolume:           u.MusicVolume,
		AllowNonFriendInvites: u.AllowNonFriendInvites,
		WalletBalance:         u.WalletBalance,
		LoginStreakDays:       u.LoginStreakDays,
		TotalXP:               u.TotalXP,
		Level:                 user.LevelForXP(u.TotalXP),
		HonorScore:            honor.Score,
		HonorTier:             honor.Tier,
		IsNewPlayer:           honor.IsNewPlayer,
		AvatarURL:             user.SmallAvatarURL(u.AvatarKey),
		CreatedAt:             u.CreatedAt,
		Token:                 accessToken,
	}
}

// Register creates the account and starts a session. The wallet is seeded at 5000 with
// last login stamped to today (UTC), so the first daily bonus lands on the next day.
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

	lang := "en"
	if user.IsSupportedLanguage(req.LanguagePreference) {
		lang = req.LanguagePreference
	}

	deck := user.CardDeckFrench
	if lang == "hr" {
		deck = user.CardDeckCroatian
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	u := &user.User{
		Email:                 req.Email,
		Username:              req.Username,
		PasswordHash:          hash,
		LanguagePreference:    lang,
		CardDeckPreference:    deck,
		AllowNonFriendInvites: true,
		WalletBalance:         5000,
		LoginStreakDays:       0,
		LastLoginAt:           &today,
	}

	if err := h.userRepo.Create(u); err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	accessToken, err := GenerateAccessTokenWithTTL(u.ID, h.jwtSecret, h.accessTTL)
	if err != nil {
		return fmt.Errorf("generating access token: %w", err)
	}

	if err := h.startSession(c, u.ID); err != nil {
		return fmt.Errorf("starting session: %w", err)
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"data": authResponseData(u, accessToken),
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

	accessToken, err := GenerateAccessTokenWithTTL(u.ID, h.jwtSecret, h.accessTTL)
	if err != nil {
		return fmt.Errorf("generating access token: %w", err)
	}

	if err := h.startSession(c, u.ID); err != nil {
		return fmt.Errorf("starting session: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": authResponseData(u, accessToken),
	})
}

// Refresh rotates a live refresh token atomically to a successor and detects replay.
// A consumed token within the grace window gets an access token only; past it, the family is revoked.
func (h *AuthHandler) Refresh(c echo.Context) error {
	cookie, err := c.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}
	hash := hashRefreshToken(cookie.Value)

	rt, err := h.refreshRepo.FindByHash(hash)
	if err != nil {
		return fmt.Errorf("looking up refresh token: %w", err)
	}
	if rt == nil || rt.RevokedAt != nil {
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	now := time.Now()
	if now.After(rt.FamilyExpiresAt) {
		h.revokeFamily(rt.FamilyID, "absolute-cap")
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	u, err := h.userRepo.FindByID(rt.UserID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		h.revokeFamily(rt.FamilyID, "user-gone")
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	if rt.RotatedAt != nil {
		return h.handleConsumed(c, rt, u, now)
	}

	if now.After(rt.ExpiresAt) {
		h.revokeFamily(rt.FamilyID, "idle-expired")
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	raw, successor, err := h.newSuccessor(rt.UserID, rt.FamilyID, rt.FamilyExpiresAt)
	if err != nil {
		return err
	}
	won, err := h.refreshRepo.RotateAndReplace(rt.ID, successor)
	if err != nil {
		return fmt.Errorf("rotating refresh token: %w", err)
	}
	if won {
		h.setRefreshCookie(c, raw, refreshCookieMaxAge(successor.ExpiresAt))
		return h.respondAccess(c, u)
	}

	fresh, err := h.refreshRepo.FindByHash(hash)
	if err != nil {
		return fmt.Errorf("re-checking refresh token: %w", err)
	}
	if fresh == nil || fresh.RevokedAt != nil {
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}
	return h.handleConsumed(c, fresh, u, time.Now())
}

// handleConsumed issues an access token without rotating or touching the cookie when
// an already-rotated token arrives within the grace window, and revokes the family past it.
func (h *AuthHandler) handleConsumed(c echo.Context, rt *refreshtoken.RefreshToken, u *user.User, now time.Time) error {
	if rt.RotatedAt != nil && now.Sub(*rt.RotatedAt) > reuseGracePeriod {
		h.revokeFamily(rt.FamilyID, "reuse-detected")
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}
	return h.respondAccess(c, u)
}

// newSuccessor builds (but does not persist) the next token in a family: a fresh
// opaque token whose idle window slides to now+idleTTL, capped at the family's
// fixed absolute deadline.
func (h *AuthHandler) newSuccessor(userID uint, familyID string, familyExpiresAt time.Time) (raw string, rt *refreshtoken.RefreshToken, err error) {
	raw, hash, err := mintRefreshToken()
	if err != nil {
		return "", nil, fmt.Errorf("minting refresh token: %w", err)
	}
	exp := time.Now().Add(h.idleTTL)
	if exp.After(familyExpiresAt) {
		exp = familyExpiresAt
	}
	return raw, &refreshtoken.RefreshToken{
		UserID:          userID,
		FamilyID:        familyID,
		TokenHash:       hash,
		ExpiresAt:       exp,
		FamilyExpiresAt: familyExpiresAt,
	}, nil
}

// respondAccess returns the standard auth envelope with a fresh access token.
// It never touches the refresh cookie.
func (h *AuthHandler) respondAccess(c echo.Context, u *user.User) error {
	accessToken, err := GenerateAccessTokenWithTTL(u.ID, h.jwtSecret, h.accessTTL)
	if err != nil {
		return fmt.Errorf("generating access token: %w", err)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": authResponseData(u, accessToken),
	})
}

// refreshCookieMaxAge is the cookie Max-Age in seconds for a token expiring at exp.
// It is clamped to at least 1 so it never means a session cookie (0) or a delete (<0).
func refreshCookieMaxAge(exp time.Time) int {
	secs := int(time.Until(exp).Seconds())
	if secs < 1 {
		secs = 1
	}
	return secs
}

func (h *AuthHandler) Logout(c echo.Context) error {
	if cookie, err := c.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		rt, err := h.refreshRepo.FindByHash(hashRefreshToken(cookie.Value))
		if err != nil {
			slog.Error("failed to look up refresh token on logout", "error", err)
		} else if rt != nil {
			h.revokeFamily(rt.FamilyID, "logout")
		}
	}

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

	username, err := user.ValidateUsername(req.Username)
	if err != nil {
		return err
	}
	req.Username = username

	if len(req.Password) < 8 {
		return apperr.ErrPasswordTooShort
	}
	if len(req.Password) > 72 {
		return apperr.ErrPasswordTooLong
	}

	return nil
}
