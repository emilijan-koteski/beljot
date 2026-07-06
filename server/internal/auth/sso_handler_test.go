package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/identity"
	"github.com/emilijan/beljot/server/internal/user"
)

// --- in-memory identity repository mimicking the GORM impl's semantics ---

type mockIdentityRepo struct {
	identities []*identity.Identity
	nextID     uint
	// createErr, when set, is returned by Create before any insert — lets
	// tests simulate the identity insert failing after the user insert.
	createErr error
}

func newMockIdentityRepo() *mockIdentityRepo {
	return &mockIdentityRepo{nextID: 1}
}

func (m *mockIdentityRepo) Create(ident *identity.Identity) error {
	if m.createErr != nil {
		return m.createErr
	}
	for _, ex := range m.identities {
		// Mirror the two unique indexes: (provider, provider_user_id) and
		// (user_id, provider) both collapse to ErrSSOIdentityInUse.
		if ex.Provider == ident.Provider && (ex.ProviderUserID == ident.ProviderUserID || ex.UserID == ident.UserID) {
			return apperr.ErrSSOIdentityInUse
		}
	}
	ident.ID = m.nextID
	ident.CreatedAt = time.Now()
	m.nextID++
	m.identities = append(m.identities, ident)
	return nil
}

func (m *mockIdentityRepo) FindByProviderSubject(provider, subject string) (*identity.Identity, error) {
	for _, ex := range m.identities {
		if ex.Provider == provider && ex.ProviderUserID == subject {
			cp := *ex
			return &cp, nil
		}
	}
	return nil, nil
}

// --- fake SSO provider — the reason the Provider interface exists: handler
// tests exercise the full flow without ever talking to Google ---

type fakeProvider struct {
	name        string
	credentials map[string]identity.ExternalIdentity
	// verifyCalls counts Verify invocations — input-floor tests assert the
	// handler never reaches the (outbound, in production) verifier.
	verifyCalls int
}

func newFakeProvider() *fakeProvider {
	// A non-"google" name proves the handler flow is provider-agnostic.
	return &fakeProvider{name: "fakeprov", credentials: map[string]identity.ExternalIdentity{}}
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Verify(_ context.Context, credential string) (*identity.ExternalIdentity, error) {
	f.verifyCalls++
	ext, ok := f.credentials[credential]
	if !ok {
		return nil, apperr.ErrSSOInvalidCredential
	}
	cp := ext
	return &cp, nil
}

// add registers a credential the fake provider will accept.
func (f *fakeProvider) add(credential string, ext identity.ExternalIdentity) {
	f.credentials[credential] = ext
}

// --- server + request helpers ---

func setupSSOHandler() (*echo.Echo, *mockUserRepo, *mockIdentityRepo, *fakeProvider) {
	ur := newMockUserRepo()
	ir := newMockIdentityRepo()
	fp := newFakeProvider()
	h := NewAuthHandler(ur, newMockRefreshRepo(), ir, identity.Registry{fp.Name(): fp}, "test-jwt-secret", "development", testAccessTTL, testIdleTTL, testAbsoluteTTL)
	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	e.POST("/api/v1/auth/register", h.Register)
	e.POST("/api/v1/auth/login", h.Login)
	e.POST("/api/v1/auth/sso/:provider", h.SSOLogin)
	e.POST("/api/v1/auth/sso/:provider/link", h.SSOLink)
	return e, ur, ir, fp
}

func doSSOLogin(e *echo.Echo, provider, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/"+provider, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doSSOLink(e *echo.Echo, provider, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/"+provider+"/link", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decodeAuthData(t *testing.T, rec *httptest.ResponseRecorder) RegisterResponseData {
	t.Helper()
	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data RegisterResponseData
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	return data
}

// errCode is shared with password_reset_test.go (same package).

func hasRefreshCookie(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" && c.MaxAge > 0 {
			return true
		}
	}
	return false
}

