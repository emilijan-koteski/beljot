package chat_test

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/chat"
	"github.com/emilijan/beljot/server/internal/ws"
)

// --- Whisper fakes ---

type whisperSend struct {
	userID uint
	msg    []byte
}

// whisperHubSpy captures per-user SendToUser calls and answers IsConnected from
// a fixed connected set. Distinct from hubSpy (chat) because whisper delivers
// per-user + pre-checks connectivity, which chat's Broadcaster does not.
type whisperHubSpy struct {
	mu        sync.Mutex
	connected map[uint]bool
	sends     []whisperSend
}

func newWhisperHubSpy(connected ...uint) *whisperHubSpy {
	set := make(map[uint]bool, len(connected))
	for _, id := range connected {
		set[id] = true
	}
	return &whisperHubSpy{connected: set}
}

func (h *whisperHubSpy) IsConnected(userID uint) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.connected[userID]
}

func (h *whisperHubSpy) SendToUser(userID uint, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	dup := make([]byte, len(msg))
	copy(dup, msg)
	h.sends = append(h.sends, whisperSend{userID: userID, msg: dup})
}

func (h *whisperHubSpy) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sends)
}

// sendsTo returns the messages delivered to a specific user.
func (h *whisperHubSpy) sendsTo(userID uint) [][]byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out [][]byte
	for _, s := range h.sends {
		if s.userID == userID {
			out = append(out, s.msg)
		}
	}
	return out
}

type fakeFriends struct {
	pairs map[[2]uint]bool
	err   error
}

func newFakeFriends(pairs ...[2]uint) *fakeFriends {
	m := make(map[[2]uint]bool, len(pairs))
	for _, p := range pairs {
		m[p] = true
	}
	return &fakeFriends{pairs: m}
}

func (f *fakeFriends) AreFriends(a, b uint) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.pairs[[2]uint{a, b}] || f.pairs[[2]uint{b, a}], nil
}

type fakePresence struct {
	rooms map[uint]uint // userID → active roomID; absent means "not in any active room/match"
	err   error
}

func newFakePresence() *fakePresence {
	return &fakePresence{rooms: make(map[uint]uint)}
}

func (p *fakePresence) ActiveRoomID(userID uint) (uint, bool, error) {
	if p.err != nil {
		return 0, false, p.err
	}
	r, ok := p.rooms[userID]
	return r, ok, nil
}

// --- Whisper helpers ---

func whisperMessage(t *testing.T, toUsername, text string) ws.WSMessage {
	t.Helper()
	payload, err := json.Marshal(ws.WhisperRequest{ToUsername: toUsername, Text: text})
	require.NoError(t, err)
	return ws.WSMessage{Type: ws.ActionWhisper, Payload: payload}
}

func decodeWhisperEnvelope(t *testing.T, raw []byte) ws.WSMessage {
	t.Helper()
	var env ws.WSMessage
	require.NoError(t, json.Unmarshal(raw, &env))
	return env
}

// newWhisperFixture wires a handler with alice(10)/bob(20)/carol(30) seeded, an
// online hub of the given IDs, and a friend/presence pair the caller mutates.
func newWhisperFixture(hub *whisperHubSpy, friends *fakeFriends, presence *fakePresence) (*chat.WhisperHandler, *userRepoStub) {
	repo := newUserRepoStub()
	repo.add(10, "alice")
	repo.add(20, "bob")
	repo.add(30, "carol")
	return chat.NewWhisperHandler(hub, repo, friends, presence), repo
}

// --- Tests ---

func TestWhisper_FriendWhisper_DeliveredToBothParticipants(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), newFakePresence())

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey there"))

	// Delivered exactly once to the target AND once to the sender (own-echo).
	require.Equal(t, 2, hub.count(), "whisper delivered to BOTH participants")
	require.Len(t, hub.sendsTo(20), 1, "target receives the whisper")
	require.Len(t, hub.sendsTo(10), 1, "sender receives own-echo")

	env := decodeWhisperEnvelope(t, hub.sendsTo(20)[0])
	assert.Equal(t, ws.SystemWhisper, env.Type)

	var payload ws.WhisperPayload
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, uint(10), payload.FromUserID)
	assert.Equal(t, "alice", payload.FromUsername)
	assert.Equal(t, uint(20), payload.ToUserID)
	assert.Equal(t, "bob", payload.ToUsername)
	assert.Equal(t, "hey there", payload.Message)
	assert.NotEmpty(t, payload.Timestamp, "server stamps RFC3339 timestamp")

	// Sender's own-echo carries the identical payload.
	senderEnv := decodeWhisperEnvelope(t, hub.sendsTo(10)[0])
	assert.Equal(t, ws.SystemWhisper, senderEnv.Type)
}

func TestWhisper_NonFriend_RejectedToSenderOnly(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	// alice and bob are NOT friends.
	h, _ := newWhisperFixture(hub, newFakeFriends(), newFakePresence())

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey"))

	require.Equal(t, 1, hub.count(), "only the error goes out")
	require.Len(t, hub.sendsTo(20), 0, "target never receives a rejected whisper")
	env := decodeWhisperEnvelope(t, hub.sendsTo(10)[0])
	assert.Equal(t, ws.ErrorNotFriends, env.Type)
}

func TestWhisper_UnknownUsername_RejectedAsNotFriends(t *testing.T) {
	hub := newWhisperHubSpy(10)
	h, _ := newWhisperFixture(hub, newFakeFriends(), newFakePresence())

	// "ghost" resolves to no user — do NOT disclose "no such user"; it's simply
	// not a friend.
	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "ghost", "hey"))

	require.Equal(t, 1, hub.count())
	env := decodeWhisperEnvelope(t, hub.sendsTo(10)[0])
	assert.Equal(t, ws.ErrorNotFriends, env.Type)
}

