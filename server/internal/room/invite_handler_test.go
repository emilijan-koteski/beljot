package room_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/ws"
)

// Story 11.5 (FR62) — friend room invites. Sibling of privacy_handler_test.go /
// honor_handler_test.go, reusing the same mockRoomRepo harness. The load-bearing
// assertions here are the NEGATIVE ones: a host grant must bypass the password
// and NOTHING else (D2/AC6).

// --- stubs -----------------------------------------------------------------

// stubFriendDirectory implements room.FriendDirectory. friends is keyed by the
// viewer; the relation is made symmetric on read so a test only declares it once.
type stubFriendDirectory struct {
	friends map[uint][]uint
	names   map[uint]string
}

func (s *stubFriendDirectory) AreFriends(a, b uint) (bool, error) {
	for _, id := range s.friends[a] {
		if id == b {
			return true, nil
		}
	}
	for _, id := range s.friends[b] {
		if id == a {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubFriendDirectory) ListFriends(userID uint) ([]room.FriendSummary, error) {
	out := make([]room.FriendSummary, 0, len(s.friends[userID]))
	for _, id := range s.friends[userID] {
		out = append(out, room.FriendSummary{UserID: id, Username: s.names[id]})
	}
	return out, nil
}

// stubPresence implements both room.ConnectionTracker and room.SessionTracker.
// The third leg of the presence trio (in-room) is not stubbed — it is read from
// the real mockRoomRepo, so the tests exercise the same FindPlayerRoom predicate
// JoinRoom's already-in-room gate uses.
type stubPresence struct {
	online  map[uint]bool
	inMatch map[uint]bool
}

func (s *stubPresence) IsConnected(userID uint) bool   { return s.online[userID] }
func (s *stubPresence) IsUserInMatch(userID uint) bool { return s.inMatch[userID] }

type sentPush struct {
	userID uint
	msg    []byte
}

// spyNotifier implements room.InviteNotifier and records every per-user push.
type spyNotifier struct {
	sent []sentPush
}

func (s *spyNotifier) SendToUser(userID uint, msg []byte) {
	s.sent = append(s.sent, sentPush{userID: userID, msg: msg})
}

// pushesOfType returns the decoded payloads of every recorded push of one type.
func (s *spyNotifier) pushesOfType(t *testing.T, msgType string) []ws.RoomInvitePayload {
	t.Helper()
	var out []ws.RoomInvitePayload
	for _, p := range s.sent {
		var env ws.WSMessage
		require.NoError(t, json.Unmarshal(p.msg, &env))
		if env.Type != msgType {
			continue
		}
		var payload ws.RoomInvitePayload
		require.NoError(t, json.Unmarshal(env.Payload, &payload))
		out = append(out, payload)
	}
	return out
}

// --- harness ---------------------------------------------------------------

type inviteHarness struct {
	e        *echo.Echo
	repo     *mockRoomRepo
	invites  *room.InviteRegistry
	friends  *stubFriendDirectory
	presence *stubPresence
	notifier *spyNotifier
}

// setupInviteTest wires the room handler AND the invite handler over ONE shared
// invite registry — the whole point of the feature is that grants issued by the
// second are consumed by the first, so a test harness that split them would
// prove nothing.
func setupInviteTest(honor room.HonorService) *inviteHarness {
	repo := newMockRoomRepo()
	invites := room.NewInviteRegistry()
	friends := &stubFriendDirectory{friends: map[uint][]uint{}, names: map[uint]string{}}
	presence := &stubPresence{online: map[uint]bool{}, inMatch: map[uint]bool{}}
	notifier := &spyNotifier{}

	roomHandler := room.NewRoomHandler(repo, nil, &mockBroadcaster{}, room.NewPresenceRegistry(), nil, honor, invites)
	inviteHandler := room.NewInviteHandler(repo, invites, friends, presence, presence, notifier)

	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware("test-jwt-secret"))
	registerRoomRoutes(api, roomHandler)
	api.GET("/rooms/:id/invitable-friends", inviteHandler.ListInvitableFriends)
	api.POST("/rooms/:id/invite", inviteHandler.InviteToRoom)
	api.POST("/rooms/:id/invite/decline", inviteHandler.DeclineInvite)

	return &inviteHarness{e: e, repo: repo, invites: invites, friends: friends, presence: presence, notifier: notifier}
}

// befriend declares a symmetric friendship and registers both usernames.
func (h *inviteHarness) befriend(a uint, aName string, b uint, bName string) {
	h.friends.friends[a] = append(h.friends.friends[a], b)
	h.friends.friends[b] = append(h.friends.friends[b], a)
	h.friends.names[a] = aName
	h.friends.names[b] = bName
}

// available marks a user online, not in a match (and, by not seating them
// anywhere, not in a room) — the full presence trio for "invitable".
func (h *inviteHarness) available(userIDs ...uint) {
	for _, id := range userIDs {
		h.presence.online[id] = true
	}
}

// seedInviteRoom inserts a waiting room owned by ownerID with the owner seated,
// optionally private (password != "") and optionally honor-gated.
func seedInviteRoom(t *testing.T, repo *mockRoomRepo, ownerID uint, password string, minHonor int, allowNewPlayers bool) *room.Room {
	t.Helper()
	r := &room.Room{
		Name:            "Invite Table",
		Code:            "INVT01",
		OwnerID:         ownerID,
		Variant:         "bitola",
		MatchMode:       "1001",
		TimerStyle:      "relaxed",
		Status:          "waiting",
		PlayerCount:     1,
		CoinBuyIn:       0,
		MinHonor:        minHonor,
		AllowNewPlayers: allowNewPlayers,
	}
	if password != "" {
		hash, err := auth.HashPassword(password)
		require.NoError(t, err)
		r.PasswordHash = &hash
	}
	r.ID = repo.nextID
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	repo.nextID++
	repo.rooms = append(repo.rooms, r)

	seat := 0
	team := teamNameForSeat(0)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{
		RoomID: r.ID, UserID: ownerID, Username: "owner", Seat: &seat, Team: &team,
	}))
	return r
}

