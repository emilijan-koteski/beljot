package chat_test

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/chat"
	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/ws"
)

// --- Fakes ---

type userRepoStub struct {
	users map[uint]*user.User
}

func newUserRepoStub() *userRepoStub {
	return &userRepoStub{users: make(map[uint]*user.User)}
}

func (s *userRepoStub) add(id uint, username string) {
	s.users[id] = &user.User{ID: id, Username: username}
}

func (s *userRepoStub) Create(*user.User) error                     { return nil }
func (s *userRepoStub) Delete(uint) error                           { return nil }
func (s *userRepoStub) FindByEmail(string) (*user.User, error)      { return nil, nil }
func (s *userRepoStub) FindByUsername(string) (*user.User, error)   { return nil, nil }
func (s *userRepoStub) FindManyByIDs([]uint) ([]user.User, error)   { return nil, nil }
func (s *userRepoStub) Count() (int64, error)                       { return int64(len(s.users)), nil }
func (s *userRepoStub) UpdateLanguagePreference(uint, string) error { return nil }
func (s *userRepoStub) UpdatePasswordHash(uint, string) error       { return nil }
func (s *userRepoStub) AddXP(map[uint]int) (map[uint]int, error)    { return nil, nil }
func (s *userRepoStub) TotalXPForUsers([]uint) (map[uint]int, error) {
	return nil, nil
}
func (s *userRepoStub) FindByID(id uint) (*user.User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, nil
}

type fakeGame struct {
	inGame       map[uint]bool
	participants map[uint][4]uint
}

func newFakeGame() *fakeGame {
	return &fakeGame{
		inGame:       make(map[uint]bool),
		participants: make(map[uint][4]uint),
	}
}

func (g *fakeGame) IsUserInMatch(userID uint) bool {
	return g.inGame[userID]
}

func (g *fakeGame) MatchParticipants(roomID uint) ([4]uint, bool) {
	p, ok := g.participants[roomID]
	return p, ok
}

type fakeRoom struct {
	members map[uint][]uint // roomID → userIDs; absent means room unknown / not waiting
}

func newFakeRoom() *fakeRoom {
	return &fakeRoom{members: make(map[uint][]uint)}
}

func (r *fakeRoom) RoomMembers(roomID uint) ([]uint, bool) {
	m, ok := r.members[roomID]
	if !ok {
		return nil, false
	}
	out := make([]uint, len(m))
	copy(out, m)
	return out, true
}

type hubSpy struct {
	mu        sync.Mutex
	connected []uint
	calls     []hubCall
}

type hubCall struct {
	userIDs []uint
	msg     []byte
}

func (h *hubSpy) ConnectedUserIDs() []uint {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]uint, len(h.connected))
	copy(out, h.connected)
	return out
}

func (h *hubSpy) BroadcastToUsers(userIDs []uint, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	dup := make([]uint, len(userIDs))
	copy(dup, userIDs)
	dupMsg := make([]byte, len(msg))
	copy(dupMsg, msg)
	h.calls = append(h.calls, hubCall{userIDs: dup, msg: dupMsg})
}

func (h *hubSpy) lastCall(t *testing.T) hubCall {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	require.NotEmpty(t, h.calls, "expected at least one broadcast")
	return h.calls[len(h.calls)-1]
}

func (h *hubSpy) callCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.calls)
}

// --- Helpers ---

func chatActionMessage(t *testing.T, channel, text string) ws.WSMessage {
	t.Helper()
	return chatActionMessageRaw(t, ws.ChatMessageRequest{Channel: channel, Text: text})
}

func chatActionMessageRaw(t *testing.T, req interface{}) ws.WSMessage {
	t.Helper()
	payload, err := json.Marshal(req)
	require.NoError(t, err)
	return ws.WSMessage{Type: ws.ActionChatMessage, Payload: payload}
}

// --- Tests ---

