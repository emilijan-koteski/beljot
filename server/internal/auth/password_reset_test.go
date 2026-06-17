package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/passwordreset"
	"github.com/emilijan/beljot/server/internal/user"
)

// --- test doubles ---------------------------------------------------------

type fakeResetRepo struct {
	tokens    []*passwordreset.PasswordResetToken
	nextID    uint
	createErr error
}

func (r *fakeResetRepo) Create(t *passwordreset.PasswordResetToken) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.nextID++
	t.ID = r.nextID
	r.tokens = append(r.tokens, t)
	return nil
}

func (r *fakeResetRepo) FindValidByHash(hash string) (*passwordreset.PasswordResetToken, error) {
	for _, t := range r.tokens {
		if t.TokenHash == hash && t.UsedAt == nil && t.ExpiresAt.After(time.Now()) {
			return t, nil
		}
	}
	return nil, nil
}

func (r *fakeResetRepo) MarkUsed(id uint) (bool, error) {
	for _, t := range r.tokens {
		if t.ID == id {
			if t.UsedAt != nil {
				return false, nil
			}
			now := time.Now()
			t.UsedAt = &now
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeResetRepo) DeleteByUserID(userID uint) error {
	var kept []*passwordreset.PasswordResetToken
	for _, t := range r.tokens {
		if t.UserID != userID {
			kept = append(kept, t)
		}
	}
	r.tokens = kept
	return nil
}

type sentEmail struct {
	to, lang, link string
}

type fakeMailer struct {
	mu    sync.Mutex
	calls []sentEmail
	err   error
}

// SendPasswordReset is invoked from a background goroutine by the handler, so
// access to calls is mutex-guarded.
func (m *fakeMailer) SendPasswordReset(_ context.Context, to, lang, link string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.calls = append(m.calls, sentEmail{to: to, lang: lang, link: link})
	return nil
}

// sent returns a snapshot copy of the recorded sends, safe to read from a test
// goroutine while the async send may still be in flight.
func (m *fakeMailer) sent() []sentEmail {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]sentEmail(nil), m.calls...)
}

// --- harness --------------------------------------------------------------

func setupResetHandler() (*mockUserRepo, *fakeResetRepo, *fakeMailer, *echo.Echo) {
	userRepo := newMockUserRepo()
	resetRepo := &fakeResetRepo{}
	fm := &fakeMailer{}
	h := NewPasswordResetHandler(userRepo, resetRepo, fm, "http://localhost:5173", time.Hour)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	e.POST("/api/v1/auth/forgot-password", h.ForgotPassword)
	e.POST("/api/v1/auth/reset-password", h.ResetPassword)
	return userRepo, resetRepo, fm, e
}

func seedResetUser(t *testing.T, repo *mockUserRepo, email, lang string) *user.User {
	t.Helper()
	hash, err := HashPassword("oldpassword123")
	require.NoError(t, err)
	u := &user.User{Email: email, Username: "tester", PasswordHash: hash, LanguagePreference: lang}
	require.NoError(t, repo.Create(u))
	return u
}

func postJSON(e *echo.Echo, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	return errResp["error"]["code"]
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	require.NoError(t, err)
	return u.Query().Get("token")
}

// --- ForgotPassword -------------------------------------------------------

