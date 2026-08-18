package friend_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/friend"
	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/ws"
)

// --- Mock friend Repository (in-memory, mirrors the GORM semantics) ---

type mockFriendRepo struct {
	rows      []*friend.Friendship
	nextID    uint
	createErr error
}

func newMockFriendRepo() *mockFriendRepo { return &mockFriendRepo{nextID: 1} }

// seed inserts a row directly (bypassing the duplicate check) so tests can set
// up any relationship state.
func (m *mockFriendRepo) seed(userID, friendID uint, status string) *friend.Friendship {
	f := &friend.Friendship{
		ID:        m.nextID,
		UserID:    userID,
		FriendID:  friendID,
		Status:    status,
		CreatedAt: time.Now().UTC(),
	}
	m.nextID++
	m.rows = append(m.rows, f)
	return f
}

func (m *mockFriendRepo) Create(f *friend.Friendship) error {
	if m.createErr != nil {
		return m.createErr
	}
	// Mirror the exact-direction unique index only (the reverse dup is the
	// handler's job to catch via FindByPair, so Create must NOT block it).
	for _, r := range m.rows {
		if r.UserID == f.UserID && r.FriendID == f.FriendID {
			return apperr.ErrFriendRequestExists
		}
	}
	f.ID = m.nextID
	m.nextID++
	now := time.Now().UTC()
	f.CreatedAt = now
	f.UpdatedAt = now
	stored := *f
	m.rows = append(m.rows, &stored)
	return nil
}

