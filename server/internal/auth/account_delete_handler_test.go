package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/account"
	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/user"
)

type fakeEraser struct {
	seated    bool
	eraseErr  error
	avatarKey *string
	erased    []uint
}

func (f *fakeEraser) InActiveRoom(uint) (bool, error) { return f.seated, nil }

func (f *fakeEraser) Erase(userID uint) (account.Erased, error) {
	if f.eraseErr != nil {
		return account.Erased{}, f.eraseErr
	}
	f.erased = append(f.erased, userID)
	return account.Erased{AvatarKey: f.avatarKey}, nil
}

type fakeSessions struct{ closed []uint }

func (f *fakeSessions) DisconnectUser(userID uint) { f.closed = append(f.closed, userID) }

type fakeAvatarRemover struct{ prefixes []string }

func (f *fakeAvatarRemover) DeleteStored(_ context.Context, _ uint, prefix, _ string) {
	f.prefixes = append(f.prefixes, prefix)
}

type deletionFixture struct {
	e        *echo.Echo
	users    *mockUserRepo
	eraser   *fakeEraser
	sessions *fakeSessions
	avatars  *fakeAvatarRemover
	revoked  *RevokedUsers
}

func setupDeletion(t *testing.T) deletionFixture {
	t.Helper()
	f := deletionFixture{
		users:    newMockUserRepo(),
		eraser:   &fakeEraser{},
		sessions: &fakeSessions{},
		avatars:  &fakeAvatarRemover{},
		revoked:  NewRevokedUsers(time.Minute),
	}
	h := NewAccountDeletionHandler(f.users, f.eraser, f.sessions, f.avatars, f.revoked, "development")
	f.e = echo.New()
	f.e.HTTPErrorHandler = testErrorHandler
	api := f.e.Group("/api/v1", AuthMiddlewareWithRevocations("test-jwt-secret", f.revoked))
	api.DELETE("/users/:id", h.DeleteAccount)
	api.GET("/ping", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	return f
}

func seedPasswordUser(t *testing.T, ur *mockUserRepo, password string) *user.User {
	t.Helper()
	hash, err := HashPassword(password)
	require.NoError(t, err)
	u := &user.User{Email: "pw@example.com", Username: "pw_player", PasswordHash: hash}
	require.NoError(t, ur.Create(u))
	return u
}

func seedGoogleOnlyUser(t *testing.T, ur *mockUserRepo) *user.User {
	t.Helper()
	u := &user.User{Email: "g@example.com", Username: "player_ab12cd", PasswordHash: ""}
	require.NoError(t, ur.Create(u))
	return u
}

func refreshCookieCleared(rec interface{ Result() *http.Response }) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

func TestDeleteAccount_ConfirmsAndEndsEverySession(t *testing.T) {
	tests := []struct {
		name string
		seed func(*testing.T, *mockUserRepo) *user.User
		body string
	}{
		{
			name: "password account confirms with its password",
			seed: func(t *testing.T, ur *mockUserRepo) *user.User { return seedPasswordUser(t, ur, "password123") },
			body: `{"password":"password123"}`,
		},
		{
			name: "google-only account confirms with its username",
			seed: seedGoogleOnlyUser,
			body: `{"username":" player_ab12cd "}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := setupDeletion(t)
			u := tc.seed(t, f.users)
			key := "avatars/abc"
			f.eraser.avatarKey = &key

			rec := doAuthedReq(f.e, http.MethodDelete, "/api/v1/users/1", u.ID, tc.body)
			require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

			assert.Equal(t, []uint{u.ID}, f.eraser.erased)
			assert.Equal(t, []uint{u.ID}, f.sessions.closed)
			assert.Equal(t, []string{key}, f.avatars.prefixes)
			assert.True(t, refreshCookieCleared(rec), "the refresh cookie must be expired")
			assert.True(t, f.revoked.IsRevoked(u.ID))

			after := doAuthedReq(f.e, http.MethodGet, "/api/v1/ping", u.ID, "")
			assert.Equal(t, http.StatusUnauthorized, after.Code, "the old access token must stop working")
		})
	}
}

func TestDeleteAccount_RejectsWrongConfirmation(t *testing.T) {
	tests := []struct {
		name string
		seed func(*testing.T, *mockUserRepo) *user.User
		body string
	}{
		{
			name: "wrong password",
			seed: func(t *testing.T, ur *mockUserRepo) *user.User { return seedPasswordUser(t, ur, "password123") },
			body: `{"password":"nope-nope"}`,
		},
		{
			name: "password account cannot confirm with its username",
			seed: func(t *testing.T, ur *mockUserRepo) *user.User { return seedPasswordUser(t, ur, "password123") },
			body: `{"username":"pw_player"}`,
		},
		{
			name: "google-only account with another username",
			seed: seedGoogleOnlyUser,
			body: `{"username":"player_zzzzzz"}`,
		},
		{
			name: "google-only account with an empty body",
			seed: seedGoogleOnlyUser,
			body: `{}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := setupDeletion(t)
			u := tc.seed(t, f.users)

			rec := doAuthedReq(f.e, http.MethodDelete, "/api/v1/users/1", u.ID, tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, apperr.ErrAccountDeleteConfirmation.Code, decodeErrorCode(t, rec))
			assert.Empty(t, f.eraser.erased)
			assert.Empty(t, f.sessions.closed)
			assert.False(t, f.revoked.IsRevoked(u.ID))
		})
	}
}