func TestForgotPassword_KnownEmail_SendsLinkInUserLanguage(t *testing.T) {
	userRepo, resetRepo, fm, e := setupResetHandler()
	seedResetUser(t, userRepo, "test@example.com", "mk")

	rec := postJSON(e, "/api/v1/auth/forgot-password", `{"email":"test@example.com"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	// The email is dispatched on a background goroutine — wait for it.
	require.Eventually(t, func() bool { return len(fm.sent()) == 1 }, time.Second, 5*time.Millisecond,
		"expected one email for a known address")
	sent := fm.sent()[0]
	assert.Equal(t, "test@example.com", sent.to)
	assert.Equal(t, "mk", sent.lang, "email language must follow the user's stored preference")
	assert.Contains(t, sent.link, "http://localhost:5173/reset-password?token=")
	require.Len(t, resetRepo.tokens, 1, "a token row must be persisted")
	assert.NotEmpty(t, resetRepo.tokens[0].TokenHash)
}

func TestForgotPassword_UnknownEmail_GenericSuccessNoEmail(t *testing.T) {
	_, resetRepo, fm, e := setupResetHandler()

	rec := postJSON(e, "/api/v1/auth/forgot-password", `{"email":"nobody@example.com"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, fm.sent(), "no email may be sent for an unknown address")
	assert.Empty(t, resetRepo.tokens, "no token may be created for an unknown address")
}

func TestForgotPassword_MalformedEmail_GenericSuccessNoToken(t *testing.T) {
	_, resetRepo, fm, e := setupResetHandler()

	rec := postJSON(e, "/api/v1/auth/forgot-password", `{"email":"   "}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, fm.sent())
	assert.Empty(t, resetRepo.tokens)
}

func TestForgotPassword_ResponseIdenticalKnownVsUnknown(t *testing.T) {
	userRepo, _, _, e := setupResetHandler()
	seedResetUser(t, userRepo, "test@example.com", "en")

	known := postJSON(e, "/api/v1/auth/forgot-password", `{"email":"test@example.com"}`)
	unknown := postJSON(e, "/api/v1/auth/forgot-password", `{"email":"nobody@example.com"}`)

	assert.Equal(t, known.Code, unknown.Code, "status must not reveal account existence")
	assert.JSONEq(t, known.Body.String(), unknown.Body.String(), "body must not reveal account existence")
}

func TestForgotPassword_ReplacesPriorTokens(t *testing.T) {
	userRepo, resetRepo, _, e := setupResetHandler()
	seedResetUser(t, userRepo, "test@example.com", "en")

	postJSON(e, "/api/v1/auth/forgot-password", `{"email":"test@example.com"}`)
	postJSON(e, "/api/v1/auth/forgot-password", `{"email":"test@example.com"}`)

	assert.Len(t, resetRepo.tokens, 1, "only the most recent reset token should be live")
}

func TestForgotPassword_MailerError_StillGenericSuccess(t *testing.T) {
	userRepo, _, fm, e := setupResetHandler()
	seedResetUser(t, userRepo, "test@example.com", "en")
	fm.err = errors.New("smtp down")

	rec := postJSON(e, "/api/v1/auth/forgot-password", `{"email":"test@example.com"}`)

	assert.Equal(t, http.StatusOK, rec.Code, "mailer failures must not leak via the response")
}

// --- ResetPassword --------------------------------------------------------

func TestResetPassword_ValidToken_UpdatesHashAndIsSingleUse(t *testing.T) {
	userRepo, _, fm, e := setupResetHandler()
	u := seedResetUser(t, userRepo, "test@example.com", "en")

	postJSON(e, "/api/v1/auth/forgot-password", `{"email":"test@example.com"}`)
	require.Eventually(t, func() bool { return len(fm.sent()) == 1 }, time.Second, 5*time.Millisecond)
	token := tokenFromLink(t, fm.sent()[0].link)
	require.NotEmpty(t, token)

	rec := postJSON(e, "/api/v1/auth/reset-password", `{"token":"`+token+`","password":"brandnewpass1"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	stored, _ := userRepo.FindByID(u.ID)
	require.NotNil(t, stored)
	assert.NoError(t, CheckPassword(stored.PasswordHash, "brandnewpass1"), "new password must verify")
	assert.Error(t, CheckPassword(stored.PasswordHash, "oldpassword123"), "old password must no longer verify")

	// Re-using the same link must fail — single use.
	reuse := postJSON(e, "/api/v1/auth/reset-password", `{"token":"`+token+`","password":"anotherpass99"}`)
	assert.Equal(t, http.StatusBadRequest, reuse.Code)
	assert.Equal(t, "INVALID_RESET_TOKEN", errCode(t, reuse))
}

func TestResetPassword_ExpiredToken_Invalid(t *testing.T) {
	userRepo, resetRepo, _, e := setupResetHandler()
	u := seedResetUser(t, userRepo, "test@example.com", "en")

	raw := "expired-raw-token"
	expired := time.Now().Add(-1 * time.Hour)
	resetRepo.tokens = append(resetRepo.tokens, &passwordreset.PasswordResetToken{
		ID: 99, UserID: u.ID, TokenHash: hashResetToken(raw), ExpiresAt: expired,
	})

	rec := postJSON(e, "/api/v1/auth/reset-password", `{"token":"`+raw+`","password":"brandnewpass1"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "INVALID_RESET_TOKEN", errCode(t, rec))
}

func TestResetPassword_InvalidToken(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unknown token", `{"token":"does-not-exist","password":"brandnewpass1"}`},
		{"empty token", `{"token":"","password":"brandnewpass1"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, e := setupResetHandler()
			rec := postJSON(e, "/api/v1/auth/reset-password", tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "INVALID_RESET_TOKEN", errCode(t, rec))
		})
	}
}

// Password validation is reached only AFTER the token is found valid (token
// validity wins over password shape), so these cases seed a live token first
// and assert the token is left unconsumed when the password is rejected.
func TestResetPassword_PasswordValidation(t *testing.T) {
	const raw = "valid-raw-token-for-pw-checks"
	tests := []struct {
		name     string
		password string
		wantCode string
	}{
		{"too short", "short", "PASSWORD_TOO_SHORT"},
		{"too long", strings.Repeat("x", 73), "PASSWORD_TOO_LONG"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userRepo, resetRepo, _, e := setupResetHandler()
			u := seedResetUser(t, userRepo, "test@example.com", "en")
			resetRepo.tokens = append(resetRepo.tokens, &passwordreset.PasswordResetToken{
				ID: 1, UserID: u.ID, TokenHash: hashResetToken(raw), ExpiresAt: time.Now().Add(time.Hour),
			})

			rec := postJSON(e, "/api/v1/auth/reset-password",
				`{"token":"`+raw+`","password":"`+tc.password+`"}`)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, tc.wantCode, errCode(t, rec))
			assert.Nil(t, resetRepo.tokens[0].UsedAt, "a rejected password must leave the token usable")
		})
	}
}