func (m *mockFriendRepo) FindByID(id uint) (*friend.Friendship, error) {
	for _, r := range m.rows {
		if r.ID == id {
			cp := *r
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockFriendRepo) FindByPair(a, b uint) (*friend.Friendship, error) {
	for _, r := range m.rows {
		if (r.UserID == a && r.FriendID == b) || (r.UserID == b && r.FriendID == a) {
			cp := *r
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockFriendRepo) Accept(id, recipientID uint) (int64, error) {
	for _, r := range m.rows {
		if r.ID == id && r.FriendID == recipientID && r.Status == friend.FriendStatusPending {
			r.Status = friend.FriendStatusAccepted
			return 1, nil
		}
	}
	return 0, nil
}

func (m *mockFriendRepo) Delete(id, recipientID uint) (int64, error) {
	for i, r := range m.rows {
		if r.ID == id && r.FriendID == recipientID && r.Status == friend.FriendStatusPending {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			return 1, nil
		}
	}
	return 0, nil
}

func (m *mockFriendRepo) Unfriend(id, userID uint) (int64, error) {
	for i, r := range m.rows {
		if r.ID == id && (r.UserID == userID || r.FriendID == userID) && r.Status == friend.FriendStatusAccepted {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			return 1, nil
		}
	}
	return 0, nil
}

func (m *mockFriendRepo) ListAccepted(userID uint) ([]friend.Friendship, error) {
	out := []friend.Friendship{}
	for _, r := range m.rows {
		if r.Status == friend.FriendStatusAccepted && (r.UserID == userID || r.FriendID == userID) {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (m *mockFriendRepo) ListIncomingPending(userID uint) ([]friend.Friendship, error) {
	out := []friend.Friendship{}
	for _, r := range m.rows {
		if r.Status == friend.FriendStatusPending && r.FriendID == userID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (m *mockFriendRepo) AreFriends(a, b uint) (bool, error) {
	for _, r := range m.rows {
		if r.Status == friend.FriendStatusAccepted &&
			((r.UserID == a && r.FriendID == b) || (r.UserID == b && r.FriendID == a)) {
			return true, nil
		}
	}
	return false, nil
}

// --- Notifier spy ---

type sentMsg struct {
	userID uint
	msg    []byte
}

type notifierSpy struct {
	connected map[uint]bool
	sent      []sentMsg
}

func newNotifierSpy() *notifierSpy { return &notifierSpy{connected: map[uint]bool{}} }

func (s *notifierSpy) IsConnected(userID uint) bool { return s.connected[userID] }

func (s *notifierSpy) SendToUser(userID uint, msg []byte) {
	s.sent = append(s.sent, sentMsg{userID: userID, msg: msg})
}

// --- Stub user.UserRepository (only FindByID / FindManyByIDs matter here) ---

type stubUserRepo struct {
	users map[uint]*user.User
}

func newStubUserRepo() *stubUserRepo { return &stubUserRepo{users: map[uint]*user.User{}} }

func (s *stubUserRepo) add(id uint, username string) *user.User {
	u := &user.User{ID: id, Username: username, Email: username + "@f.test", LanguagePreference: "en"}
	s.users[id] = u
	return u
}

func (s *stubUserRepo) FindByID(id uint) (*user.User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, nil
}

func (s *stubUserRepo) FindManyByIDs(ids []uint) ([]user.User, error) {
	out := []user.User{}
	for _, id := range ids {
		if u, ok := s.users[id]; ok {
			out = append(out, *u)
		}
	}
	return out, nil
}

// The remaining UserRepository methods are unused by the friend handler.
func (s *stubUserRepo) Create(*user.User) error                   { return nil }
func (s *stubUserRepo) Delete(uint) error                         { return nil }
func (s *stubUserRepo) FindByEmail(string) (*user.User, error)    { return nil, nil }
func (s *stubUserRepo) FindByUsername(string) (*user.User, error) { return nil, nil }
func (s *stubUserRepo) SearchByUsername(string, uint, int) ([]user.User, error) {
	return nil, nil
}
func (s *stubUserRepo) Count() (int64, error)                       { return int64(len(s.users)), nil }
func (s *stubUserRepo) UpdateLanguagePreference(uint, string) error { return nil }
func (s *stubUserRepo) UpdatePasswordHash(uint, string) error       { return nil }
func (s *stubUserRepo) UpdateUsername(uint, string) (time.Time, error) {
	return time.Time{}, nil
}
func (s *stubUserRepo) AddXP(map[uint]int) (map[uint]int, error) { return nil, nil }
func (s *stubUserRepo) TotalXPForUsers([]uint) (map[uint]int, error) {
	return nil, nil
}
func (s *stubUserRepo) ApplyHonorEvents(map[uint]user.HonorEvent, time.Time) (map[uint]user.HonorSnapshot, error) {
	return nil, nil
}
func (s *stubUserRepo) ResetHonor(uint) error { return nil }

// --- Test harness ---

func testErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		_ = c.JSON(appErr.Status, map[string]interface{}{
			"error": map[string]string{"code": appErr.Code, "message": appErr.Message},
		})
		return
	}
	_ = c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"error": map[string]string{"code": "INTERNAL_ERROR", "message": "An internal error occurred"},
	})
}

const testJWTSecret = "test-jwt-secret"

func setup() (*mockFriendRepo, *stubUserRepo, *notifierSpy, *echo.Echo) {
	repo := newMockFriendRepo()
	users := newStubUserRepo()
	spy := newNotifierSpy()
	h := friend.NewHandler(repo, users, spy)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware(testJWTSecret))
	api.POST("/friends/request", h.SendRequest)
	api.GET("/friends", h.ListFriends)
	api.GET("/friends/requests", h.ListRequests)
	api.GET("/friends/status/:id", h.GetStatus)
	api.POST("/friends/:id/accept", h.Accept)
	api.POST("/friends/:id/decline", h.Decline)
	api.DELETE("/friends/:id", h.Unfriend)
	return repo, users, spy, e
}

func tokenFor(t *testing.T, id uint) string {
	t.Helper()
	tok, err := auth.GenerateAccessToken(id, testJWTSecret)
	require.NoError(t, err)
	return tok
}

func doPost(e *echo.Echo, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doGet(e *echo.Echo, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doDelete(e *echo.Echo, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func uid(id uint) string { return strconv.FormatUint(uint64(id), 10) }

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Error.Code
}

func statusResp(t *testing.T, rec *httptest.ResponseRecorder) friend.FriendStatusResponse {
	t.Helper()
	var resp struct {
		Data friend.FriendStatusResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data
}

// --- SendRequest ---

func TestSendRequest_Self(t *testing.T) {
	_, users, _, e := setup()
	users.add(1, "alice")
	rec := doPost(e, "/api/v1/friends/request", `{"userId":1}`, tokenFor(t, 1))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "SELF_FRIEND_REQUEST", errorCode(t, rec))
}

func TestSendRequest_UnknownTarget(t *testing.T) {
	_, users, _, e := setup()
	users.add(1, "alice")
	rec := doPost(e, "/api/v1/friends/request", `{"userId":999}`, tokenFor(t, 1))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "USER_NOT_FOUND", errorCode(t, rec))
}

func TestSendRequest_HappyPath_OnlineRecipientGetsPush(t *testing.T) {
	repo, users, spy, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	spy.connected[2] = true

	rec := doPost(e, "/api/v1/friends/request", `{"userId":2}`, tokenFor(t, 1))
	require.Equal(t, http.StatusCreated, rec.Code)

	require.Len(t, repo.rows, 1)
	assert.Equal(t, uint(1), repo.rows[0].UserID)
	assert.Equal(t, uint(2), repo.rows[0].FriendID)
	assert.Equal(t, friend.FriendStatusPending, repo.rows[0].Status)

	// The online recipient receives the best-effort system:friend_request push.
	require.Len(t, spy.sent, 1)
	assert.Equal(t, uint(2), spy.sent[0].userID)

	var msg ws.WSMessage
	require.NoError(t, json.Unmarshal(spy.sent[0].msg, &msg))
	assert.Equal(t, ws.SystemFriendRequest, msg.Type)

	var p ws.FriendRequestPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &p))
	assert.Equal(t, uint(1), p.FromUserID)
	assert.Equal(t, "alice", p.FromUsername)
	assert.Equal(t, repo.rows[0].ID, p.RequestID)
}

func TestSendRequest_OfflineRecipient_NoPushStill201(t *testing.T) {
	repo, users, spy, e := setup()
	users.add(1, "alice")
	users.add(2, "bob") // deliberately NOT connected

	rec := doPost(e, "/api/v1/friends/request", `{"userId":2}`, tokenFor(t, 1))
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Len(t, repo.rows, 1)
	assert.Empty(t, spy.sent, "an offline recipient gets no push, and the request still succeeds")
}

func TestSendRequest_PendingDuplicate_EitherDirection(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	repo.seed(1, 2, friend.FriendStatusPending)

	// Same direction again → exists.
	rec := doPost(e, "/api/v1/friends/request", `{"userId":2}`, tokenFor(t, 1))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "FRIEND_REQUEST_EXISTS", errorCode(t, rec))

	// Reverse direction (2 requests 1) → also exists, caught by FindByPair.
	rec = doPost(e, "/api/v1/friends/request", `{"userId":1}`, tokenFor(t, 2))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "FRIEND_REQUEST_EXISTS", errorCode(t, rec))
}

func TestSendRequest_AcceptedDuplicate(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	repo.seed(1, 2, friend.FriendStatusAccepted)

	rec := doPost(e, "/api/v1/friends/request", `{"userId":2}`, tokenFor(t, 1))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ALREADY_FRIENDS", errorCode(t, rec))

	// Reverse direction also reports already-friends.
	rec = doPost(e, "/api/v1/friends/request", `{"userId":1}`, tokenFor(t, 2))
	assert.Equal(t, "ALREADY_FRIENDS", errorCode(t, rec))
}

// --- GetStatus ---

func TestGetStatus_AllStates(t *testing.T) {
	repo, users, _, e := setup()
	for id, name := range map[uint]string{1: "alice", 2: "bob", 3: "carol", 4: "dave", 5: "erin"} {
		users.add(id, name)
	}

	// none — no relationship.
	assert.Equal(t, "none", statusResp(t, doGet(e, "/api/v1/friends/status/2", tokenFor(t, 1))).Status)

	// self → none (button never shown on your own page).
	assert.Equal(t, "none", statusResp(t, doGet(e, "/api/v1/friends/status/1", tokenFor(t, 1))).Status)

	// pending_outgoing — viewer 1 sent it.
	f := repo.seed(1, 3, friend.FriendStatusPending)
	out := statusResp(t, doGet(e, "/api/v1/friends/status/3", tokenFor(t, 1)))
	assert.Equal(t, "pending_outgoing", out.Status)
	require.NotNil(t, out.RequestID)
	assert.Equal(t, f.ID, *out.RequestID)

	// pending_incoming — subject 4 sent it to viewer 1.
	repo.seed(4, 1, friend.FriendStatusPending)
	assert.Equal(t, "pending_incoming", statusResp(t, doGet(e, "/api/v1/friends/status/4", tokenFor(t, 1))).Status)

	// friends — accepted.
	repo.seed(1, 5, friend.FriendStatusAccepted)
	assert.Equal(t, "friends", statusResp(t, doGet(e, "/api/v1/friends/status/5", tokenFor(t, 1))).Status)
}

// --- Accept / Decline ---

func TestAccept_RecipientOnly(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	f := repo.seed(1, 2, friend.FriendStatusPending)

	// The requester (1) cannot accept → uniform 404.
	rec := doPost(e, "/api/v1/friends/"+uid(f.ID)+"/accept", "", tokenFor(t, 1))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "FRIEND_REQUEST_NOT_FOUND", errorCode(t, rec))

	// The recipient (2) accepts → 200 and the row is accepted.
	rec = doPost(e, "/api/v1/friends/"+uid(f.ID)+"/accept", "", tokenFor(t, 2))
	assert.Equal(t, http.StatusOK, rec.Code)
	got, _ := repo.FindByID(f.ID)
	require.NotNil(t, got)
	assert.Equal(t, friend.FriendStatusAccepted, got.Status)
}