func doInvite(e *echo.Echo, roomID string, token string, body string) *httptest.ResponseRecorder {
	return doPostJSON(e, "/api/v1/rooms/"+roomID+"/invite", token, body)
}

func doListInvitable(e *echo.Echo, roomID string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rooms/"+roomID+"/invitable-friends", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, raw []byte) string {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &resp))
	return resp.Error.Code
}

// --- InviteToRoom (AC1, AC2) ------------------------------------------------

func TestInviteToRoom_OwnerInviteIssuesHostGrantAndPushes(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	h.befriend(100, "owner", 200, "buddy")
	h.available(200)

	rec := doInvite(h.e, "1", validToken(100), `{"friendUserId":200}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, h.invites.Pending(1, 200), "the invite is outstanding")

	pushes := h.notifier.pushesOfType(t, ws.SystemRoomInvite)
	require.Len(t, pushes, 1)
	assert.Equal(t, uint(1), pushes[0].RoomID)
	assert.Equal(t, "Invite Table", pushes[0].RoomName)
	assert.Equal(t, uint(100), pushes[0].InviterUserID)
	assert.Equal(t, "owner", pushes[0].InviterUsername)
	assert.True(t, pushes[0].IsPrivate, "a password-protected room is advertised as private")
	assert.True(t, pushes[0].IsHostInvite, "the OWNER's invite is a host invite")
	assert.NotEmpty(t, pushes[0].ExpiresAt, "the popup gets an absolute expiry, never a duration")
	assert.Equal(t, uint(200), h.notifier.sent[0].userID, "the push goes to the invitee only")
}

func TestInviteToRoom_MemberInviteIssuesNoHostGrant(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	require.NoError(t, h.repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 150, Username: "member"}))
	h.befriend(150, "member", 200, "buddy")
	h.available(200)

	rec := doInvite(h.e, "1", validToken(150), `{"friendUserId":200}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	pushes := h.notifier.pushesOfType(t, ws.SystemRoomInvite)
	require.Len(t, pushes, 1)
	assert.False(t, pushes[0].IsHostInvite, "a non-owner member's invite is NOT a host invite")

	assert.True(t, h.invites.Pending(1, 200), "the invite exists")
	assert.False(t, h.invites.ConsumeHostGrant(1, 200), "but it carries no password bypass")
}

func TestInviteToRoom_CallerNotInRoomIsRejected(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "", 0, true)
	h.befriend(999, "outsider", 200, "buddy")
	h.available(200)

	rec := doInvite(h.e, "1", validToken(999), `{"friendUserId":200}`)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "NOT_IN_ROOM", errorCode(t, rec.Body.Bytes()))
	assert.False(t, h.invites.Pending(1, 200))
	assert.Empty(t, h.notifier.sent, "a rejected invite pushes nothing")
}