// verifiedIdentity is the default happy-path external identity.
func verifiedIdentity(sub, email, name string) identity.ExternalIdentity {
	return identity.ExternalIdentity{Subject: sub, Email: email, EmailVerified: true, DisplayName: name}
}

// --- SSO registration (fresh Google account) ---

func TestSSOLogin_RegistersNewUser(t *testing.T) {
	e, ur, ir, fp := setupSSOHandler()
	fp.add("cred-1", verifiedIdentity("sub-1", "new@example.com", "New Player"))

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	data := decodeAuthData(t, rec)
	assert.Equal(t, "new@example.com", data.Email)
	assert.Equal(t, "NewPlayer", data.Username)
	assert.NotEmpty(t, data.Token)
	assert.True(t, hasRefreshCookie(rec), "SSO registration must open a refresh session")

	// Wallet/streak seeded exactly like Register (balance 5000, streak 0,
	// last_login stamped today UTC), and the password-hash sentinel is "".
	require.Len(t, ur.users, 1)
	u := ur.users[0]
	assert.Equal(t, "", u.PasswordHash)
	assert.Equal(t, 5000, u.WalletBalance)
	assert.Equal(t, 0, u.LoginStreakDays)
	require.NotNil(t, u.LastLoginAt)
	gotY, gotM, gotD := u.LastLoginAt.UTC().Date()
	wantY, wantM, wantD := time.Now().UTC().Date()
	assert.Equal(t, wantY, gotY)
	assert.Equal(t, wantM, gotM)
	assert.Equal(t, wantD, gotD)

	// Identity row links (provider, subject) to the new user.
	ident, err := ir.FindByProviderSubject("fakeprov", "sub-1")
	require.NoError(t, err)
	require.NotNil(t, ident)
	assert.Equal(t, u.ID, ident.UserID)
	assert.Equal(t, "new@example.com", ident.Email)
}

// --- SSO login (identity already exists) ---

func TestSSOLogin_ExistingIdentityLogsIn(t *testing.T) {
	e, _, _, fp := setupSSOHandler()
	fp.add("cred-1", verifiedIdentity("sub-1", "player@example.com", "Player One"))

	first := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	require.Equal(t, http.StatusCreated, first.Code)
	created := decodeAuthData(t, first)

	second := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	require.Equal(t, http.StatusOK, second.Code, "second SSO login must be a login, not a registration")

	data := decodeAuthData(t, second)
	assert.Equal(t, created.ID, data.ID)
	assert.Equal(t, created.Username, data.Username)
	assert.NotEmpty(t, data.Token)
	assert.True(t, hasRefreshCookie(second))
}

func TestSSOLogin_ExistingIdentitySkipsEmailVerifiedGate(t *testing.T) {
	e, _, _, fp := setupSSOHandler()
	fp.add("cred-1", verifiedIdentity("sub-1", "player@example.com", "Player One"))
	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	require.Equal(t, http.StatusCreated, rec.Code)

	// The provider later reports the email unverified — an already-linked
	// identity still logs in (the gate only guards email-based matching).
	fp.add("cred-1", identity.ExternalIdentity{Subject: "sub-1", Email: "player@example.com", EmailVerified: false, DisplayName: "Player One"})
	again := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusOK, again.Code)
}

// --- Email collision → link required ---

func TestSSOLogin_EmailCollisionRequiresLink(t *testing.T) {
	e, ur, ir, fp := setupSSOHandler()
	doRegister(e, `{"email":"taken@example.com","username":"taken","password":"password123"}`)
	fp.add("cred-1", verifiedIdentity("sub-1", "taken@example.com", "Taken Name"))

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SSO_LINK_REQUIRED", errCode(t, rec))

	// No identity is created and no second user appears.
	assert.Empty(t, ir.identities)
	assert.Len(t, ur.users, 1)
}