func TestHandler_GlobalMessage_BroadcastsToEligibleClients(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")

	game := newFakeGame()
	hub := &hubSpy{connected: []uint{10, 20, 30}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	client := &ws.Client{UserID: 10}
	h.HandleAction(client, chatActionMessage(t, "lobby", "hello world"))

	require.Equal(t, 1, hub.callCount())
	call := hub.lastCall(t)
	assert.ElementsMatch(t, []uint{10, 20, 30}, call.userIDs,
		"sender included so they see their own message")

	var env ws.WSMessage
	require.NoError(t, json.Unmarshal(call.msg, &env))
	assert.Equal(t, ws.SystemChatMessage, env.Type)

	var payload ws.ChatMessagePayload
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, uint(10), payload.UserID)
	assert.Equal(t, "alice", payload.Username)
	assert.Equal(t, "hello world", payload.Message)
	assert.Equal(t, "lobby", payload.Scope)
	assert.NotEmpty(t, payload.Timestamp, "server stamps RFC3339 timestamp")
}

func TestHandler_GlobalMessage_ExcludesInGameUsers(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")

	game := newFakeGame()
	game.inGame[20] = true
	game.inGame[30] = true

	hub := &hubSpy{connected: []uint{10, 20, 30, 40}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "lobby", "hi"))

	require.Equal(t, 1, hub.callCount())
	call := hub.lastCall(t)
	assert.ElementsMatch(t, []uint{10, 40}, call.userIDs,
		"in-game users (20,30) excluded; idle users (10,40) included")
}

func TestHandler_GlobalMessage_DroppedWhenSenderInGame(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")

	game := newFakeGame()
	game.inGame[10] = true

	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "lobby", "hi"))

	assert.Equal(t, 0, hub.callCount(), "no broadcast when sender is in a game")
}

func TestHandler_GlobalMessage_SenderAlwaysReceivesOwnEcho(t *testing.T) {
	// Race repro: sender passes the initial in-game check, but then joins
	// a game before the per-recipient filter loop runs. The sender must
	// still receive their own message — the per-recipient filter exempts
	// the sender unconditionally once handleGlobal started executing.
	repo := newUserRepoStub()
	repo.add(10, "alice")

	game := newFakeGame()
	hub := &hubSpy{connected: []uint{10, 20, 30}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())

	// Capture the initial check, then flip the sender to in-game before
	// the broadcast filter runs. Simulate by mutating the fake between
	// HandleAction calls — but since HandleAction is synchronous, we
	// instead assert that even when the sender IS marked in-game on the
	// per-recipient pass, they still appear in the recipient list as long
	// as the initial check (before mutation) passed. Approximate by
	// pre-marking the sender as in-game and verifying the dedicated
	// "dropped" test catches that path; for THIS test we verify the
	// happy path includes the sender:
	game.inGame[20] = true // unrelated player in a game
	h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "lobby", "hi"))

	require.Equal(t, 1, hub.callCount())
	call := hub.lastCall(t)
	assert.Contains(t, call.userIDs, uint(10), "sender always echoed back")
	assert.NotContains(t, call.userIDs, uint(20), "in-game user excluded")
	assert.Contains(t, call.userIDs, uint(30), "idle user included")
}

func TestHandler_RejectsEmpty(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"only newlines", "\n\n\t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newUserRepoStub()
			repo.add(10, "alice")
			game := newFakeGame()
			hub := &hubSpy{connected: []uint{10, 20}}

			h := chat.NewHandler(hub, repo, game, newFakeRoom())
			h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "lobby", tc.text))

			assert.Equal(t, 0, hub.callCount())
		})
	}
}

func TestHandler_RejectsTooLong(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	tooLong := strings.Repeat("x", 501)
	h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "lobby", tooLong))

	assert.Equal(t, 0, hub.callCount(), "501-rune message rejected")

	// 500 runes exactly should be allowed
	exactly500 := strings.Repeat("y", 500)
	h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "lobby", exactly500))
	assert.Equal(t, 1, hub.callCount(), "500-rune message accepted (boundary)")
}

func TestHandler_LengthCapIsRunesNotBytes(t *testing.T) {
	// Cyrillic/emoji messages at the 500-rune boundary must be accepted.
	// Server uses utf8.RuneCountInString, not len(), so multi-byte runes
	// don't inflate against the cap. This matches the client's
	// `string.length` (UTF-16 code units approximate runes for the BMP).
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())

	// 500 Cyrillic "ч" runes = 1000 bytes — would be rejected by len() check
	cyrillic500 := strings.Repeat("ч", 500)
	h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "lobby", cyrillic500))
	assert.Equal(t, 1, hub.callCount(), "500-rune Cyrillic message accepted")

	// 501 Cyrillic runes — over the cap, must be rejected
	cyrillic501 := strings.Repeat("ч", 501)
	h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "lobby", cyrillic501))
	assert.Equal(t, 1, hub.callCount(), "501-rune Cyrillic message rejected (no new broadcast)")
}

