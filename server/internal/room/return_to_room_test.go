package room_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
)

func doPostJSON(e *echo.Echo, path string, token string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decodeRoomDetail(t *testing.T, raw []byte) room.RoomDetailResponse {
	t.Helper()
	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &resp))
	var detail room.RoomDetailResponse
	require.NoError(t, json.Unmarshal(resp["data"], &detail))
	return detail
}

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

	// Broadcasts: one system:bot_removed per cleared seat, then a
	// system:player_returned for the returner (v2), all room-scoped; plus a
	// lobby-wide system:room_updated.
	require.Len(t, broadcaster.calls, 3)
	assert.Equal(t, "system:bot_removed", msgTypeOf(t, broadcaster.calls[0].msg))
	assert.Equal(t, "system:bot_removed", msgTypeOf(t, broadcaster.calls[1].msg))
	assert.Equal(t, "system:player_returned", msgTypeOf(t, broadcaster.calls[2].msg))
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

	// The reopen side effects stay silent — no bot_removed fan-out, no redundant
	// lobby room_updated (the first returner already broadcast both). But the
	// returner is still announced present: a single system:player_returned fires
	// (v2) so the owner's "waiting to return" gate updates.
	require.Len(t, broadcaster.calls, 1)
	assert.Equal(t, "system:player_returned", msgTypeOf(t, broadcaster.calls[0].msg))
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

// v2 presence: each /return marks the caller present (surfaced as
// returnedUserIds) and announces it with a system:player_returned broadcast;
// later returners accumulate.
func TestReturnToRoom_MarksPresentAndBroadcasts(t *testing.T) {
	e, repo, broadcaster := setupTestWithBroadcast()
	seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, nil)

	rec := doReturnToRoom(e, "1", validToken(100))
	require.Equal(t, http.StatusOK, rec.Code)

	detail := decodeRoomDetail(t, rec.Body.Bytes())
	assert.Equal(t, []uint{100}, detail.ReturnedUserIds, "returner should be present")

	// No bots to clear → the only room-scoped broadcast is player_returned.
	require.Len(t, broadcaster.calls, 1)
	assert.Equal(t, "system:player_returned", msgTypeOf(t, broadcaster.calls[0].msg))
	var payload struct {
		RoomID uint `json:"roomId"`
		UserID uint `json:"userId"`
	}
	require.NoError(t, json.Unmarshal(payloadOf(t, broadcaster.calls[0].msg), &payload))
	assert.Equal(t, uint(1), payload.RoomID)
	assert.Equal(t, uint(100), payload.UserID)

	// A second returner accumulates (sorted ascending for deterministic payloads).
	rec2 := doReturnToRoom(e, "1", validToken(200))
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, []uint{100, 200}, decodeRoomDetail(t, rec2.Body.Bytes()).ReturnedUserIds)
}

// v2 presence: kicking a member drops them from the presence set so they no
// longer count toward the owner's "all seated humans present" Start gate.
func TestReturnToRoom_KickRemovesPresence(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, nil)

	require.Equal(t, http.StatusOK, doReturnToRoom(e, "1", validToken(100)).Code)
	require.Equal(t, http.StatusOK, doReturnToRoom(e, "1", validToken(200)).Code)

	require.Equal(t, http.StatusOK, doPostJSON(e, "/api/v1/rooms/1/kick", validToken(100), `{"userId":200}`).Code)

	detail := decodeRoomDetail(t, doGetRoom(e, "1", validToken(100)).Body.Bytes())
	assert.Equal(t, []uint{100}, detail.ReturnedUserIds, "kicked player should be dropped from presence")
}

// v2 presence: leaving the room drops the leaver from the presence set.
func TestReturnToRoom_LeaveRemovesPresence(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1}, nil)

	require.Equal(t, http.StatusOK, doReturnToRoom(e, "1", validToken(100)).Code)
	require.Equal(t, http.StatusOK, doReturnToRoom(e, "1", validToken(200)).Code)

	require.Equal(t, http.StatusOK, doPostJSON(e, "/api/v1/rooms/1/leave", validToken(200), "").Code)

	detail := decodeRoomDetail(t, doGetRoom(e, "1", validToken(100)).Body.Bytes())
	assert.Equal(t, []uint{100}, detail.ReturnedUserIds, "leaver should be dropped from presence")
}

// v2 presence: starting the match clears the room's presence set so the next
// reopen starts from an empty "who's back" state.
func TestReturnToRoom_StartClearsPresence(t *testing.T) {
	e, repo, _ := setupTestWithBroadcast()
	seedFinishedRoom(repo, 100, map[uint]int{100: 0, 200: 1, 300: 2, 400: 3}, nil)

	for _, uid := range []uint{100, 200, 300, 400} {
		require.Equal(t, http.StatusOK, doReturnToRoom(e, "1", validToken(uid)).Code)
	}
	before := decodeRoomDetail(t, doGetRoom(e, "1", validToken(100)).Body.Bytes())
	assert.Equal(t, []uint{100, 200, 300, 400}, before.ReturnedUserIds)

	require.Equal(t, http.StatusOK, doPostJSON(e, "/api/v1/rooms/1/start", validToken(100), "").Code)

	after := decodeRoomDetail(t, doGetRoom(e, "1", validToken(100)).Body.Bytes())
	assert.Empty(t, after.ReturnedUserIds, "presence should be cleared on match start")
}
