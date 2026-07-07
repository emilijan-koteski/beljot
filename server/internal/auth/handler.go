package auth

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/text/unicode/norm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/identity"
	"github.com/emilijan/beljot/server/internal/refreshtoken"
	"github.com/emilijan/beljot/server/internal/user"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// reuseGracePeriod is how long after a token is rotated an already-rotated
// token may still be presented without being treated as a stolen-token replay.
// It absorbs benign races (a second browser tab, or a retried request that
// never saw the rotated cookie); a replay after this window revokes the family.
const reuseGracePeriod = 20 * time.Second

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
	ID                 uint   `json:"id"`
	Username           string `json:"username"`
	Email              string `json:"email"`
	LanguagePreference string `json:"languagePreference"`
	WalletBalance      int    `json:"walletBalance"`
	LoginStreakDays    int    `json:"loginStreakDays"`
	// XP & level (Story 9.5) — read-only echoes so the top-nav banner has the
	// level + XP immediately on auth, without a separate profile fetch. TotalXP
	// is the loaded lifetime total (0 for a fresh registration); Level is derived
	// from it via user.LevelForXP (never stored).
	TotalXP   int       `json:"totalXp"`
	Level     int       `json:"level"`
	CreatedAt time.Time `json:"createdAt"`
	Token     string    `json:"token"`
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
	// Idle deadline never exceeds the absolute cap (matches newSuccessor; only
	// differs under a misconfigured idleTTL > absoluteTTL, which config warns on).
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
	return RegisterResponseData{
		ID:                 u.ID,
		Username:           u.Username,
		Email:              u.Email,
		LanguagePreference: u.LanguagePreference,
		WalletBalance:      u.WalletBalance,
		LoginStreakDays:    u.LoginStreakDays,
		TotalXP:            u.TotalXP,
		Level:              user.LevelForXP(u.TotalXP),
		CreatedAt:          u.CreatedAt,
		Token:              accessToken,
	}
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

// Refresh rotates the refresh token and detects replay. The cookie value is an
// opaque token looked up by hash. See the spec's Design Notes for the full
// state table; in short: a live token rotates (atomically) to a successor; a
// consumed token within the grace window is a benign race and gets an
// access-token only (no re-rotation, no cookie write) so the winner's live
// token stays the cookie; a consumed token past grace is a replay → revoke the
// family.
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
	// Unknown token (never issued, or a stale pre-rotation JWT cookie from before
	// this feature) or an already-revoked family → reject and clear.
	if rt == nil || rt.RevokedAt != nil {
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	now := time.Now()
	// Absolute cap applies to every token in the family (copied unchanged onto
	// each successor), so it is safe to check on whatever token was presented.
	if now.After(rt.FamilyExpiresAt) {
		h.revokeFamily(rt.FamilyID, "absolute-cap")
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	// Load the user before any rotation: if the account was deleted after the
	// token was issued, revoke the family and stop — never mint for a gone user.
	u, err := h.userRepo.FindByID(rt.UserID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	if u == nil {
		h.revokeFamily(rt.FamilyID, "user-gone")
		h.clearRefreshCookie(c)
		return apperr.ErrUnauthorized
	}

	// A consumed token was presented — benign race or replay.
	if rt.RotatedAt != nil {
		return h.handleConsumed(c, rt, u, now)
	}

	// Live token. Idle expiry is a property of the LIVE token (the family's last
	// activity), not of a spent sibling.
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

	// Lost the rotation race: another request consumed this exact token (or the
	// family was revoked) between our read and the update. Re-read for fresh
	// state and reject a revoked family; otherwise treat as the now-consumed
	// benign race.
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

// handleConsumed serves an already-rotated token. Within the grace window it is
// a benign race (a second tab, a retried request, or bootstrap): issue a fresh
// access token WITHOUT rotating and WITHOUT touching the cookie — the request
// that won the rotation already set the live successor as this browser's cookie,
// and re-rotating here would consume it. Past the window it is a replay of a
// consumed token — the compromise signal — so revoke the whole family.
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

// refreshCookieMaxAge is the cookie Max-Age (seconds) for a token expiring at
// exp. Clamped to >= 1 so it is always a positive lifetime (never Go's MaxAge==0
// "session cookie" or <0 "delete" semantics); callers only reach here with a
// future exp.
func refreshCookieMaxAge(exp time.Time) int {
	secs := int(time.Until(exp).Seconds())
	if secs < 1 {
		secs = 1
	}
	return secs
}

func (h *AuthHandler) Logout(c echo.Context) error {
	// Revoke the presented session's family so the refresh token can't be
	// reused after logout. Best-effort: logout always succeeds for the client.
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