func TestHandler_IgnoresUnknownChannel(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())

	// Should not panic, should not broadcast
	assert.NotPanics(t, func() {
		h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "private", "hello"))
	})
	assert.Equal(t, 0, hub.callCount())
}

// --- Match-scope chat tests ---

func matchActionMessage(t *testing.T, roomID uint, text string) ws.WSMessage {
	t.Helper()
	return chatActionMessageRaw(t, ws.ChatMessageRequest{
		Channel: "match",
		RoomID:  &roomID,
		Text:    text,
	})
}

func TestHandler_MatchMessage_BroadcastsToFourParticipants(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")

	game := newFakeGame()
	game.participants[7] = [4]uint{10, 20, 30, 40}

	// Connected set is intentionally broader than participants — match chat
	// must fan out to participants only, ignoring the hub snapshot.
	hub := &hubSpy{connected: []uint{10, 20, 30, 40, 50, 60}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	h.HandleAction(&ws.Client{UserID: 10}, matchActionMessage(t, 7, "hi team"))

	require.Equal(t, 1, hub.callCount())
	call := hub.lastCall(t)
	assert.ElementsMatch(t, []uint{10, 20, 30, 40}, call.userIDs,
		"broadcast goes to the 4 session participants")

	var env ws.WSMessage
	require.NoError(t, json.Unmarshal(call.msg, &env))
	assert.Equal(t, ws.SystemChatMessage, env.Type)

	var payload ws.ChatMessagePayload
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, uint(10), payload.UserID)
	assert.Equal(t, "alice", payload.Username)
	assert.Equal(t, "hi team", payload.Message)
	assert.Equal(t, "match", payload.Scope)
	assert.NotEmpty(t, payload.Timestamp)
}