func TestWhisper_SameRoomOrMatch_Blocked(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	presence := newFakePresence()
	presence.rooms[10] = 7 // both in the same active room/match
	presence.rooms[20] = 7
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), presence)

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey"))

	require.Equal(t, 1, hub.count(), "blocked whisper is never delivered")
	require.Len(t, hub.sendsTo(20), 0)
	env := decodeWhisperEnvelope(t, hub.sendsTo(10)[0])
	assert.Equal(t, ws.ErrorWhisperBlockedInGame, env.Type)
}

func TestWhisper_DifferentRoom_Allowed(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	presence := newFakePresence()
	presence.rooms[10] = 7 // different active rooms — a valid whisper target
	presence.rooms[20] = 9
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), presence)

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey"))

	require.Equal(t, 2, hub.count(), "friends in different rooms may whisper")
	require.Len(t, hub.sendsTo(20), 1)
}

func TestWhisper_OneInRoomOneInLobby_Allowed(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	presence := newFakePresence()
	presence.rooms[10] = 7 // sender in a room, target in the lobby (absent)
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), presence)

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey"))

	require.Equal(t, 2, hub.count(), "only a SHARED room/match blocks — not either being in one")
}

func TestWhisper_OfflineTarget_RejectedAndNotDelivered(t *testing.T) {
	// Only the sender is connected; bob is offline.
	hub := newWhisperHubSpy(10)
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), newFakePresence())

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey"))

	require.Equal(t, 1, hub.count(), "only the offline error goes out")
	require.Len(t, hub.sendsTo(20), 0, "offline target receives nothing")
	env := decodeWhisperEnvelope(t, hub.sendsTo(10)[0])
	assert.Equal(t, ws.ErrorWhisperRecipientOffline, env.Type)
}

func TestWhisper_RejectsEmptyAndTooLong(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), newFakePresence())

	for _, text := range []string{"", "   ", "\n\t"} {
		h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", text))
	}
	assert.Equal(t, 0, hub.count(), "empty/whitespace whispers ignored (nothing sent, no error)")

	// 501 runes rejected (no delivery).
	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", strings.Repeat("x", 501)))
	assert.Equal(t, 0, hub.count(), "501-rune whisper rejected")

	// 500 runes accepted (boundary).
	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", strings.Repeat("y", 500)))
	assert.Equal(t, 2, hub.count(), "500-rune whisper accepted and delivered to both")
}

func TestWhisper_LengthCapIsRunesNotBytes(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), newFakePresence())

	// 500 Cyrillic runes = 1000 bytes — accepted (rune cap, not byte cap).
	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", strings.Repeat("ч", 500)))
	assert.Equal(t, 2, hub.count(), "500-rune Cyrillic whisper accepted")

	// 501 Cyrillic runes rejected — no new delivery.
	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", strings.Repeat("ч", 501)))
	assert.Equal(t, 2, hub.count(), "501-rune Cyrillic whisper rejected")
}

func TestWhisper_SelfWhisper_Ignored(t *testing.T) {
	hub := newWhisperHubSpy(10)
	h, _ := newWhisperFixture(hub, newFakeFriends(), newFakePresence())

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "alice", "note to self"))

	assert.Equal(t, 0, hub.count(), "a self-whisper is silently ignored (no delivery, no error)")
}

func TestWhisper_WrongActionType_NoOp(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), newFakePresence())

	wrong := ws.WSMessage{Type: ws.ActionChatMessage, Payload: json.RawMessage(`{}`)}
	assert.NotPanics(t, func() { h.HandleAction(&ws.Client{UserID: 10}, wrong) })
	assert.Equal(t, 0, hub.count(), "non-whisper action types are a silent no-op (composite routing)")
}

func TestWhisper_InvalidPayload_NoOp(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), newFakePresence())

	garbage := ws.WSMessage{Type: ws.ActionWhisper, Payload: json.RawMessage(`not json`)}
	h.HandleAction(&ws.Client{UserID: 10}, garbage)
	assert.Equal(t, 0, hub.count())
}

func TestWhisper_FriendCheckError_FailsClosed(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	friends := newFakeFriends()
	friends.err = errors.New("db down")
	h, _ := newWhisperFixture(hub, friends, newFakePresence())

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey"))

	assert.Equal(t, 0, hub.count(),
		"an internal friend-check error fails closed — no delivery, no leak")
}

func TestWhisper_PresenceLookupError_FailsClosed(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	presence := newFakePresence()
	presence.err = errors.New("presence db down")
	// alice & bob ARE friends — the presence lookup is what fails.
	h, _ := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), presence)

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey"))

	assert.Equal(t, 0, hub.count(),
		"an internal presence-lookup error fails closed — no delivery, no error leak")
}

func TestWhisper_TargetLookupError_FailsClosed(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	h, repo := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), newFakePresence())
	// The target username resolution errors — must fail closed WITHOUT leaking
	// "not_friends" (which would disclose an internal DB fault as a friend verdict).
	repo.findByUsernameErr = errors.New("user db down")

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey"))

	assert.Equal(t, 0, hub.count(),
		"an internal target-lookup error fails closed — no delivery, no error to the sender")
}

func TestWhisper_SenderLookupError_FailsClosed(t *testing.T) {
	hub := newWhisperHubSpy(10, 20)
	h, repo := newWhisperFixture(hub, newFakeFriends([2]uint{10, 20}), newFakePresence())
	// The sender resolution errors before anything else — silent no-op.
	repo.findByIDErr = errors.New("user db down")

	h.HandleAction(&ws.Client{UserID: 10}, whisperMessage(t, "bob", "hey"))

	assert.Equal(t, 0, hub.count(),
		"an internal sender-lookup error fails closed — no delivery, no error")
}