func TestSSOLogin_EmailCollisionMatchesNormalizedEmail(t *testing.T) {
	e, _, _, fp := setupSSOHandler()
	doRegister(e, `{"email":"taken@example.com","username":"taken","password":"password123"}`)
	// Provider reports the email with different casing — still a collision.
	fp.add("cred-1", verifiedIdentity("sub-1", "Taken@Example.COM", "Taken Name"))

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SSO_LINK_REQUIRED", errCode(t, rec))
}

// --- Unverified email ---

func TestSSOLogin_UnverifiedEmailRejected(t *testing.T) {
	e, ur, ir, fp := setupSSOHandler()
	fp.add("cred-1", identity.ExternalIdentity{Subject: "sub-1", Email: "new@example.com", EmailVerified: false, DisplayName: "New Player"})

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "SSO_EMAIL_UNVERIFIED", errCode(t, rec))

	// Nothing created, nothing matched.
	assert.Empty(t, ur.users)
	assert.Empty(t, ir.identities)
}

// --- Bad credential ---

func TestSSOLogin_InvalidCredential(t *testing.T) {
	e, _, _, _ := setupSSOHandler()

	rec := doSSOLogin(e, "fakeprov", `{"credential":"forged"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "SSO_INVALID_CREDENTIAL", errCode(t, rec))
}

// --- Input floors: rejected before the provider is ever called ---

func TestSSOLogin_RejectsUnusableCredentialBeforeVerify(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty credential", body: `{"credential":""}`},
		{name: "whitespace credential", body: `{"credential":"   "}`},
		{name: "missing credential field", body: `{}`},
		{name: "oversized credential", body: `{"credential":"` + strings.Repeat("a", 4097) + `"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, ur, ir, fp := setupSSOHandler()

			rec := doSSOLogin(e, "fakeprov", tc.body)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Equal(t, "SSO_INVALID_CREDENTIAL", errCode(t, rec))
			assert.Equal(t, 0, fp.verifyCalls, "provider.Verify must never be reached")
			assert.Empty(t, ur.users)
			assert.Empty(t, ir.identities)
		})
	}
}

