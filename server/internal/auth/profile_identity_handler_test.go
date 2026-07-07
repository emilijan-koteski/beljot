package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/identity"
	"github.com/emilijan/beljot/server/internal/user"
)

// --- authed self-management setup: real AuthMiddleware + JWT, mocked repos ---

func setupIdentityHandler() (*echo.Echo, *mockUserRepo, *mockIdentityRepo, *fakeProvider) {
	ur := newMockUserRepo()
	ir := newMockIdentityRepo()
	fp := newFakeProvider()
	h := NewAuthHandler(ur, newMockRefreshRepo(), ir, identity.Registry{fp.Name(): fp},
		"test-jwt-secret", "development", testAccessTTL, testIdleTTL, testAbsoluteTTL)
	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", AuthMiddleware("test-jwt-secret"))
	api.GET("/users/:id/identities", h.ListIdentities)
	api.POST("/users/:id/identities/:provider", h.LinkIdentity)
	api.DELETE("/users/:id/identities/:provider", h.UnlinkIdentity)
	return e, ur, ir, fp
}

// doAuthedReq issues a request authenticated as authUserID (Bearer JWT). Pass
// authUserID 0 to send no Authorization header (unauthenticated case).
func doAuthedReq(e *echo.Echo, method, target string, authUserID uint, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if authUserID != 0 {
		token, _ := GenerateAccessToken(authUserID, "test-jwt-secret")
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func seedUser(ur *mockUserRepo, email, passwordHash string) *user.User {
	u := &user.User{Email: email, Username: email, PasswordHash: passwordHash}
	_ = ur.Create(u)
	return u
}

func seedIdentity(ir *mockIdentityRepo, userID uint, provider, subject, email string) {
	_ = ir.Create(&identity.Identity{UserID: userID, Provider: provider, ProviderUserID: subject, Email: email})
}

func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp["error"]["code"]
}

// --- ListIdentities ---

func TestListIdentities_ReturnsIdentitiesAndHasPassword(t *testing.T) {
	e, ur, ir, _ := setupIdentityHandler()
	u := seedUser(ur, "player@example.com", "hashed-pw")
	seedIdentity(ir, u.ID, "fakeprov", "sub-secret-123", "g@example.com")

	rec := doAuthedReq(e, http.MethodGet, "/api/v1/users/1/identities", u.ID, "")
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var resp struct {
		Data LinkedAccountsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Data.HasPassword)
	require.Len(t, resp.Data.Identities, 1)
	assert.Equal(t, "fakeprov", resp.Data.Identities[0].Provider)
	assert.Equal(t, "g@example.com", resp.Data.Identities[0].Email)
	assert.False(t, resp.Data.Identities[0].CreatedAt.IsZero())
	// The provider subject (Google `sub`) must never reach the client.
	assert.NotContains(t, rec.Body.String(), "sub-secret-123")
}

func TestListIdentities_PasswordlessReportsHasPasswordFalse(t *testing.T) {
	e, ur, ir, _ := setupIdentityHandler()
	u := seedUser(ur, "sso@example.com", "") // empty-string sentinel = no password
	seedIdentity(ir, u.ID, "fakeprov", "sub1", "sso@example.com")

	rec := doAuthedReq(e, http.MethodGet, "/api/v1/users/1/identities", u.ID, "")
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data LinkedAccountsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Data.HasPassword)
}

