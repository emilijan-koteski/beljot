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

	"github.com/emilijan/beljot/server/internal/room"
)

func doReturnToRoom(e *echo.Echo, id string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/"+id+"/return", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// seedFinishedRoom builds a just-finished room (status "completed") with the
// given humans seated and bots occupying botSeats — the exact post-match shape
// the room is left in before anyone clicks "Return to room".
func seedFinishedRoom(repo *mockRoomRepo, ownerID uint, humanSeats map[uint]int, botSeats []int) *room.Room {
	r := &room.Room{
		Name:        "Rematch",
		Code:        "RMTCH1",
		OwnerID:     ownerID,
		Variant:     "bitola",
		MatchMode:   "1001",
		TimerStyle:  "relaxed",
		Status:      "completed",
		PlayerCount: len(humanSeats),
	}
	r.ID = repo.nextID
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	repo.nextID++
	repo.rooms = append(repo.rooms, r)

	for uid, seat := range humanSeats {
		s := seat
		team := teamNameForSeat(seat)
		repo.players = append(repo.players, &room.RoomPlayer{
			ID:        repo.nextPID,
			RoomID:    r.ID,
			UserID:    uid,
			Username:  "u",
			Seat:      &s,
			Team:      &team,
			CreatedAt: time.Now(),
		})
		repo.nextPID++
	}
	for _, seat := range botSeats {
		repo.bots = append(repo.bots, &room.RoomBot{ID: repo.nextBID, RoomID: r.ID, Seat: seat, CreatedAt: time.Now()})
		repo.nextBID++
	}
	return r
}

func errorCodeOf(t *testing.T, raw []byte) string {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &resp))
	return resp.Error.Code
}

// First return flips the completed room back to waiting, clears the previous
// match's bots, preserves the returner's seat, and broadcasts the open seats +
// the lobby refresh.
func TestReturnToRoom_FirstReturn_ReopensAndClearsBots(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	// owner=100 at seat 0, human 200 at seat 1; bots fill seats 2 and 3.
	seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, []int{2, 3})

	rec := doReturnToRoom(e, "1", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	// Room reopened.
	r, err := repo.FindByID(1)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "waiting", r.Status)

	// Bots cleared.
	bots, err := repo.FindBotsByRoomID(1)
	require.NoError(t, err)
	assert.Empty(t, bots)

	// Human seats preserved.
	players, err := repo.FindPlayersByRoomID(1)
	require.NoError(t, err)
	require.Len(t, players, 2)
	for _, p := range players {
		require.NotNil(t, p.Seat, "returner %d lost their seat", p.UserID)
	}

	// Broadcasts: one system:bot_removed per cleared seat, plus a lobby
	// system:room_updated.
	require.Len(t, broadcaster.calls, 2)
	assert.Equal(t, "system:bot_removed", msgTypeOf(t, broadcaster.calls[0].msg))
	assert.Equal(t, "system:bot_removed", msgTypeOf(t, broadcaster.calls[1].msg))
	require.Len(t, broadcaster.allCalls, 1)
	assert.Equal(t, "system:room_updated", msgTypeOf(t, broadcaster.allCalls[0].msg))

	// Response carries the reopened room detail (humans only — bots gone).
	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	var detail room.RoomDetailResponse
	require.NoError(t, json.Unmarshal(resp["data"], &detail))
	require.NotNil(t, detail.Room)
	assert.Equal(t, "waiting", detail.Room.Status)
	assert.Len(t, detail.Players, 2)
}

// A return when the room is already waiting (an earlier caller reopened it) is
// an idempotent no-op: status stays waiting, no bot is cleared, and no
// duplicate reopen side effects fire.
func TestReturnToRoom_AlreadyWaiting_Idempotent(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	r := seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, []int{2, 3})
	r.Status = "waiting" // already reopened by a prior returner

	rec := doReturnToRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusOK, rec.Code)

	// Still waiting; bots left untouched (only the first reopen clears them).
	assert.Equal(t, "waiting", r.Status)
	bots, err := repo.FindBotsByRoomID(1)
	require.NoError(t, err)
	assert.Len(t, bots, 2)

	// Idempotent path stays fully silent — neither a bot_removed fan-out nor a
	// redundant lobby room_updated (the first returner already broadcast both).
	assert.Empty(t, broadcaster.calls)
	assert.Empty(t, broadcaster.allCalls)
}

// A player who is no longer a member (kicked, or who left) cannot reopen or
// re-enter the room.
func TestReturnToRoom_NonMember_Rejected(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, []int{2, 3})

	rec := doReturnToRoom(e, "1", validToken(999)) // 999 is not in the room
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "NOT_IN_ROOM", errorCodeOf(t, rec.Body.Bytes()))

	// Room untouched, nothing broadcast.
	r, err := repo.FindByID(1)
	require.NoError(t, err)
	assert.Equal(t, "completed", r.Status)
	assert.Empty(t, broadcaster.calls)
	assert.Empty(t, broadcaster.allCalls)
}

// A live match cannot be reopened — the "playing" status is rejected.
func TestReturnToRoom_PlayingRoom_Rejected(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	r := seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, []int{2, 3})
	r.Status = "playing"

	rec := doReturnToRoom(e, "1", validToken(100))
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "MATCH_ALREADY_STARTED", errorCodeOf(t, rec.Body.Bytes()))
	assert.Equal(t, "playing", r.Status)
}