func TestInviteToRoom_NonWaitingRoomIsRejected(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "", 0, true)
	r.Status = "playing"
	h.befriend(100, "owner", 200, "buddy")
	h.available(200)

	rec := doInvite(h.e, "1", validToken(100), `{"friendUserId":200}`)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "ROOM_NOT_FOUND", errorCode(t, rec.Body.Bytes()))
}

func TestInviteToRoom_NonFriendIsRejected(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "", 0, true)
	h.available(200)

	rec := doInvite(h.e, "1", validToken(100), `{"friendUserId":200}`)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "NOT_FRIENDS", errorCode(t, rec.Body.Bytes()))
	assert.False(t, h.invites.Pending(1, 200))
}

func TestInviteToRoom_SelfInviteIsRejected(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "", 0, true)

	rec := doInvite(h.e, "1", validToken(100), `{"friendUserId":100}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// The presence trio, driven through the real endpoint. Availability is ALWAYS
// recomputed server-side — the client never gets to assert it.
func TestInviteToRoom_UnavailableFriendIsRejected(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *inviteHarness)
	}{
		{
			name:  "offline friend",
			setup: func(h *inviteHarness) { h.presence.online[200] = false },
		},
		{
			name: "friend already in a match",
			setup: func(h *inviteHarness) {
				h.available(200)
				h.presence.inMatch[200] = true
			},
		},
		{
			name: "friend already in another room",
			setup: func(h *inviteHarness) {
				h.available(200)
				other := seedInviteRoom(t, h.repo, 300, "", 0, true)
				other.Name = "Other Table"
				other.Code = "OTHER1"
				require.NoError(t, h.repo.AddPlayer(&room.RoomPlayer{
					RoomID: other.ID, UserID: 200, Username: "buddy",
				}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := setupInviteTest(nil)
			seedInviteRoom(t, h.repo, 100, "", 0, true)
			h.befriend(100, "owner", 200, "buddy")
			tt.setup(h)

			rec := doInvite(h.e, "1", validToken(100), `{"friendUserId":200}`)

			assert.Equal(t, http.StatusConflict, rec.Code)
			assert.Equal(t, "FRIEND_NOT_AVAILABLE", errorCode(t, rec.Body.Bytes()))
			assert.False(t, h.invites.Pending(1, 200), "no grant is issued for an unavailable friend")
			assert.Empty(t, h.notifier.sent)
		})
	}
}

func TestInviteToRoom_FullRoomIsRejected(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "", 0, true)
	r.PlayerCount = 2
	require.NoError(t, h.repo.AddBot(r.ID, 1))
	require.NoError(t, h.repo.AddBot(r.ID, 2))
	require.NoError(t, h.repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 150, Username: "member"}))
	h.befriend(100, "owner", 200, "buddy")
	h.available(200)

	rec := doInvite(h.e, "1", validToken(100), `{"friendUserId":200}`)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ROOM_FULL", errorCode(t, rec.Body.Bytes()))
	assert.False(t, h.invites.Pending(1, 200))
}

// --- ListInvitableFriends (AC1) --------------------------------------------

func TestListInvitableFriends_AnnotatesAvailability(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "", 0, true)

	h.befriend(100, "owner", 200, "free")
	h.befriend(100, "owner", 201, "offline")
	h.befriend(100, "owner", 202, "playing")
	h.befriend(100, "owner", 203, "seated")

	h.available(200, 202, 203)
	h.presence.inMatch[202] = true
	other := seedInviteRoom(t, h.repo, 300, "", 0, true)
	other.Name = "Other Table"
	other.Code = "OTHER1"
	require.NoError(t, h.repo.AddPlayer(&room.RoomPlayer{RoomID: other.ID, UserID: 203, Username: "seated"}))

	rec := doListInvitable(h.e, "1", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []room.InvitableFriendDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 4, "unavailable friends are listed too, just disabled")

	byID := map[uint]room.InvitableFriendDTO{}
	for _, f := range resp.Data {
		byID[f.UserID] = f
	}
	assert.True(t, byID[200].Available)
	assert.Empty(t, byID[200].Reason)
	assert.False(t, byID[201].Available)
	assert.Equal(t, "offline", byID[201].Reason)
	assert.False(t, byID[202].Available)
	assert.Equal(t, "in_match", byID[202].Reason)
	assert.False(t, byID[203].Available)
	assert.Equal(t, "in_room", byID[203].Reason)
}

func TestListInvitableFriends_CallerNotInRoomIsRejected(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "", 0, true)

	rec := doListInvitable(h.e, "1", validToken(999))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "NOT_IN_ROOM", errorCode(t, rec.Body.Bytes()))
}

// --- JoinRoom under a grant (AC3-AC7) ---------------------------------------

// AC3: a host grant stands in for the password — and is spent doing so.
func TestJoinRoom_HostGrantBypassesPassword(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	h.invites.Issue(1, 200, 100, true)

	// No password sent at all.
	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")

	assert.Equal(t, http.StatusOK, rec.Code)
	players, _ := h.repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 2, "the invitee is seated")
	assert.False(t, h.invites.Pending(1, 200), "the grant is one-time and now spent")
}

// AC4: a NON-host invite grants nothing — the password gate still stands.
func TestJoinRoom_NonHostGrantStillRequiresPassword(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	h.invites.Issue(1, 200, 150, false)

	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "WRONG_ROOM_PASSWORD", errorCode(t, rec.Body.Bytes()))
	players, _ := h.repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 1, "no bypass, no seat")

	// The correct password still works for the same invitee.
	rec = doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), `{"password":"hunter2"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// The unchanged 9.6 behaviour: no grant at all, private room, no password.
func TestJoinRoom_NoGrantOnPrivateRoomStillRejects(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)

	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "WRONG_ROOM_PASSWORD", errorCode(t, rec.Body.Bytes()))
}

