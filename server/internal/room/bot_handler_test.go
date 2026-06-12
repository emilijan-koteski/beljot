package room_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
)

// --- Request helpers ---

func doAddBot(e *echo.Echo, id string, body string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/bots", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doRemoveBot(e *echo.Echo, id string, seat string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/rooms/"+id+"/bots/"+seat, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func errCodeOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var errObj map[string]string
	require.NoError(t, json.Unmarshal(resp["error"], &errObj))
	return errObj["code"]
}

// playersOf decodes the {"data":{"players":[...]}} envelope shared by the
// bot endpoints and swap-seats.
func playersOf(t *testing.T, rec *httptest.ResponseRecorder) []room.RoomPlayer {
	t.Helper()
	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var data struct {
		Players []room.RoomPlayer `json:"players"`
	}
	require.NoError(t, json.Unmarshal(resp["data"], &data))
	return data.Players
}

func botAtSeat(players []room.RoomPlayer, seat int) *room.RoomPlayer {
	for i := range players {
		if players[i].IsBot && players[i].Seat != nil && *players[i].Seat == seat {
			return &players[i]
		}
	}
	return nil
}

// seedOwnerRoom creates a waiting room owned by user 100 seated at seat 0.
func seedOwnerRoom(t *testing.T, repo *mockRoomRepo) *room.Room {
	t.Helper()
	r := &room.Room{Name: "Bot Test", OwnerID: 100, Status: "waiting", PlayerCount: 1}
	require.NoError(t, repo.Create(r))
	seat := 0
	team := "teamA"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{
		RoomID: r.ID, UserID: 100, Username: "Owner", Seat: &seat, Team: &team,
	}))
	return r
}

// --- AddBot ---

func TestAddBot_OwnerSeatsBot(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)

	rec := doAddBot(e, "1", `{"seat": 2}`, validToken(100))
	require.Equal(t, http.StatusCreated, rec.Code)

	players := playersOf(t, rec)
	bot := botAtSeat(players, 2)
	require.NotNil(t, bot, "players payload must contain the bot entry")
	assert.True(t, bot.IsBot)
	assert.Equal(t, uint(0), bot.UserID)
	assert.Equal(t, "", bot.Username)
	require.NotNil(t, bot.Team)
	assert.Equal(t, "teamA", *bot.Team)

	bots, err := repo.FindBotsByRoomID(r.ID)
	require.NoError(t, err)
	require.Len(t, bots, 1)
	assert.Equal(t, 2, bots[0].Seat)

	// system:bot_added to room members, then a lobby-wide room_updated snapshot.
	require.NotEmpty(t, broadcaster.calls)
	assert.Equal(t, "system:bot_added", msgTypeOf(t, broadcaster.calls[0].msg))
	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_updated", msgTypeOf(t, broadcaster.allCalls[0].msg))
}

func TestAddBot_ThreeBotsAnySeats(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)

	for _, seat := range []string{`{"seat": 1}`, `{"seat": 2}`, `{"seat": 3}`} {
		rec := doAddBot(e, "1", seat, validToken(100))
		require.Equal(t, http.StatusCreated, rec.Code)
	}
	bots, err := repo.FindBotsByRoomID(r.ID)
	require.NoError(t, err)
	assert.Len(t, bots, 3)
}

func TestAddBot_NonOwnerRejected(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "Guest"}))

	rec := doAddBot(e, "1", `{"seat": 2}`, validToken(200))
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "NOT_ROOM_OWNER", errCodeOf(t, rec))
}

func TestAddBot_SeatTakenByHuman(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	seat := 2
	team := "teamA"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "Guest", Seat: &seat, Team: &team}))

	rec := doAddBot(e, "1", `{"seat": 2}`, validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SEAT_TAKEN", errCodeOf(t, rec))
}