func TestDecline_RecipientOnly(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	f := repo.seed(1, 2, friend.FriendStatusPending)

	// The requester (1) cannot decline → uniform 404.
	rec := doPost(e, "/api/v1/friends/"+uid(f.ID)+"/decline", "", tokenFor(t, 1))
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// The recipient (2) declines → 200 and the row is gone.
	rec = doPost(e, "/api/v1/friends/"+uid(f.ID)+"/decline", "", tokenFor(t, 2))
	assert.Equal(t, http.StatusOK, rec.Code)
	got, _ := repo.FindByID(f.ID)
	assert.Nil(t, got)
}

// --- Unfriend ---

func TestUnfriend_EitherParty(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")

	// The requester (1) unfriends → 200 and the row is gone.
	f := repo.seed(1, 2, friend.FriendStatusAccepted)
	rec := doDelete(e, "/api/v1/friends/"+uid(f.ID), tokenFor(t, 1))
	assert.Equal(t, http.StatusOK, rec.Code)
	got, _ := repo.FindByID(f.ID)
	assert.Nil(t, got)

	// The recipient (2) unfriends a fresh row just the same — the guard is
	// party-agnostic, unlike Accept/Decline.
	f = repo.seed(1, 2, friend.FriendStatusAccepted)
	rec = doDelete(e, "/api/v1/friends/"+uid(f.ID), tokenFor(t, 2))
	assert.Equal(t, http.StatusOK, rec.Code)
	got, _ = repo.FindByID(f.ID)
	assert.Nil(t, got)
}

