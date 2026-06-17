package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/user"
)

func generateExpiredRefreshToken(userID uint, secret string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		Audience:  jwt.ClaimStrings{"refresh"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-8 * 24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

type mockUserRepo struct {
	users  []*user.User
	nextID uint
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{nextID: 1}
}

func (m *mockUserRepo) Create(u *user.User) error {
	u.ID = m.nextID
	u.CreatedAt = time.Now()
	m.nextID++
	m.users = append(m.users, u)
	return nil
}

func (m *mockUserRepo) FindByEmail(email string) (*user.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) FindByUsername(username string) (*user.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) FindByID(id uint) (*user.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) UpdateLanguagePreference(id uint, lang string) error {
	for _, u := range m.users {
		if u.ID == id {
			u.LanguagePreference = lang
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *mockUserRepo) UpdatePasswordHash(id uint, hash string) error {
	for _, u := range m.users {
		if u.ID == id {
			u.PasswordHash = hash
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *mockUserRepo) FindManyByIDs(ids []uint) ([]user.User, error) {
	if len(ids) == 0 {
		return []user.User{}, nil
	}
	wanted := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	out := make([]user.User, 0, len(ids))
	for _, u := range m.users {
		if _, ok := wanted[u.ID]; ok {
			out = append(out, *u)
		}
	}
	return out, nil
}

func (m *mockUserRepo) Count() (int64, error) {
	return int64(len(m.users)), nil
}

func testErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		_ = c.JSON(appErr.Status, map[string]interface{}{
			"error": map[string]string{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	_ = c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": "An internal error occurred",
		},
	})
}

func setupHandler() (*AuthHandler, *echo.Echo) {
	repo := newMockUserRepo()
	handler := NewAuthHandler(repo, "test-jwt-secret", "development")
	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	e.POST("/api/v1/auth/register", handler.Register)
	e.POST("/api/v1/auth/login", handler.Login)
	e.POST("/api/v1/auth/refresh", handler.Refresh)
	e.POST("/api/v1/auth/logout", handler.Logout)
	return handler, e
}

func doRegister(e *echo.Echo, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestRegister_Success(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"test@example.com","username":"testuser","password":"password123"}`
	rec := doRegister(e, body)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data RegisterResponseData
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, uint(1), data.ID)
	assert.Equal(t, "testuser", data.Username)
	assert.Equal(t, "test@example.com", data.Email)
	assert.Equal(t, "en", data.LanguagePreference)
	assert.NotEmpty(t, data.Token)
	assert.False(t, data.CreatedAt.IsZero(), "createdAt should be set")
}

func TestRegister_LanguagePreference(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "absent field defaults to en",
			body:     `{"email":"a@example.com","username":"absent","password":"password123"}`,
			expected: "en",
		},
		{
			name:     "empty string defaults to en",
			body:     `{"email":"b@example.com","username":"empty","password":"password123","languagePreference":""}`,
			expected: "en",
		},
		{
			name:     "unsupported code falls back to en (no 4xx)",
			body:     `{"email":"c@example.com","username":"badlang","password":"password123","languagePreference":"fr"}`,
			expected: "en",
		},
		{
			name:     "supported code mk is persisted",
			body:     `{"email":"d@example.com","username":"mkuser","password":"password123","languagePreference":"mk"}`,
			expected: "mk",
		},
		{
			name:     "supported code hr is persisted",
			body:     `{"email":"e@example.com","username":"hruser","password":"password123","languagePreference":"hr"}`,
			expected: "hr",
		},
		{
			name:     "supported code sr is persisted",
			body:     `{"email":"f@example.com","username":"sruser","password":"password123","languagePreference":"sr"}`,
			expected: "sr",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, e := setupHandler()
			rec := doRegister(e, tc.body)

			require.Equal(t, http.StatusCreated, rec.Code, "register should succeed; body: %s", rec.Body.String())

			var resp map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			var data RegisterResponseData
			require.NoError(t, json.Unmarshal(resp["data"], &data))
			assert.Equal(t, tc.expected, data.LanguagePreference)
		})
	}
}

func TestRegister_SetsRefreshCookie(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"test@example.com","username":"testuser","password":"password123"}`
	rec := doRegister(e, body)

	assert.Equal(t, http.StatusCreated, rec.Code)

	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}

	require.NotNil(t, refreshCookie, "refresh_token cookie should be set")
	assert.True(t, refreshCookie.HttpOnly)
	assert.False(t, refreshCookie.Secure, "Secure should be false in development environment")
	assert.Equal(t, http.SameSiteStrictMode, refreshCookie.SameSite)
	assert.Equal(t, "/api/v1/auth", refreshCookie.Path)
	assert.Equal(t, 7*24*60*60, refreshCookie.MaxAge)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"dup@example.com","username":"user1","password":"password123"}`
	rec := doRegister(e, body)
	assert.Equal(t, http.StatusCreated, rec.Code)

	body2 := `{"email":"dup@example.com","username":"user2","password":"password123"}`
	rec2 := doRegister(e, body2)
	assert.Equal(t, http.StatusConflict, rec2.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &errResp))
	assert.Equal(t, "EMAIL_TAKEN", errResp["error"]["code"])
}

func TestRegister_DuplicateUsername(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"user1@example.com","username":"sameuser","password":"password123"}`
	rec := doRegister(e, body)
	assert.Equal(t, http.StatusCreated, rec.Code)

	body2 := `{"email":"user2@example.com","username":"sameuser","password":"password123"}`
	rec2 := doRegister(e, body2)
	assert.Equal(t, http.StatusConflict, rec2.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &errResp))
	assert.Equal(t, "USERNAME_TAKEN", errResp["error"]["code"])
}

func TestRegister_InvalidEmail(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"not-an-email","username":"testuser","password":"password123"}`
	rec := doRegister(e, body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_EMAIL", errResp["error"]["code"])
}

func TestRegister_PasswordTooShort(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"test@example.com","username":"testuser","password":"short"}`
	rec := doRegister(e, body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "PASSWORD_TOO_SHORT", errResp["error"]["code"])
}

func TestRegister_PasswordTooLong(t *testing.T) {
	_, e := setupHandler()

	longPassword := strings.Repeat("a", 73)
	body := `{"email":"test@example.com","username":"testuser","password":"` + longPassword + `"}`
	rec := doRegister(e, body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "PASSWORD_TOO_LONG", errResp["error"]["code"])
}

func TestRegister_EmptyFields(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"","username":"","password":""}`
	rec := doRegister(e, body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegister_UsernameTooShort(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"test@example.com","username":"ab","password":"password123"}`
	rec := doRegister(e, body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "USERNAME_TOO_SHORT", errResp["error"]["code"])
}