func TestAddBot_SeatTakenByBot(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	seedOwnerRoom(t, repo)

	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 2}`, validToken(100)).Code)
	rec := doAddBot(e, "1", `{"seat": 2}`, validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SEAT_TAKEN", errCodeOf(t, rec))
}

func TestAddBot_RoomNotWaiting(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	r.Status = "playing"

	rec := doAddBot(e, "1", `{"seat": 2}`, validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ROOM_NOT_WAITING", errCodeOf(t, rec))
}

func TestAddBot_QuickPlayRejected(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	r.IsQuickPlay = true

	rec := doAddBot(e, "1", `{"seat": 2}`, validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "BOTS_NOT_ALLOWED", errCodeOf(t, rec))
}

func TestAddBot_InvalidSeat(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	seedOwnerRoom(t, repo)

	for _, body := range []string{`{"seat": 4}`, `{"seat": -1}`, `{}`} {
		rec := doAddBot(e, "1", body, validToken(100))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "INVALID_SEAT", errCodeOf(t, rec))
	}
}

// --- RemoveBot ---

func TestRemoveBot_FreesSeat(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 2}`, validToken(100)).Code)
	broadcaster.calls = nil
	broadcaster.allCalls = nil

	rec := doRemoveBot(e, "1", "2", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	bots, err := repo.FindBotsByRoomID(r.ID)
	require.NoError(t, err)
	assert.Empty(t, bots)
	assert.Nil(t, botAtSeat(playersOf(t, rec), 2))

	require.NotEmpty(t, broadcaster.calls)
	assert.Equal(t, "system:bot_removed", msgTypeOf(t, broadcaster.calls[0].msg))

	// The freed seat is takeable again.
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 2}`, validToken(100)).Code)
}

func TestRemoveBot_EmptySeat(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	seedOwnerRoom(t, repo)

	rec := doRemoveBot(e, "1", "3", validToken(100))
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "NO_BOT_ON_SEAT", errCodeOf(t, rec))
}

func TestRemoveBot_NonOwnerRejected(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "Guest"}))
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 2}`, validToken(100)).Code)

	rec := doRemoveBot(e, "1", "2", validToken(200))
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "NOT_ROOM_OWNER", errCodeOf(t, rec))
}

// --- SelectSeat onto a bot seat ---

func TestSelectSeat_BotSeatReadsAsTaken(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "Guest"}))
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 2}`, validToken(100)).Code)

	rec := doSelectSeat(e, "1", `{"seat": 2}`, validToken(200))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SEAT_TAKEN", errCodeOf(t, rec))
}

// --- SwapSeats with bots ---

func TestSwapSeats_HumanWithBot(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 1}`, validToken(100)).Code)
	broadcaster.calls = nil
	broadcaster.allCalls = nil

	// Owner (human, seat 0) swaps with the bot on seat 1.
	rec := doSwapSeats(e, "1", `{"seatA": 0, "seatB": 1}`, validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	// Human moved 0 → 1 (team B), bot moved 1 → 0 (team A).
	humans, _ := repo.FindPlayersByRoomID(r.ID)
	require.Len(t, humans, 1)
	require.NotNil(t, humans[0].Seat)
	assert.Equal(t, 1, *humans[0].Seat)
	assert.Equal(t, "teamB", *humans[0].Team)

	bots, _ := repo.FindBotsByRoomID(r.ID)
	require.Len(t, bots, 1)
	assert.Equal(t, 0, bots[0].Seat)

	// Event sequence: seat_updated (human), bot_removed{1}, bot_added{0}.
	require.Len(t, broadcaster.calls, 3)
	assert.Equal(t, "system:seat_updated", msgTypeOf(t, broadcaster.calls[0].msg))
	assert.Equal(t, "system:bot_removed", msgTypeOf(t, broadcaster.calls[1].msg))
	assert.Equal(t, "system:bot_added", msgTypeOf(t, broadcaster.calls[2].msg))
	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_updated", msgTypeOf(t, broadcaster.allCalls[0].msg))

	var msg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(broadcaster.calls[1].msg, &msg))
	var removed struct {
		Seat int `json:"seat"`
	}
	require.NoError(t, json.Unmarshal(msg["payload"], &removed))
	assert.Equal(t, 1, removed.Seat)

	require.NoError(t, json.Unmarshal(broadcaster.calls[2].msg, &msg))
	var added struct {
		Seat int    `json:"seat"`
		Team string `json:"team"`
	}
	require.NoError(t, json.Unmarshal(msg["payload"], &added))
	assert.Equal(t, 0, added.Seat)
	assert.Equal(t, "teamA", added.Team)
}