// AC6, the load-bearing negative: the grant bypasses the PASSWORD and nothing
// else. An honor-barred invitee is rejected and never seated.
func TestJoinRoom_HostGrantDoesNotBypassHonorGate(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(90),
		200: honorOf(40),
	}}
	h := setupInviteTest(honor)
	seedInviteRoom(t, h.repo, 100, "hunter2", 80, true)
	h.invites.Issue(1, 200, 100, true)

	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "HONOR_TOO_LOW", errorCode(t, rec.Body.Bytes()))
	players, _ := h.repo.FindPlayersByRoomID(1)
	assert.Len(t, players, 1, "an honor-barred invitee is NOT seated")
}

// AC6: the allow_new_players half of the gate is equally un-bypassable.
func TestJoinRoom_HostGrantDoesNotBypassNewPlayerGate(t *testing.T) {
	honor := &stubHonor{snaps: map[uint]user.HonorSnapshot{
		100: honorOf(90),
		200: newPlayerHonorOf(80),
	}}
	h := setupInviteTest(honor)
	seedInviteRoom(t, h.repo, 100, "hunter2", 0, false)
	h.invites.Issue(1, 200, 100, true)

	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "NEW_PLAYER_NOT_ALLOWED", errorCode(t, rec.Body.Bytes()))
}