func TestHandler_MatchMessage_SenderReceivesOwnEcho(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	game.participants[7] = [4]uint{10, 20, 30, 40}
	hub := &hubSpy{connected: []uint{10, 20, 30, 40}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	h.HandleAction(&ws.Client{UserID: 10}, matchActionMessage(t, 7, "own echo"))

	call := hub.lastCall(t)
	assert.Contains(t, call.userIDs, uint(10), "sender always included")
}

func TestHandler_MatchMessage_ExcludesNonParticipants(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	game.participants[7] = [4]uint{10, 20, 30, 40}
	// User 50 is connected but NOT a participant — must not appear in the
	// recipient list even though the hub reports them connected.
	hub := &hubSpy{connected: []uint{10, 20, 30, 40, 50}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	h.HandleAction(&ws.Client{UserID: 10}, matchActionMessage(t, 7, "hi"))

	call := hub.lastCall(t)
	assert.NotContains(t, call.userIDs, uint(50))
}

func TestHandler_MatchMessage_DroppedWhenRoomIDMissing(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	hub := &hubSpy{connected: []uint{10, 20, 30, 40}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	// Build a match-channel request with no roomId at all.
	msg := chatActionMessageRaw(t, ws.ChatMessageRequest{Channel: "match", Text: "hi"})
	h.HandleAction(&ws.Client{UserID: 10}, msg)

	assert.Equal(t, 0, hub.callCount(), "no broadcast when roomId is nil")
}

func TestHandler_MatchMessage_DroppedWhenMatchUnknown(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame() // no participants registered for any matchID
	hub := &hubSpy{connected: []uint{10, 20, 30, 40}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	h.HandleAction(&ws.Client{UserID: 10}, matchActionMessage(t, 999, "hi"))

	assert.Equal(t, 0, hub.callCount(), "no broadcast when matchId has no session")
}

func TestHandler_MatchMessage_DroppedWhenSenderNotParticipant(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(999, "interloper")
	game := newFakeGame()
	game.participants[7] = [4]uint{10, 20, 30, 40}
	hub := &hubSpy{connected: []uint{10, 20, 30, 40, 999}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	h.HandleAction(&ws.Client{UserID: 999}, matchActionMessage(t, 7, "hi"))

	assert.Equal(t, 0, hub.callCount(),
		"sender not among session participants → silently dropped")
}

func TestHandler_MatchMessage_RejectsEmptyAndTooLong(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	game.participants[7] = [4]uint{10, 20, 30, 40}
	hub := &hubSpy{connected: []uint{10, 20, 30, 40}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())

	// Empty / whitespace-only rejected via the shared prologue.
	for _, text := range []string{"", "   ", "\n\n"} {
		h.HandleAction(&ws.Client{UserID: 10}, matchActionMessage(t, 7, text))
	}
	assert.Equal(t, 0, hub.callCount(), "empty/whitespace match messages rejected")

	// 501 runes rejected.
	h.HandleAction(&ws.Client{UserID: 10}, matchActionMessage(t, 7, strings.Repeat("x", 501)))
	assert.Equal(t, 0, hub.callCount(), "501-rune match message rejected")

	// 500 runes accepted (boundary).
	h.HandleAction(&ws.Client{UserID: 10}, matchActionMessage(t, 7, strings.Repeat("y", 500)))
	assert.Equal(t, 1, hub.callCount(), "500-rune match message accepted")
}

func TestHandler_MatchMessage_ScopeStampedAsMatch(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	game.participants[7] = [4]uint{10, 20, 30, 40}
	hub := &hubSpy{connected: []uint{10, 20, 30, 40}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	h.HandleAction(&ws.Client{UserID: 10}, matchActionMessage(t, 7, "hi"))

	call := hub.lastCall(t)
	var env ws.WSMessage
	require.NoError(t, json.Unmarshal(call.msg, &env))
	var payload ws.ChatMessagePayload
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, "match", payload.Scope,
		"match broadcast payload must carry scope=match for client dispatch")
}

// --- Room-scope chat tests ---

func roomActionMessage(t *testing.T, roomID uint, text string) ws.WSMessage {
	t.Helper()
	return chatActionMessageRaw(t, ws.ChatMessageRequest{
		Channel: "room",
		RoomID:  &roomID,
		Text:    text,
	})
}

func TestHandler_RoomMessage_BroadcastsToAllMembers(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")

	game := newFakeGame()
	fakeRooms := newFakeRoom()
	// Room 5 has seated (10, 20) + unseated (30) members. Unseated (no seat)
	// still receives and can send — room_players membership is the only gate.
	fakeRooms.members[5] = []uint{10, 20, 30}

	hub := &hubSpy{connected: []uint{10, 20, 30, 40}}

	h := chat.NewHandler(hub, repo, game, fakeRooms)
	h.HandleAction(&ws.Client{UserID: 10}, roomActionMessage(t, 5, "room ping"))

	require.Equal(t, 1, hub.callCount())
	call := hub.lastCall(t)
	assert.ElementsMatch(t, []uint{10, 20, 30}, call.userIDs,
		"broadcast goes to every room_players member including unseated")

	var env ws.WSMessage
	require.NoError(t, json.Unmarshal(call.msg, &env))
	assert.Equal(t, ws.SystemChatMessage, env.Type)

	var payload ws.ChatMessagePayload
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, uint(10), payload.UserID)
	assert.Equal(t, "alice", payload.Username)
	assert.Equal(t, "room ping", payload.Message)
	assert.Equal(t, "room", payload.Scope)
	assert.NotEmpty(t, payload.Timestamp)
}

func TestHandler_RoomMessage_SenderReceivesOwnEcho(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	fakeRooms := newFakeRoom()
	fakeRooms.members[5] = []uint{10, 20, 30}
	hub := &hubSpy{connected: []uint{10, 20, 30}}

	h := chat.NewHandler(hub, repo, game, fakeRooms)
	h.HandleAction(&ws.Client{UserID: 10}, roomActionMessage(t, 5, "hi"))

	call := hub.lastCall(t)
	assert.Contains(t, call.userIDs, uint(10), "sender always included in room broadcast")
}

func TestHandler_RoomMessage_ExcludesNonMembers(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	fakeRooms := newFakeRoom()
	fakeRooms.members[5] = []uint{10, 20, 30}
	// User 99 is connected but NOT in the room — must not receive.
	hub := &hubSpy{connected: []uint{10, 20, 30, 99}}

	h := chat.NewHandler(hub, repo, game, fakeRooms)
	h.HandleAction(&ws.Client{UserID: 10}, roomActionMessage(t, 5, "hi"))

	call := hub.lastCall(t)
	assert.NotContains(t, call.userIDs, uint(99), "non-room user excluded from broadcast")
}

func TestHandler_RoomMessage_DroppedWhenRoomIDMissing(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	fakeRooms := newFakeRoom()
	fakeRooms.members[5] = []uint{10, 20}
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, fakeRooms)
	// channel:room with no roomId field at all
	msg := chatActionMessageRaw(t, ws.ChatMessageRequest{Channel: "room", Text: "hi"})
	h.HandleAction(&ws.Client{UserID: 10}, msg)

	assert.Equal(t, 0, hub.callCount(), "no broadcast when roomId is nil")
}

func TestHandler_RoomMessage_DroppedWhenRoomUnknownOrNotWaiting(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	// Room 999 is not registered → RoomMembers returns (nil, false).
	// fakeRoom semantics: absent from the map == unknown OR not waiting.
	fakeRooms := newFakeRoom()
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, fakeRooms)
	h.HandleAction(&ws.Client{UserID: 10}, roomActionMessage(t, 999, "hi"))

	assert.Equal(t, 0, hub.callCount(),
		"no broadcast when room unknown or status != waiting")
}

func TestHandler_RoomMessage_DroppedWhenSenderNotMember(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(99, "interloper")
	game := newFakeGame()
	fakeRooms := newFakeRoom()
	fakeRooms.members[5] = []uint{10, 20, 30}
	hub := &hubSpy{connected: []uint{10, 20, 30, 99}}

	h := chat.NewHandler(hub, repo, game, fakeRooms)
	h.HandleAction(&ws.Client{UserID: 99}, roomActionMessage(t, 5, "hi"))

	assert.Equal(t, 0, hub.callCount(),
		"sender not in room_players → silently dropped")
}

func TestHandler_RoomMessage_RejectsEmptyAndTooLong(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	fakeRooms := newFakeRoom()
	fakeRooms.members[5] = []uint{10, 20}
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, fakeRooms)

	for _, text := range []string{"", "   ", "\n"} {
		h.HandleAction(&ws.Client{UserID: 10}, roomActionMessage(t, 5, text))
	}
	assert.Equal(t, 0, hub.callCount(), "empty/whitespace rejected")

	h.HandleAction(&ws.Client{UserID: 10}, roomActionMessage(t, 5, strings.Repeat("x", 501)))
	assert.Equal(t, 0, hub.callCount(), "501-rune message rejected")

	h.HandleAction(&ws.Client{UserID: 10}, roomActionMessage(t, 5, strings.Repeat("y", 500)))
	assert.Equal(t, 1, hub.callCount(), "500-rune message accepted (boundary)")
}

func TestHandler_RoomMessage_RecipientInMatchStillReceives(t *testing.T) {
	// The in-game exclusion applies ONLY to global chat. A room member
	// who happens to be in an active game still gets their room chat.
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	game.inGame[20] = true // recipient 20 is in a match
	fakeRooms := newFakeRoom()
	fakeRooms.members[5] = []uint{10, 20}
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, fakeRooms)
	h.HandleAction(&ws.Client{UserID: 10}, roomActionMessage(t, 5, "hi"))

	call := hub.lastCall(t)
	assert.Contains(t, call.userIDs, uint(20),
		"in-game recipient still receives room chat — exclusion is global-scope only")
}

func TestHandler_IgnoresWrongActionType(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	wrongAction := ws.WSMessage{Type: ws.ActionPlayCard, Payload: json.RawMessage(`{}`)}
	h.HandleAction(&ws.Client{UserID: 10}, wrongAction)

	assert.Equal(t, 0, hub.callCount(),
		"non-chat action types are no-ops (composite routing safety)")
}

func TestHandler_InvalidPayload_NoBroadcast(t *testing.T) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	game := newFakeGame()
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	garbage := ws.WSMessage{Type: ws.ActionChatMessage, Payload: json.RawMessage(`not json`)}
	h.HandleAction(&ws.Client{UserID: 10}, garbage)

	assert.Equal(t, 0, hub.callCount())
}

func TestHandler_SenderNotFound_NoBroadcast(t *testing.T) {
	repo := newUserRepoStub() // user 10 not added
	game := newFakeGame()
	hub := &hubSpy{connected: []uint{10, 20}}

	h := chat.NewHandler(hub, repo, game, newFakeRoom())
	h.HandleAction(&ws.Client{UserID: 10}, chatActionMessage(t, "lobby", "hi"))

	assert.Equal(t, 0, hub.callCount())
}