func TestSSOLink_RejectsUnusableCredentialBeforeVerify(t *testing.T) {
	e, _, ir, fp := setupSSOHandler()

	rec := doSSOLink(e, "fakeprov", `{"credential":"  ","password":"password123"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "SSO_INVALID_CREDENTIAL", errCode(t, rec))
	assert.Equal(t, 0, fp.verifyCalls, "provider.Verify must never be reached")
	assert.Empty(t, ir.identities)
}

func TestSSOLogin_RejectsEmptySubjectFromProvider(t *testing.T) {
	e, ur, ir, fp := setupSSOHandler()
	fp.add("cred-1", identity.ExternalIdentity{Subject: "", Email: "new@example.com", EmailVerified: true})

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "SSO_INVALID_CREDENTIAL", errCode(t, rec))
	assert.Empty(t, ur.users)
	assert.Empty(t, ir.identities)
}

func TestSSOLogin_RejectsOversizedProviderEmail(t *testing.T) {
	e, ur, _, fp := setupSSOHandler()
	longEmail := strings.Repeat("a", 250) + "@example.com" // > 255 after normalization
	fp.add("cred-1", verifiedIdentity("sub-1", longEmail, "Long Email"))

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "SSO_INVALID_CREDENTIAL", errCode(t, rec))
	assert.Empty(t, ur.users)
}

// --- Registration write failures: compensation and race mapping ---

func TestSSOLogin_IdentityCreateFailureDeletesOrphanedUser(t *testing.T) {
	e, ur, ir, fp := setupSSOHandler()
	ir.createErr = errors.New("identity insert failed")
	fp.add("cred-1", verifiedIdentity("sub-1", "new@example.com", "New Player"))

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// The compensating delete removes the just-created passwordless user so
	// its email is not bricked, and no session is issued.
	assert.Empty(t, ur.users, "orphaned user must be deleted by the compensating action")
	assert.Empty(t, ir.identities)
	assert.False(t, hasRefreshCookie(rec), "a failed registration must not open a session")

	// The email is free again: the same credential can register cleanly.
	ir.createErr = nil
	retry := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusCreated, retry.Code, "body: %s", retry.Body.String())
}

func TestSSOLogin_EmailInsertRaceRequiresLink(t *testing.T) {
	e, ur, ir, fp := setupSSOHandler()
	// The pre-check sees no user, but the insert loses the race to a
	// concurrent password registration — the repo surfaces ErrEmailTaken.
	ur.createErrs = []error{apperr.ErrEmailTaken}
	fp.add("cred-1", verifiedIdentity("sub-1", "raced@example.com", "Raced Player"))

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SSO_LINK_REQUIRED", errCode(t, rec), "an insert-time email race must answer like the pre-check")
	assert.Empty(t, ir.identities)
}

func TestSSOLogin_UsernameInsertRaceRetriesCreate(t *testing.T) {
	e, ur, _, fp := setupSSOHandler()
	// First insert loses a username race; the retry with a regenerated
	// username succeeds.
	ur.createErrs = []error{apperr.ErrUsernameTaken}
	fp.add("cred-1", verifiedIdentity("sub-1", "new@example.com", "New Player"))

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
	require.Len(t, ur.users, 1)
}

func TestSSOLogin_UsernameInsertRaceGivesUpAfterBoundedRetries(t *testing.T) {
	e, ur, ir, fp := setupSSOHandler()
	ur.createErrs = []error{apperr.ErrUsernameTaken, apperr.ErrUsernameTaken, apperr.ErrUsernameTaken}
	fp.add("cred-1", verifiedIdentity("sub-1", "new@example.com", "New Player"))

	rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Empty(t, ur.users)
	assert.Empty(t, ir.identities)
}

// --- Unknown provider ---

func TestSSOLogin_UnknownProvider(t *testing.T) {
	e, _, _, _ := setupSSOHandler()

	rec := doSSOLogin(e, "facebook", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "SSO_UNKNOWN_PROVIDER", errCode(t, rec))
}

func TestSSOLink_UnknownProvider(t *testing.T) {
	e, _, _, _ := setupSSOHandler()

	rec := doSSOLink(e, "facebook", `{"credential":"cred-1","password":"password123"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "SSO_UNKNOWN_PROVIDER", errCode(t, rec))
}

// --- Link during login ---

func TestSSOLink_CorrectPasswordLinksAndLogsIn(t *testing.T) {
	e, _, ir, fp := setupSSOHandler()
	doRegister(e, `{"email":"taken@example.com","username":"taken","password":"password123"}`)
	fp.add("cred-1", verifiedIdentity("sub-1", "taken@example.com", "Taken Name"))

	rec := doSSOLink(e, "fakeprov", `{"credential":"cred-1","password":"password123"}`)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	data := decodeAuthData(t, rec)
	assert.Equal(t, "taken", data.Username)
	assert.NotEmpty(t, data.Token)
	assert.True(t, hasRefreshCookie(rec))

	ident, err := ir.FindByProviderSubject("fakeprov", "sub-1")
	require.NoError(t, err)
	require.NotNil(t, ident)

	// The next SSO login goes straight through — no link dialog again.
	next := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
	assert.Equal(t, http.StatusOK, next.Code)
}

func TestSSOLink_RetryOfExistingLinkSucceedsIdempotently(t *testing.T) {
	e, _, ir, fp := setupSSOHandler()
	doRegister(e, `{"email":"taken@example.com","username":"taken","password":"password123"}`)
	fp.add("cred-1", verifiedIdentity("sub-1", "taken@example.com", "Taken Name"))

	first := doSSOLink(e, "fakeprov", `{"credential":"cred-1","password":"password123"}`)
	require.Equal(t, http.StatusOK, first.Code)

	// The client never saw the response and retries the same link: the
	// identity row already belongs to the same user, so this must be a 200
	// with a fresh session — not a 409.
	second := doSSOLink(e, "fakeprov", `{"credential":"cred-1","password":"password123"}`)
	require.Equal(t, http.StatusOK, second.Code, "body: %s", second.Body.String())

	data := decodeAuthData(t, second)
	assert.Equal(t, "taken", data.Username)
	assert.NotEmpty(t, data.Token)
	assert.True(t, hasRefreshCookie(second), "the idempotent retry must still open a session")
	assert.Len(t, ir.identities, 1, "no duplicate identity row may be created")
}

func TestSSOLink_IdentityOwnedByAnotherUserConflicts(t *testing.T) {
	e, _, ir, fp := setupSSOHandler()
	// sub-1 registers via SSO and owns the (provider, subject) identity.
	fp.add("cred-orig", verifiedIdentity("sub-1", "owner@example.com", "Owner"))
	require.Equal(t, http.StatusCreated, doSSOLogin(e, "fakeprov", `{"credential":"cred-orig"}`).Code)

	// A different password account tries to link the SAME subject: the
	// wrapped ErrSSOIdentityInUse must surface as 409 through the error
	// handler, and never as an idempotent success.
	doRegister(e, `{"email":"other@example.com","username":"otheruser","password":"password123"}`)
	fp.add("cred-cross", verifiedIdentity("sub-1", "other@example.com", "Other"))

	rec := doSSOLink(e, "fakeprov", `{"credential":"cred-cross","password":"password123"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SSO_IDENTITY_IN_USE", errCode(t, rec))
	assert.False(t, hasRefreshCookie(rec), "a rejected link must not open a session")
	assert.Len(t, ir.identities, 1, "the identity must stay with its original owner")
}

func TestSSOLink_WrongPasswordLinksNothing(t *testing.T) {
	e, _, ir, fp := setupSSOHandler()
	doRegister(e, `{"email":"taken@example.com","username":"taken","password":"password123"}`)
	fp.add("cred-1", verifiedIdentity("sub-1", "taken@example.com", "Taken Name"))

	rec := doSSOLink(e, "fakeprov", `{"credential":"cred-1","password":"wrongpassword"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "INVALID_CREDENTIALS", errCode(t, rec))
	assert.Empty(t, ir.identities)
}

func TestSSOLink_PasswordlessAccountRejected(t *testing.T) {
	e, _, ir, fp := setupSSOHandler()
	// An SSO-only account (empty password hash) for the same email, created
	// through a second provider identity in a fresh registration.
	fp.add("cred-orig", verifiedIdentity("sub-orig", "ssoonly@example.com", "Sso Only"))
	require.Equal(t, http.StatusCreated, doSSOLogin(e, "fakeprov", `{"credential":"cred-orig"}`).Code)

	// A different subject with the same email tries to link with "its password"
	// — there is none, so this must fail exactly like a wrong password.
	fp.add("cred-2", verifiedIdentity("sub-2", "ssoonly@example.com", "Sso Only"))
	rec := doSSOLink(e, "fakeprov", `{"credential":"cred-2","password":"anything123"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "INVALID_CREDENTIALS", errCode(t, rec))
	assert.Len(t, ir.identities, 1, "only the original identity may exist")
}

func TestSSOLink_UnknownEmailRejected(t *testing.T) {
	e, _, ir, fp := setupSSOHandler()
	fp.add("cred-1", verifiedIdentity("sub-1", "nobody@example.com", "No Body"))

	// No oracle: a missing account answers exactly like a wrong password.
	rec := doSSOLink(e, "fakeprov", `{"credential":"cred-1","password":"password123"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "INVALID_CREDENTIALS", errCode(t, rec))
	assert.Empty(t, ir.identities)
}

func TestSSOLink_UnverifiedEmailRejected(t *testing.T) {
	e, _, ir, fp := setupSSOHandler()
	doRegister(e, `{"email":"taken@example.com","username":"taken","password":"password123"}`)
	fp.add("cred-1", identity.ExternalIdentity{Subject: "sub-1", Email: "taken@example.com", EmailVerified: false, DisplayName: "Taken Name"})

	rec := doSSOLink(e, "fakeprov", `{"credential":"cred-1","password":"password123"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "SSO_EMAIL_UNVERIFIED", errCode(t, rec))
	assert.Empty(t, ir.identities)
}

func TestSSOLink_InvalidCredential(t *testing.T) {
	e, _, _, _ := setupSSOHandler()

	rec := doSSOLink(e, "fakeprov", `{"credential":"forged","password":"password123"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "SSO_INVALID_CREDENTIAL", errCode(t, rec))
}

// --- Password login on an SSO-only account ---

func TestLogin_SSOOnlyAccountFailsWithoutPanic(t *testing.T) {
	e, ur, _, fp := setupSSOHandler()
	fp.add("cred-1", verifiedIdentity("sub-1", "ssoonly@example.com", "Sso Only"))
	require.Equal(t, http.StatusCreated, doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`).Code)
	require.Equal(t, "", ur.users[0].PasswordHash, "SSO registration must not set a password hash")

	// bcrypt against the empty-hash sentinel errors safely — 401, no panic.
	rec := doLogin(e, `{"email":"ssoonly@example.com","password":"password123"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "INVALID_CREDENTIALS", errCode(t, rec))
}

// CheckPassword against the empty-hash sentinel must error, never match —
// the invariant the whole passwordless-account design leans on.
func TestCheckPassword_EmptyHashFails(t *testing.T) {
	assert.Error(t, CheckPassword("", "anything"))
	assert.Error(t, CheckPassword("", ""))
}

// --- Username generation ---

func TestSSOLogin_UsernameGeneration(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		email       string
		existing    []string
		want        string
	}{
		{
			name:        "display name sanitized to username alphabet",
			displayName: "Ана Мария-Смит",
			email:       "ana@example.com",
			want:        "ana", // Cyrillic strips to nothing → email local-part
		},
		{
			name:        "latin display name keeps letters and digits",
			displayName: "John Smith 3rd!",
			email:       "john@example.com",
			want:        "JohnSmith3rd",
		},
		{
			name:        "email local-part used when display name empty",
			displayName: "",
			email:       "local.part+tag@example.com",
			want:        "localparttag",
		},
		{
			name:        "player fallback when nothing sanitizes",
			displayName: "!!",
			email:       "--@example.com",
			want:        "player",
		},
		{
			name:        "long display name clamped to 20",
			displayName: "AVeryLongDisplayNameIndeedYes",
			email:       "long@example.com",
			want:        "AVeryLongDisplayName",
		},
		{
			name:        "collision gets numeric suffix",
			displayName: "Popular Name",
			email:       "popular@example.com",
			existing:    []string{"PopularName"},
			want:        "PopularName1",
		},
		{
			name:        "second collision increments the suffix",
			displayName: "Popular Name",
			email:       "popular@example.com",
			existing:    []string{"PopularName", "PopularName1"},
			want:        "PopularName2",
		},
		{
			name:        "suffix stays within the 20-char clamp",
			displayName: "ExactlyTwentyCharsAB",
			email:       "clamp@example.com",
			existing:    []string{"ExactlyTwentyCharsAB"},
			want:        "ExactlyTwentyCharsA1",
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, ur, _, fp := setupSSOHandler()
			for j, taken := range tc.existing {
				require.NoError(t, ur.Create(&user.User{
					Email:    "taken" + uniqSuffix(i, j) + "@example.com",
					Username: taken,
				}))
			}
			fp.add("cred-1", verifiedIdentity("sub-1", tc.email, tc.displayName))

			rec := doSSOLogin(e, "fakeprov", `{"credential":"cred-1"}`)
			require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
			data := decodeAuthData(t, rec)
			assert.Equal(t, tc.want, data.Username)
		})
	}
}

// uniqSuffix builds a unique email fragment for pre-seeded users in the
// username-generation table test.
func uniqSuffix(i, j int) string {
	const digits = "0123456789"
	return string(digits[i%10]) + string(digits[j%10])
}