func TestSwapSeats_BotToEmptySeat(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 1}`, validToken(100)).Code)
	broadcaster.calls = nil
	broadcaster.allCalls = nil

	rec := doSwapSeats(e, "1", `{"seatA": 1, "seatB": 3}`, validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	bots, _ := repo.FindBotsByRoomID(r.ID)
	require.Len(t, bots, 1)
	assert.Equal(t, 3, bots[0].Seat)

	// No human moved: bot_removed{1} + bot_added{3} only, then snapshot.
	require.Len(t, broadcaster.calls, 2)
	assert.Equal(t, "system:bot_removed", msgTypeOf(t, broadcaster.calls[0].msg))
	assert.Equal(t, "system:bot_added", msgTypeOf(t, broadcaster.calls[1].msg))

	// Bot name follows the seat: the response carries the bot at seat 3.
	assert.NotNil(t, botAtSeat(playersOf(t, rec), 3))
}

func TestSwapSeats_BotWithBot_NoOp(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 1}`, validToken(100)).Code)
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 2}`, validToken(100)).Code)
	broadcaster.calls = nil
	broadcaster.allCalls = nil

	rec := doSwapSeats(e, "1", `{"seatA": 1, "seatB": 2}`, validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	// No state change, no broadcasts.
	bots, _ := repo.FindBotsByRoomID(r.ID)
	require.Len(t, bots, 2)
	assert.Equal(t, 1, bots[0].Seat)
	assert.Equal(t, 2, bots[1].Seat)
	assert.Empty(t, broadcaster.calls)
	assert.Empty(t, broadcaster.allCalls)
}

func TestSwapSeats_NonOwnerWithBot(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	seat := 2
	team := "teamA"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "Guest", Seat: &seat, Team: &team}))
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 1}`, validToken(100)).Code)

	rec := doSwapSeats(e, "1", `{"seatA": 1, "seatB": 2}`, validToken(200))
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "NOT_ROOM_OWNER", errCodeOf(t, rec))
}

// --- StartMatch with bots ---

func doStartMatch(e *echo.Echo, id string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/start", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// seedMixedRoom seats `humans` human players (owner first at seat 0) and
// fills the remaining seats up to `humans+bots` with bots.
func seedMixedRoom(t *testing.T, e *echo.Echo, repo *mockRoomRepo, humans, bots int) *room.Room {
	t.Helper()
	r := &room.Room{Name: "Mixed Start", OwnerID: 100, Status: "waiting", PlayerCount: humans}
	require.NoError(t, repo.Create(r))
	for i := 0; i < humans; i++ {
		seat := i
		team := teamNameForSeat(i)
		require.NoError(t, repo.AddPlayer(&room.RoomPlayer{
			RoomID: r.ID, UserID: uint(100 + i*100), Username: "H", Seat: &seat, Team: &team,
		}))
	}
	for i := humans; i < humans+bots; i++ {
		rec := doAddBot(e, "1", `{"seat": `+string(rune('0'+i))+`}`, validToken(100))
		require.Equal(t, http.StatusCreated, rec.Code)
	}
	return r
}

func teamNameForSeat(seat int) string {
	if seat%2 == 0 {
		return "teamA"
	}
	return "teamB"
}

func TestStartMatch_WithBots(t *testing.T) {
	tests := []struct {
		name   string
		humans int
		bots   int
	}{
		{"one bot", 3, 1},
		{"two bots", 2, 2},
		{"three bots", 1, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			starter := &fakeMatchStarter{}
			e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
			seedMixedRoom(t, e, repo, tt.humans, tt.bots)

			rec := doStartMatch(e, "1", validToken(100))
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, 1, starter.called)

			for seat := 0; seat < 4; seat++ {
				info := starter.lastPlayers[seat]
				assert.Equal(t, seat, info.Seat)
				if seat < tt.humans {
					assert.False(t, info.IsBot, "seat %d should be human", seat)
					assert.Equal(t, uint(100+seat*100), info.UserID)
				} else {
					assert.True(t, info.IsBot, "seat %d should be a bot", seat)
					assert.Equal(t, uint(0), info.UserID)
					assert.Equal(t, "", info.Username)
				}
			}
		})
	}
}

func TestStartMatch_ThreeHumansOneEmptyStillFails(t *testing.T) {
	starter := &fakeMatchStarter{}
	e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
	seedMixedRoom(t, e, repo, 3, 0)

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "NOT_ALL_SEATED", errCodeOf(t, rec))
	assert.Equal(t, 0, starter.called)
}