func TestUnfriend_RepeatCall404(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	f := repo.seed(1, 2, friend.FriendStatusAccepted)

	rec := doDelete(e, "/api/v1/friends/"+uid(f.ID), tokenFor(t, 1))
	require.Equal(t, http.StatusOK, rec.Code)

	// The second delete (the both-unfriend race, or a stale button) → uniform 404.
	rec = doDelete(e, "/api/v1/friends/"+uid(f.ID), tokenFor(t, 2))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "FRIENDSHIP_NOT_FOUND", errorCode(t, rec))
}

func TestUnfriend_PendingRow404(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	f := repo.seed(1, 2, friend.FriendStatusPending)

	// A pending row is decline's job, not unfriend's → uniform 404, row survives.
	rec := doDelete(e, "/api/v1/friends/"+uid(f.ID), tokenFor(t, 2))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "FRIENDSHIP_NOT_FOUND", errorCode(t, rec))
	got, _ := repo.FindByID(f.ID)
	require.NotNil(t, got)
}

func TestUnfriend_ThirdParty404(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	users.add(3, "carol")
	f := repo.seed(1, 2, friend.FriendStatusAccepted)

	// A third party (3) gets the same uniform 404 as a missing row — never a 403
	// that would leak the row's existence — and the row survives.
	rec := doDelete(e, "/api/v1/friends/"+uid(f.ID), tokenFor(t, 3))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "FRIENDSHIP_NOT_FOUND", errorCode(t, rec))
	got, _ := repo.FindByID(f.ID)
	require.NotNil(t, got)
}