// AC6/AC7: capacity still applies under a grant, and fails gracefully.
func TestJoinRoom_HostGrantDoesNotBypassCapacity(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	r.PlayerCount = 4
	h.invites.Issue(1, 200, 100, true)

	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ROOM_FULL", errorCode(t, rec.Body.Bytes()))
}

// AC6: bot-covered seats count under a grant exactly as they do without one.
func TestJoinRoom_HostGrantDoesNotBypassBotCapacity(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	require.NoError(t, h.repo.AddBot(r.ID, 1))
	require.NoError(t, h.repo.AddBot(r.ID, 2))
	require.NoError(t, h.repo.AddBot(r.ID, 3))
	h.invites.Issue(1, 200, 100, true)

	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ROOM_FULL", errorCode(t, rec.Body.Bytes()))
}

// AC7: a closed room fails gracefully — the invitee stays in the lobby.
func TestJoinRoom_HostGrantOnClosedRoomFailsGracefully(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	h.invites.Issue(1, 200, 100, true)
	r.Status = "completed"

	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "ROOM_NOT_FOUND", errorCode(t, rec.Body.Bytes()))
}

// AC2: a join that fills the room voids every invite still outstanding for it,
// so a seat freed later cannot be entered on a stale bypass.
func TestJoinRoom_FillingTheRoomVoidsOutstandingInvites(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	r.PlayerCount = 3
	h.invites.Issue(1, 200, 100, true) // the joiner
	h.invites.Issue(1, 201, 100, true) // a second invitee who never acted

	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")
	require.Equal(t, http.StatusOK, rec.Code)

	assert.False(t, h.invites.Pending(1, 201), "the room is full — the other invite is void")
	assert.False(t, h.invites.ConsumeHostGrant(1, 201))
}

// --- Code review 2026-08-16: patched behaviours ----------------------------

