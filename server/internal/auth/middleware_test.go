package auth

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateExpiredAccessToken(userID uint, secret string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		Audience:  jwt.ClaimStrings{"access"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

const testSecret = "test-jwt-secret"

func setupMiddlewareTest() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler

	protected := e.Group("", AuthMiddleware(testSecret))
	protected.GET("/protected", func(c echo.Context) error {
		userID, err := GetUserID(c)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]uint{"userID": userID})
	})

	return e
}

func doProtectedRequest(e *echo.Echo, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestMiddleware_AllowsValidAccessToken(t *testing.T) {
	e := setupMiddlewareTest()

	token, err := GenerateAccessToken(42, testSecret)
	require.NoError(t, err)

	rec := doProtectedRequest(e, "Bearer "+token)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_RejectsExpiredToken(t *testing.T) {
	e := setupMiddlewareTest()

	token, err := generateExpiredAccessToken(1, testSecret)
	require.NoError(t, err)

	rec := doProtectedRequest(e, "Bearer "+token)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_RejectsWrongSecret(t *testing.T) {
	e := setupMiddlewareTest()

	token, err := GenerateAccessToken(1, "wrong-secret")
	require.NoError(t, err)

	rec := doProtectedRequest(e, "Bearer "+token)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_RejectsMissingAuthHeader(t *testing.T) {
	e := setupMiddlewareTest()

	rec := doProtectedRequest(e, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_RejectsMalformedBearerToken(t *testing.T) {
	e := setupMiddlewareTest()

	rec := doProtectedRequest(e, "Bearer")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_RejectsNonBearerScheme(t *testing.T) {
	e := setupMiddlewareTest()

	token, err := GenerateAccessToken(1, testSecret)
	require.NoError(t, err)

	rec := doProtectedRequest(e, "Basic "+token)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_RejectsNonAccessAudience(t *testing.T) {
	e := setupMiddlewareTest()

	// A validly-signed, unexpired JWT whose audience is NOT "access" must be
	// rejected — the middleware gates on the access audience, not just signature.
	claims := jwt.RegisteredClaims{
		Subject:   "1",
		Audience:  jwt.ClaimStrings{"refresh"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	require.NoError(t, err)

	rec := doProtectedRequest(e, "Bearer "+token)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetUserID_ExtractsFromContext(t *testing.T) {
	e := setupMiddlewareTest()

	token, err := GenerateAccessToken(42, testSecret)
	require.NoError(t, err)

	rec := doProtectedRequest(e, "Bearer "+token)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"userID":42`)
}