func TestUnfriend_BadID(t *testing.T) {
	_, users, _, e := setup()
	users.add(1, "alice")

	// Non-numeric and zero ids are both a 400 BAD_REQUEST from parseIDParam.
	rec := doDelete(e, "/api/v1/friends/abc", tokenFor(t, 1))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "BAD_REQUEST", errorCode(t, rec))
	rec = doDelete(e, "/api/v1/friends/0", tokenFor(t, 1))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "BAD_REQUEST", errorCode(t, rec))
}

// --- Lists ---

func TestListRequests_IncomingWithUsernames(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	users.add(3, "carol")
	users.add(4, "dave")

	repo.seed(2, 1, friend.FriendStatusPending) // incoming
	repo.seed(3, 1, friend.FriendStatusPending) // incoming
	repo.seed(1, 4, friend.FriendStatusPending) // outgoing — excluded

	rec := doGet(e, "/api/v1/friends/requests", tokenFor(t, 1))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []friend.PendingRequestDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)

	names := map[string]bool{}
	for _, r := range resp.Data {
		names[r.FromUsername] = true
	}
	assert.True(t, names["bob"])
	assert.True(t, names["carol"])
}

func TestListRequests_EmptySerializesArray(t *testing.T) {
	_, users, _, e := setup()
	users.add(1, "alice")
	rec := doGet(e, "/api/v1/friends/requests", tokenFor(t, 1))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"data":[]`)
}

func TestListFriends_OnlineFlagAndSymmetry(t *testing.T) {
	repo, users, spy, e := setup()
	users.add(1, "alice")
	users.add(2, "bob")
	users.add(3, "carol")

	repo.seed(1, 2, friend.FriendStatusAccepted) // alice requester
	repo.seed(3, 1, friend.FriendStatusAccepted) // alice recipient
	spy.connected[2] = true                      // bob online, carol offline

	rec := doGet(e, "/api/v1/friends", tokenFor(t, 1))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []friend.FriendDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)

	online := map[uint]bool{}
	for _, f := range resp.Data {
		online[f.ID] = f.Online
	}
	assert.True(t, online[2], "bob is connected")
	assert.False(t, online[3], "carol is offline")
}

func TestListFriends_SoftDeletedOmitted(t *testing.T) {
	repo, users, _, e := setup()
	users.add(1, "alice")
	// A friendship to user 2 exists, but user 2 has no live row (soft-deleted →
	// absent from FindManyByIDs). It must be omitted, not 500.
	repo.seed(1, 2, friend.FriendStatusAccepted)

	rec := doGet(e, "/api/v1/friends", tokenFor(t, 1))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"data":[]`)
}

func TestListFriends_EmptySerializesArray(t *testing.T) {
	_, users, _, e := setup()
	users.add(1, "alice")
	rec := doGet(e, "/api/v1/friends", tokenFor(t, 1))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"data":[]`)
}

// --- Unauthenticated ---

func TestFriendEndpoints_RequireAuth(t *testing.T) {
	_, _, _, e := setup()
	rec := doGet(e, "/api/v1/friends", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	rec = doDelete(e, "/api/v1/friends/1", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
