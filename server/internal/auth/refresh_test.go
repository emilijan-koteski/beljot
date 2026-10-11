package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/identity"
	"github.com/emilijan/beljot/server/internal/refreshtoken"
)

const (
	testAccessTTL   = 15 * time.Minute
	testIdleTTL     = 30 * 24 * time.Hour
	testAbsoluteTTL = 180 * 24 * time.Hour
)

type mockRefreshRepo struct {
	mu     sync.Mutex
	rows   []*refreshtoken.RefreshToken
	nextID uint
}

func newMockRefreshRepo() *mockRefreshRepo {
	return &mockRefreshRepo{nextID: 1}
}

func (m *mockRefreshRepo) Create(t *refreshtoken.RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if r.TokenHash == t.TokenHash {
			return errDuplicateTokenHash
		}
	}
	stored := *t
	stored.ID = m.nextID
	m.nextID++
	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = time.Now()
	}
	m.rows = append(m.rows, &stored)
	return nil
}

func (m *mockRefreshRepo) FindByHash(hash string) (*refreshtoken.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if r.TokenHash == hash {
			cp := *r
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockRefreshRepo) RotateAndReplace(oldID uint, successor *refreshtoken.RefreshToken) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if r.ID == oldID && r.RotatedAt == nil && r.RevokedAt == nil {
			now := time.Now()
			r.RotatedAt = &now
			stored := *successor
			stored.ID = m.nextID
			m.nextID++
			if stored.CreatedAt.IsZero() {
				stored.CreatedAt = time.Now()
			}
			m.rows = append(m.rows, &stored)
			return true, nil
		}
	}
	return false, nil
}

func (m *mockRefreshRepo) RevokeFamily(familyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, r := range m.rows {
		if r.FamilyID == familyID && r.RevokedAt == nil {
			t := now
			r.RevokedAt = &t
		}
	}
	return nil
}

var errDuplicateTokenHash = &uniqueErr{}

type uniqueErr struct{}

func (*uniqueErr) Error() string { return "unique constraint: token_hash" }

func (m *mockRefreshRepo) backdateRotations(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if r.RotatedAt != nil {
			t := r.RotatedAt.Add(-d)
			r.RotatedAt = &t
		}
	}
}

func (m *mockRefreshRepo) expireIdleAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	past := time.Now().Add(-time.Hour)
	for _, r := range m.rows {
		r.ExpiresAt = past
	}
}

func (m *mockRefreshRepo) expireFamilyCapAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	past := time.Now().Add(-time.Hour)
	for _, r := range m.rows {
		r.FamilyExpiresAt = past
	}
}

func (m *mockRefreshRepo) byFamily(familyID string) []refreshtoken.RefreshToken {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]refreshtoken.RefreshToken, 0)
	for _, r := range m.rows {
		if r.FamilyID == familyID {
			out = append(out, *r)
		}
	}
	return out
}

func (m *mockRefreshRepo) familyRevoked(familyID string) bool {
	for _, r := range m.byFamily(familyID) {
		if r.RevokedAt == nil {
			return false
		}
	}
	return true
}

func (m *mockRefreshRepo) firstFamilyID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.rows) == 0 {
		return ""
	}
	return m.rows[0].FamilyID
}

func setupAuthServer() (*echo.Echo, *mockUserRepo, *mockRefreshRepo) {
	ur := newMockUserRepo()
	rr := newMockRefreshRepo()
	h := NewAuthHandler(ur, rr, newMockIdentityRepo(), identity.Registry{}, "test-jwt-secret", "development", testAccessTTL, testIdleTTL, testAbsoluteTTL)
	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	e.POST("/api/v1/auth/register", h.Register)
	e.POST("/api/v1/auth/login", h.Login)
	e.POST("/api/v1/auth/refresh", h.Refresh)
	e.POST("/api/v1/auth/logout", h.Logout)
	return e, ur, rr
}

func refreshCookieValue(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" && c.MaxAge >= 0 {
			return c.Value
		}
	}
	return ""
}

func cookieFor(value string) *http.Cookie {
	return &http.Cookie{Name: "refresh_token", Value: value}
}

func doLoginOn(e *echo.Echo, body string) *httptest.ResponseRecorder {
	return doLogin(e, body)
}

func TestRefresh_RotatesTokenEachUse(t *testing.T) {
	e, _, _ := setupAuthServer()
	reg := registerUser(e)
	require.Equal(t, http.StatusCreated, reg.Code)
	first := refreshCookieValue(reg)
	require.NotEmpty(t, first)

	rec := doRefresh(e, []*http.Cookie{cookieFor(first)})
	require.Equal(t, http.StatusOK, rec.Code)
	second := refreshCookieValue(rec)
	require.NotEmpty(t, second)
	assert.NotEqual(t, first, second, "refresh must rotate the cookie to a new value")
}

func TestRefresh_SlidesIdleKeepsAbsoluteCap(t *testing.T) {
	e, _, rr := setupAuthServer()
	reg := registerUser(e)
	fam := rr.firstFamilyID()
	before := rr.byFamily(fam)
	require.Len(t, before, 1)

	rec := doRefresh(e, []*http.Cookie{cookieFor(refreshCookieValue(reg))})
	require.Equal(t, http.StatusOK, rec.Code)

	after := rr.byFamily(fam)
	require.Len(t, after, 2, "rotation should mint a successor in the same family")
	successor := after[1]

	assert.WithinDuration(t, before[0].FamilyExpiresAt, successor.FamilyExpiresAt, time.Second)
	assert.WithinDuration(t, time.Now().Add(testIdleTTL), successor.ExpiresAt, time.Minute)
}