// --- Room lifecycle invariants with bots ---

func TestLeaveRoom_SoleHumanWithBotsClosesRoom(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	for _, seat := range []string{`{"seat": 1}`, `{"seat": 2}`, `{"seat": 3}`} {
		require.Equal(t, http.StatusCreated, doAddBot(e, "1", seat, validToken(100)).Code)
	}

	rec := doLeaveRoom(e, "1", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	updated, err := repo.FindByID(r.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "completed", updated.Status, "bots never keep a room alive")
	assert.Equal(t, uint(100), updated.OwnerID, "ownership never transfers to a bot")
}

// --- Payload merging ---

func TestGetRoom_PlayersIncludeBots(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	seedOwnerRoom(t, repo)
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 3}`, validToken(100)).Code)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rooms/1", nil)
	req.Header.Set("Authorization", "Bearer "+validToken(100))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var detail struct {
		Players []room.RoomPlayer `json:"players"`
	}
	require.NoError(t, json.Unmarshal(resp["data"], &detail))
	require.Len(t, detail.Players, 2)
	bot := botAtSeat(detail.Players, 3)
	require.NotNil(t, bot)
	require.NotNil(t, bot.Team)
	assert.Equal(t, "teamB", *bot.Team)

	// Humans always serialize isBot:false.
	for _, p := range detail.Players {
		if p.UserID == 100 {
			assert.False(t, p.IsBot)
		}
	}
}

func TestListRooms_PreviewsIncludeBots(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	seedOwnerRoom(t, repo)
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 1}`, validToken(100)).Code)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+validToken(100))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var rooms []room.Room
	require.NoError(t, json.Unmarshal(resp["data"], &rooms))
	require.Len(t, rooms, 1)
	require.NotNil(t, botAtSeat(rooms[0].Players, 1))
}

// --- Capacity invariant: humans + bots never exceed the four seats ---

func TestJoinRoom_BotCoveredSeatsCountTowardCapacity(t *testing.T) {
	// Owner + 3 bots: PlayerCount is 1 but every seat is covered — a joiner
	// could never sit, so the room reads full for joiners.
	e, repo, _ := setupTestWithBroadcast()
	seedOwnerRoom(t, repo)
	for _, seat := range []string{`{"seat": 1}`, `{"seat": 2}`, `{"seat": 3}`} {
		require.Equal(t, http.StatusCreated, doAddBot(e, "1", seat, validToken(100)).Code)
	}

	rec := doJoinRoom(e, "1", validToken(500))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ROOM_FULL", errCodeOf(t, rec))
}

func TestAddBot_RejectedWhenMembersNeedTheSeats(t *testing.T) {
	// Owner seated + one unseated member: at most two bots fit — a third
	// would leave the member permanently unseatable in a waiting room.
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "Guest"}))
	repo.rooms[0].PlayerCount = 2

	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 1}`, validToken(100)).Code)
	require.Equal(t, http.StatusCreated, doAddBot(e, "1", `{"seat": 2}`, validToken(100)).Code)

	rec := doAddBot(e, "1", `{"seat": 3}`, validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ROOM_FULL", errCodeOf(t, rec))
}

func TestStartMatch_UnseatedMemberBlocksStart(t *testing.T) {
	// Owner + 3 bots cover the seats, but an unseated member is still in the
	// room — starting would strand them in a match they hold no seat in
	// (no userToRoom entry, no state, no reconnect window).
	starter := &fakeMatchStarter{}
	e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
	r := seedMixedRoom(t, e, repo, 1, 3)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 900, Username: "Lurker"}))

	rec := doStartMatch(e, "1", validToken(100))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "NOT_ALL_SEATED", errCodeOf(t, rec))
	assert.Equal(t, 0, starter.called)
}

func TestAddBot_NonOwnerWithJunkSeatGetsNotRoomOwner(t *testing.T) {
	// Validation order mirrors KickPlayer: ownership gates before seat-range
	// validation, so a probing non-owner learns nothing about seat shapes.
	e, repo, _ := setupTestWithBroadcast()
	r := seedOwnerRoom(t, repo)
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: 200, Username: "Guest"}))

	rec := doAddBot(e, "1", `{"seat": 9}`, validToken(200))
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "NOT_ROOM_OWNER", errCodeOf(t, rec))
}