func TestRegister_UsernameTooLong(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"test@example.com","username":"abcdefghijklmnopqrstu","password":"password123"}`
	rec := doRegister(e, body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "USERNAME_TOO_LONG", errResp["error"]["code"])
}

func TestRegister_UsernameInvalidChars(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"test@example.com","username":"bad user!","password":"password123"}`
	rec := doRegister(e, body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "USERNAME_INVALID_CHARS", errResp["error"]["code"])
}

func TestRegister_NormalizesEmail(t *testing.T) {
	_, e := setupHandler()

	body := `{"email":"  Test@Example.COM  ","username":"testuser","password":"password123"}`
	rec := doRegister(e, body)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data RegisterResponseData
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, "test@example.com", data.Email)
}

// --- Helper functions for login/refresh/logout tests ---

func doLogin(e *echo.Echo, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doRefresh(e *echo.Echo, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doLogout(e *echo.Echo) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func registerUser(e *echo.Echo) *httptest.ResponseRecorder {
	body := `{"email":"test@example.com","username":"testuser","password":"password123"}`
	return doRegister(e, body)
}

// --- Login tests ---

func TestLogin_Success(t *testing.T) {
	_, e := setupHandler()

	registerUser(e)

	rec := doLogin(e, `{"email":"test@example.com","password":"password123"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data RegisterResponseData
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, uint(1), data.ID)
	assert.Equal(t, "testuser", data.Username)
	assert.Equal(t, "test@example.com", data.Email)
	assert.NotEmpty(t, data.Token)

	// Check refresh cookie is set
	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie, "refresh_token cookie should be set on login")
}

func TestLogin_WrongPassword(t *testing.T) {
	_, e := setupHandler()

	registerUser(e)

	rec := doLogin(e, `{"email":"test@example.com","password":"wrongpassword"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_CREDENTIALS", errResp["error"]["code"])
}

func TestLogin_NonExistentEmail(t *testing.T) {
	_, e := setupHandler()

	rec := doLogin(e, `{"email":"noone@example.com","password":"password123"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var errResp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "INVALID_CREDENTIALS", errResp["error"]["code"])
}

func TestLogin_NormalizesEmail(t *testing.T) {
	_, e := setupHandler()

	registerUser(e)

	rec := doLogin(e, `{"email":"  Test@Example.COM  ","password":"password123"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Refresh tests ---

func TestRefresh_Success(t *testing.T) {
	_, e := setupHandler()

	regRec := registerUser(e)
	cookies := regRec.Result().Cookies()

	rec := doRefresh(e, cookies)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var data RegisterResponseData
	require.NoError(t, json.Unmarshal(resp["data"], &data))

	assert.Equal(t, uint(1), data.ID)
	assert.Equal(t, "testuser", data.Username)
	assert.NotEmpty(t, data.Token)
}

func TestRefresh_MissingCookie(t *testing.T) {
	_, e := setupHandler()

	rec := doRefresh(e, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefresh_InvalidToken(t *testing.T) {
	_, e := setupHandler()

	cookies := []*http.Cookie{
		{Name: "refresh_token", Value: "invalid-token"},
	}
	rec := doRefresh(e, cookies)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefresh_ExpiredToken(t *testing.T) {
	_, e := setupHandler()

	registerUser(e)

	expiredToken, err := generateExpiredRefreshToken(1, "test-jwt-secret")
	require.NoError(t, err)

	cookies := []*http.Cookie{
		{Name: "refresh_token", Value: expiredToken},
	}
	rec := doRefresh(e, cookies)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefresh_WithAccessToken_WrongAudience(t *testing.T) {
	_, e := setupHandler()

	registerUser(e)

	// Generate an access token and try to use it as refresh
	accessToken, err := GenerateAccessToken(1, "test-jwt-secret")
	require.NoError(t, err)

	cookies := []*http.Cookie{
		{Name: "refresh_token", Value: accessToken},
	}
	rec := doRefresh(e, cookies)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefresh_DeletedUser(t *testing.T) {
	_, e := setupHandler()

	// Generate a refresh token for a user that doesn't exist (ID 999)
	refreshToken, err := GenerateRefreshToken(999, "test-jwt-secret")
	require.NoError(t, err)

	cookies := []*http.Cookie{
		{Name: "refresh_token", Value: refreshToken},
	}
	rec := doRefresh(e, cookies)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefresh_ClearsCookieOnFailure(t *testing.T) {
	_, e := setupHandler()

	cookies := []*http.Cookie{
		{Name: "refresh_token", Value: "invalid-token"},
	}
	rec := doRefresh(e, cookies)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Check that cookie is cleared (MaxAge = -1)
	respCookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range respCookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie, "cleared refresh_token cookie should be present")
	assert.Equal(t, -1, refreshCookie.MaxAge)
}

// --- Logout tests ---

func TestLogout_Success(t *testing.T) {
	_, e := setupHandler()

	rec := doLogout(e)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Check cookie is cleared
	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie, "cleared refresh_token cookie should be present")
	assert.Equal(t, -1, refreshCookie.MaxAge)

	var resp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "logged out", resp["data"]["message"])
}