func TestRefresh_ReuseAfterGraceRevokesFamily(t *testing.T) {
	e, _, rr := setupAuthServer()
	reg := registerUser(e)
	first := refreshCookieValue(reg)

	rec := doRefresh(e, []*http.Cookie{cookieFor(first)})
	require.Equal(t, http.StatusOK, rec.Code)
	second := refreshCookieValue(rec)

	rr.backdateRotations(reuseGracePeriod + time.Minute)

	replay := doRefresh(e, []*http.Cookie{cookieFor(first)})
	assert.Equal(t, http.StatusUnauthorized, replay.Code, "replayed consumed token must be rejected")

	fam := rr.firstFamilyID()
	assert.True(t, rr.familyRevoked(fam), "reuse must revoke the whole family")

	afterRevoke := doRefresh(e, []*http.Cookie{cookieFor(second)})
	assert.Equal(t, http.StatusUnauthorized, afterRevoke.Code)
}

func TestRefresh_ReuseWithinGraceServesAccessOnly(t *testing.T) {
	e, _, rr := setupAuthServer()
	reg := registerUser(e)
	first := refreshCookieValue(reg)

	rec := doRefresh(e, []*http.Cookie{cookieFor(first)})
	require.Equal(t, http.StatusOK, rec.Code)
	successor := refreshCookieValue(rec)
	require.NotEmpty(t, successor)
	familyRowsBefore := len(rr.byFamily(rr.firstFamilyID()))

	healed := doRefresh(e, []*http.Cookie{cookieFor(first)})
	assert.Equal(t, http.StatusOK, healed.Code, "in-grace replay should be tolerated")
	assert.Empty(t, refreshCookieValue(healed), "heal must NOT rotate / set a new refresh cookie")

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(healed.Body.Bytes(), &resp))
	var data RegisterResponseData
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.NotEmpty(t, data.Token, "heal still returns a fresh access token")

	fam := rr.firstFamilyID()
	assert.False(t, rr.familyRevoked(fam), "in-grace race must not revoke the family")
	assert.Equal(t, familyRowsBefore, len(rr.byFamily(fam)), "heal must not mint a new refresh token row")

	after := doRefresh(e, []*http.Cookie{cookieFor(successor)})
	assert.Equal(t, http.StatusOK, after.Code, "the live successor keeps working")
}

func TestRefresh_IdleExpiryRevokes(t *testing.T) {
	e, _, rr := setupAuthServer()
	reg := registerUser(e)
	rr.expireIdleAll()

	rec := doRefresh(e, []*http.Cookie{cookieFor(refreshCookieValue(reg))})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.True(t, rr.familyRevoked(rr.firstFamilyID()))
}

func TestRefresh_AbsoluteCapRevokes(t *testing.T) {
	e, _, rr := setupAuthServer()
	reg := registerUser(e)
	rr.expireFamilyCapAll()

	rec := doRefresh(e, []*http.Cookie{cookieFor(refreshCookieValue(reg))})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.True(t, rr.familyRevoked(rr.firstFamilyID()))
}

func TestRefresh_MultiDeviceIsolation(t *testing.T) {
	e, _, rr := setupAuthServer()

	regA := registerUser(e)
	deviceA := refreshCookieValue(regA)

	loginB := doLoginOn(e, `{"email":"test@example.com","password":"password123"}`)
	require.Equal(t, http.StatusOK, loginB.Code)
	deviceB := refreshCookieValue(loginB)
	require.NotEqual(t, deviceA, deviceB)

	require.GreaterOrEqual(t, len(rr.rows), 2)

	out := doLogoutWith(e, deviceA)
	require.Equal(t, http.StatusOK, out.Code)

	recA := doRefresh(e, []*http.Cookie{cookieFor(deviceA)})
	assert.Equal(t, http.StatusUnauthorized, recA.Code, "device A is logged out")

	recB := doRefresh(e, []*http.Cookie{cookieFor(deviceB)})
	assert.Equal(t, http.StatusOK, recB.Code, "device B keeps refreshing")
}

func TestLogout_RevokesPresentedFamily(t *testing.T) {
	e, _, rr := setupAuthServer()
	reg := registerUser(e)
	token := refreshCookieValue(reg)

	out := doLogoutWith(e, token)
	require.Equal(t, http.StatusOK, out.Code)
	assert.True(t, rr.familyRevoked(rr.firstFamilyID()))

	rec := doRefresh(e, []*http.Cookie{cookieFor(token)})
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "refresh after logout must fail")
}

// doLogoutWith sends a logout carrying the refresh cookie so the handler can
// revoke that session's family.
func doLogoutWith(e *echo.Echo, refreshToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(cookieFor(refreshToken))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestRefresh_DeletedUser(t *testing.T) {
	e, _, rr := setupAuthServer()

	raw, hash, err := mintRefreshToken()
	require.NoError(t, err)
	fam, err := newFamilyID()
	require.NoError(t, err)
	require.NoError(t, rr.Create(&refreshtoken.RefreshToken{
		UserID:          999,
		FamilyID:        fam,
		TokenHash:       hash,
		ExpiresAt:       time.Now().Add(testIdleTTL),
		FamilyExpiresAt: time.Now().Add(testAbsoluteTTL),
	}))

	rec := doRefresh(e, []*http.Cookie{cookieFor(raw)})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.True(t, rr.familyRevoked(fam), "a gone user's family must be revoked, not left with live tokens")
}

// sanity: response envelope on a rotated refresh still echoes wallet fields.
func TestRefresh_RotatedEchoesWalletFields(t *testing.T) {
	e, _, _ := setupAuthServer()
	reg := registerUser(e)

	rec := doRefresh(e, []*http.Cookie{cookieFor(refreshCookieValue(reg))})
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data RegisterResponseData
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	assert.Equal(t, 5000, data.WalletBalance)
	assert.NotEmpty(t, data.Token)
}