func TestListIdentities_EmptyListSerializesAsArray(t *testing.T) {
	e, ur, _, _ := setupIdentityHandler()
	u := seedUser(ur, "noidents@example.com", "hashed-pw")

	rec := doAuthedReq(e, http.MethodGet, "/api/v1/users/1/identities", u.ID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	// Never null — the client maps over identities.
	assert.Contains(t, rec.Body.String(), `"identities":[]`)
}

func TestListIdentities_ForeignIDForbidden(t *testing.T) {
	e, ur, _, _ := setupIdentityHandler()
	seedUser(ur, "one@example.com", "pw")   // id 1
	other := seedUser(ur, "two@example.com", "pw") // id 2

	rec := doAuthedReq(e, http.MethodGet, "/api/v1/users/1/identities", other.ID, "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "FORBIDDEN", decodeErrorCode(t, rec))
}

func TestListIdentities_Unauthenticated(t *testing.T) {
	e, ur, _, _ := setupIdentityHandler()
	seedUser(ur, "one@example.com", "pw")

	rec := doAuthedReq(e, http.MethodGet, "/api/v1/users/1/identities", 0, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- LinkIdentity ---

func TestLinkIdentity_Success(t *testing.T) {
	e, ur, ir, fp := setupIdentityHandler()
	u := seedUser(ur, "player@example.com", "hashed-pw")
	fp.add("cred-new", identity.ExternalIdentity{Subject: "sub-new", Email: "g@example.com", EmailVerified: true})

	rec := doAuthedReq(e, http.MethodPost, "/api/v1/users/1/identities/fakeprov", u.ID, `{"credential":"cred-new"}`)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var resp struct {
		Data IdentityView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "fakeprov", resp.Data.Provider)
	assert.Equal(t, "g@example.com", resp.Data.Email)

	stored, _ := ir.FindByUserID(u.ID)
	require.Len(t, stored, 1)
	assert.Equal(t, "sub-new", stored[0].ProviderUserID)
}

func TestLinkIdentity_IdempotentWhenAlreadyLinkedToSelf(t *testing.T) {
	e, ur, ir, fp := setupIdentityHandler()
	u := seedUser(ur, "player@example.com", "hashed-pw")
	seedIdentity(ir, u.ID, "fakeprov", "sub-1", "g@example.com")
	fp.add("cred-1", identity.ExternalIdentity{Subject: "sub-1", Email: "g@example.com", EmailVerified: true})

	rec := doAuthedReq(e, http.MethodPost, "/api/v1/users/1/identities/fakeprov", u.ID, `{"credential":"cred-1"}`)
	require.Equal(t, http.StatusOK, rec.Code, "idempotent re-link is 200, not an error; body: %s", rec.Body.String())

	stored, _ := ir.FindByUserID(u.ID)
	assert.Len(t, stored, 1, "no duplicate identity created")
}

func TestLinkIdentity_ConflictWhenSubjectLinkedToAnotherUser(t *testing.T) {
	e, ur, ir, fp := setupIdentityHandler()
	seedUser(ur, "owner@example.com", "pw")        // id 1 already owns the Google account
	caller := seedUser(ur, "caller@example.com", "pw") // id 2
	seedIdentity(ir, 1, "fakeprov", "sub-shared", "g@example.com")
	fp.add("cred-shared", identity.ExternalIdentity{Subject: "sub-shared", Email: "g@example.com", EmailVerified: true})

	rec := doAuthedReq(e, http.MethodPost, "/api/v1/users/2/identities/fakeprov", caller.ID, `{"credential":"cred-shared"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SSO_IDENTITY_IN_USE", decodeErrorCode(t, rec))
}

func TestLinkIdentity_ConflictWhenUserAlreadyHasProvider(t *testing.T) {
	e, ur, ir, fp := setupIdentityHandler()
	u := seedUser(ur, "player@example.com", "pw")
	seedIdentity(ir, u.ID, "fakeprov", "sub-old", "old@example.com")
	// A different Google account for a provider the user already linked.
	fp.add("cred-other", identity.ExternalIdentity{Subject: "sub-other", Email: "other@example.com", EmailVerified: true})

	rec := doAuthedReq(e, http.MethodPost, "/api/v1/users/1/identities/fakeprov", u.ID, `{"credential":"cred-other"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SSO_IDENTITY_IN_USE", decodeErrorCode(t, rec))
}

func TestLinkIdentity_UnverifiedEmail(t *testing.T) {
	e, ur, _, fp := setupIdentityHandler()
	u := seedUser(ur, "player@example.com", "pw")
	fp.add("cred-unverified", identity.ExternalIdentity{Subject: "sub-x", Email: "g@example.com", EmailVerified: false})

	rec := doAuthedReq(e, http.MethodPost, "/api/v1/users/1/identities/fakeprov", u.ID, `{"credential":"cred-unverified"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "SSO_EMAIL_UNVERIFIED", decodeErrorCode(t, rec))
}

func TestLinkIdentity_BadCredential(t *testing.T) {
	e, ur, _, _ := setupIdentityHandler()
	u := seedUser(ur, "player@example.com", "pw")

	rec := doAuthedReq(e, http.MethodPost, "/api/v1/users/1/identities/fakeprov", u.ID, `{"credential":"totally-bogus"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "SSO_INVALID_CREDENTIAL", decodeErrorCode(t, rec))
}

func TestLinkIdentity_UnknownProvider(t *testing.T) {
	e, ur, _, _ := setupIdentityHandler()
	u := seedUser(ur, "player@example.com", "pw")

	rec := doAuthedReq(e, http.MethodPost, "/api/v1/users/1/identities/facebook", u.ID, `{"credential":"cred"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "SSO_UNKNOWN_PROVIDER", decodeErrorCode(t, rec))
}

func TestLinkIdentity_UserNotFound(t *testing.T) {
	e, _, _, fp := setupIdentityHandler()
	// No user seeded: the self-only guard passes (token subject == :id) but the
	// account no longer exists, so no orphan identity may be created.
	fp.add("cred", identity.ExternalIdentity{Subject: "sub", Email: "g@example.com", EmailVerified: true})

	rec := doAuthedReq(e, http.MethodPost, "/api/v1/users/42/identities/fakeprov", 42, `{"credential":"cred"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "USER_NOT_FOUND", decodeErrorCode(t, rec))
}

func TestLinkIdentity_ForeignIDForbidden(t *testing.T) {
	e, ur, _, _ := setupIdentityHandler()
	seedUser(ur, "one@example.com", "pw")
	other := seedUser(ur, "two@example.com", "pw")

	rec := doAuthedReq(e, http.MethodPost, "/api/v1/users/1/identities/fakeprov", other.ID, `{"credential":"cred"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- UnlinkIdentity ---

func TestUnlinkIdentity_SuccessWhenAccountHasPassword(t *testing.T) {
	e, ur, ir, _ := setupIdentityHandler()
	u := seedUser(ur, "player@example.com", "hashed-pw")
	seedIdentity(ir, u.ID, "fakeprov", "sub-1", "g@example.com")

	rec := doAuthedReq(e, http.MethodDelete, "/api/v1/users/1/identities/fakeprov", u.ID, "")
	require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())

	stored, _ := ir.FindByUserID(u.ID)
	assert.Empty(t, stored)
}

func TestUnlinkIdentity_PasswordlessLastMethodBlocked(t *testing.T) {
	e, ur, ir, _ := setupIdentityHandler()
	u := seedUser(ur, "sso@example.com", "") // passwordless
	seedIdentity(ir, u.ID, "fakeprov", "sub-1", "sso@example.com")

	rec := doAuthedReq(e, http.MethodDelete, "/api/v1/users/1/identities/fakeprov", u.ID, "")
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SSO_CANNOT_UNLINK_LAST", decodeErrorCode(t, rec))

	stored, _ := ir.FindByUserID(u.ID)
	assert.Len(t, stored, 1, "nothing removed when it would lock the account out")
}

func TestUnlinkIdentity_PasswordlessAllowedWhenAnotherRemains(t *testing.T) {
	e, ur, ir, _ := setupIdentityHandler()
	u := seedUser(ur, "sso@example.com", "") // passwordless
	seedIdentity(ir, u.ID, "fakeprov", "sub-1", "sso@example.com")
	seedIdentity(ir, u.ID, "otherprov", "sub-2", "sso@example.com")

	rec := doAuthedReq(e, http.MethodDelete, "/api/v1/users/1/identities/fakeprov", u.ID, "")
	require.Equal(t, http.StatusNoContent, rec.Code)

	stored, _ := ir.FindByUserID(u.ID)
	require.Len(t, stored, 1)
	assert.Equal(t, "otherprov", stored[0].Provider)
}

func TestUnlinkIdentity_NotLinked(t *testing.T) {
	e, ur, _, _ := setupIdentityHandler()
	u := seedUser(ur, "player@example.com", "pw")

	rec := doAuthedReq(e, http.MethodDelete, "/api/v1/users/1/identities/fakeprov", u.ID, "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "SSO_IDENTITY_NOT_FOUND", decodeErrorCode(t, rec))
}

func TestUnlinkIdentity_ForeignIDForbidden(t *testing.T) {
	e, ur, ir, _ := setupIdentityHandler()
	seedUser(ur, "one@example.com", "pw")
	other := seedUser(ur, "two@example.com", "pw")
	seedIdentity(ir, 1, "fakeprov", "sub-1", "g@example.com")

	rec := doAuthedReq(e, http.MethodDelete, "/api/v1/users/1/identities/fakeprov", other.ID, "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