func TestDeleteAccount_RejectedWhileSeatedInARoom(t *testing.T) {
	f := setupDeletion(t)
	u := seedPasswordUser(t, f.users, "password123")
	f.eraser.seated = true

	rec := doAuthedReq(f.e, http.MethodDelete, "/api/v1/users/1", u.ID, `{"password":"password123"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, apperr.ErrAccountDeleteInRoom.Code, decodeErrorCode(t, rec))
	assert.Empty(t, f.eraser.erased)
}

func TestDeleteAccount_RoomJoinedDuringEraseIsReported(t *testing.T) {
	f := setupDeletion(t)
	u := seedPasswordUser(t, f.users, "password123")
	f.eraser.eraseErr = apperr.ErrAccountDeleteInRoom

	rec := doAuthedReq(f.e, http.MethodDelete, "/api/v1/users/1", u.ID, `{"password":"password123"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Empty(t, f.sessions.closed)
	assert.False(t, f.revoked.IsRevoked(u.ID))
}

func TestDeleteAccount_EraseFailureKeepsTheSession(t *testing.T) {
	f := setupDeletion(t)
	u := seedPasswordUser(t, f.users, "password123")
	f.eraser.eraseErr = errors.New("db down")

	rec := doAuthedReq(f.e, http.MethodDelete, "/api/v1/users/1", u.ID, `{"password":"password123"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Empty(t, f.sessions.closed)
	assert.False(t, refreshCookieCleared(rec))
	assert.False(t, f.revoked.IsRevoked(u.ID))
}

func TestDeleteAccount_OnlyTheOwnAccount(t *testing.T) {
	f := setupDeletion(t)
	seedPasswordUser(t, f.users, "password123")
	other := seedGoogleOnlyUser(t, f.users)

	rec := doAuthedReq(f.e, http.MethodDelete, "/api/v1/users/1", other.ID, `{"username":"player_ab12cd"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, f.eraser.erased)
}

func TestDeleteAccount_NoAvatarSkipsStorage(t *testing.T) {
	f := setupDeletion(t)
	u := seedPasswordUser(t, f.users, "password123")

	rec := doAuthedReq(f.e, http.MethodDelete, "/api/v1/users/1", u.ID, `{"password":"password123"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, f.avatars.prefixes)
}

func TestRevokedUsers_ExpireAfterTTL(t *testing.T) {
	r := NewRevokedUsers(time.Minute)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }
	r.Revoke(7)
	assert.True(t, r.IsRevoked(7))
	assert.False(t, r.IsRevoked(8))

	now = now.Add(2 * time.Minute)
	assert.False(t, r.IsRevoked(7))

	var none *RevokedUsers
	assert.False(t, none.IsRevoked(7))
}