// The password gate PEEKS at the grant instead of consuming it, so a rejection
// from any LATER gate leaves the bypass intact for a retry. Before this, a
// transient ROOM_FULL burned the grant and the retry hit bcrypt with a password
// the invitee was never given — an unrecoverable lockout.
func TestJoinRoom_RejectedJoinDoesNotBurnTheHostGrant(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	r.PlayerCount = 4
	h.invites.Issue(1, 200, 100, true)

	// First attempt bounces off capacity, which sits AFTER the password block.
	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Equal(t, "ROOM_FULL", errorCode(t, rec.Body.Bytes()))

	// The grant must have survived: it was never the reason the join failed.
	assert.True(t, h.invites.HasHostGrant(1, 200), "a capacity rejection must not spend the grant")

	// A seat frees up and the same invite still works — no password supplied.
	r.PlayerCount = 3
	rec = doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// The flip side: a SUCCESSFUL join spends the grant, so it is genuinely one-time.
func TestJoinRoom_SuccessfulJoinSpendsTheHostGrant(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	h.invites.Issue(1, 200, 100, true)

	rec := doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")
	require.Equal(t, http.StatusOK, rec.Code)

	assert.False(t, h.invites.HasHostGrant(1, 200), "a completed join must consume the grant")
}

// A member's later invite must not strip the owner's password bypass.
func TestInviteRegistry_NonHostIssueNeverDowngradesALiveHostGrant(t *testing.T) {
	reg := room.NewInviteRegistry()
	reg.Issue(1, 200, 100, true)  // owner invites
	reg.Issue(1, 200, 300, false) // ordinary member invites the same friend

	assert.True(t, reg.HasHostGrant(1, 200), "the stronger host grant must survive a non-host re-issue")
}

// VoidInviter kills only the departing player's grants, not everyone's.
func TestInviteRegistry_VoidInviterIsScopedToThatInviter(t *testing.T) {
	reg := room.NewInviteRegistry()
	reg.Issue(1, 200, 100, true)
	reg.Issue(1, 201, 300, true)

	reg.VoidInviter(1, 100)

	assert.False(t, reg.HasHostGrant(1, 200), "the departing inviter's grant is gone")
	assert.True(t, reg.HasHostGrant(1, 201), "another inviter's grant is untouched")
}

// An ex-owner's grant must stop working the moment ownership moves.
func TestTransferOwnership_VoidsThePreviousOwnersGrants(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	seat := 1
	team := teamNameForSeat(1)
	require.NoError(t, h.repo.AddPlayer(&room.RoomPlayer{
		RoomID: r.ID, UserID: 300, Username: "newowner", Seat: &seat, Team: &team,
	}))
	r.PlayerCount = 2
	h.invites.Issue(1, 200, 100, true)

	rec := doPostJSON(h.e, "/api/v1/rooms/1/transfer-ownership", validToken(100), `{"userId":300}`)
	require.Equal(t, http.StatusOK, rec.Code)

	assert.False(t, h.invites.HasHostGrant(1, 200),
		"the former owner's host grant must not bypass the NEW owner's password")
}

// The throttle: one live invite per (room, friend).
func TestInviteToRoom_RejectsASecondInviteWhileOneIsPending(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "", 0, true)
	h.befriend(100, "owner", 200, "ana")
	h.available(200)

	rec := doInvite(h.e, "1", validToken(100), `{"friendUserId":200}`)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = doInvite(h.e, "1", validToken(100), `{"friendUserId":200}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "INVITE_ALREADY_PENDING", errorCode(t, rec.Body.Bytes()))
	assert.Len(t, h.notifier.pushesOfType(t, ws.SystemRoomInvite), 1, "no second popup is pushed")
}

// Quick-play rooms are matchmaking brackets — JoinRoom has no bracket check, so
// an invite would seat a friend in a bracket they never qualified for.
func TestInviteToRoom_RejectsQuickPlayRooms(t *testing.T) {
	h := setupInviteTest(nil)
	r := seedInviteRoom(t, h.repo, 100, "", 0, true)
	r.IsQuickPlay = true
	h.befriend(100, "owner", 200, "ana")
	h.available(200)

	rec := doInvite(h.e, "1", validToken(100), `{"friendUserId":200}`)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "ROOM_NOT_FOUND", errorCode(t, rec.Body.Bytes()))
	assert.Empty(t, h.notifier.pushesOfType(t, ws.SystemRoomInvite))
}

// Declining voids the grant immediately rather than leaving it live for the TTL.
func TestDeclineInvite_VoidsTheGrant(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)
	h.invites.Issue(1, 200, 100, true)
	require.True(t, h.invites.HasHostGrant(1, 200))

	rec := doPostJSON(h.e, "/api/v1/rooms/1/invite/decline", validToken(200), "")
	require.Equal(t, http.StatusOK, rec.Code)

	assert.False(t, h.invites.HasHostGrant(1, 200))

	// And the declined invitee is now back on the normal password gate.
	rec = doPostJSON(h.e, "/api/v1/rooms/1/join", validToken(200), "")
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "WRONG_ROOM_PASSWORD", errorCode(t, rec.Body.Bytes()))
}

// Declining something that is already gone is a no-op, not an error.
func TestDeclineInvite_IsIdempotent(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "hunter2", 0, true)

	rec := doPostJSON(h.e, "/api/v1/rooms/1/invite/decline", validToken(200), "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// The payload carries the room's honor floor so the invitee gets the SPECIFIC
// "you need N honor" message on a rejected accept, like every other join path.
func TestInviteToRoom_PushCarriesTheRoomsHonorFloor(t *testing.T) {
	h := setupInviteTest(nil)
	seedInviteRoom(t, h.repo, 100, "", 70, true)
	h.befriend(100, "owner", 200, "ana")
	h.available(200)

	rec := doInvite(h.e, "1", validToken(100), `{"friendUserId":200}`)
	require.Equal(t, http.StatusOK, rec.Code)

	pushes := h.notifier.pushesOfType(t, ws.SystemRoomInvite)
	require.Len(t, pushes, 1)
	assert.Equal(t, 70, pushes[0].MinHonor)
}
