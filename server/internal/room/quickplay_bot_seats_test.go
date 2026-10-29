package room_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
)

func quickPlayRoomWithBots(t *testing.T, repo *mockRoomRepo, ownerID uint, botSeats ...int) *room.Room {
	t.Helper()
	qp := &room.Room{
		Name:        fmt.Sprintf("Quick Play BOTS%d", ownerID),
		Code:        fmt.Sprintf("QB%04d", ownerID),
		OwnerID:     ownerID,
		Variant:     "croatia",
		MatchMode:   "501",
		TimerStyle:  "per-move",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: 1,
	}
	require.NoError(t, repo.Create(qp))
	seat0, teamA := 0, "teamA"
	require.NoError(t, repo.AddPlayer(&room.RoomPlayer{RoomID: qp.ID, UserID: ownerID, Seat: &seat0, Team: &teamA, Username: "owner"}))
	for _, s := range botSeats {
		require.NoError(t, repo.AddBot(qp.ID, s))
	}
	return qp
}

type quickPlayResult struct {
	Room         room.Room `json:"room"`
	Seat         int       `json:"seat"`
	MatchStarted bool      `json:"matchStarted"`
}

func decodeQuickPlay(t *testing.T, rec *httptest.ResponseRecorder) quickPlayResult {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Data quickPlayResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data
}

func TestQuickPlay_NeverSeatsAHumanOnABotSeat(t *testing.T) {
	starter := &fakeMatchStarter{}
	e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
	qp := quickPlayRoomWithBots(t, repo, 500, 1)

	got := decodeQuickPlay(t, doQuickPlay(e, validToken(501)))

	assert.Equal(t, qp.ID, got.Room.ID)
	assert.Equal(t, 2, got.Seat, "seat 1 holds a bot, so the joiner takes seat 2")
	assert.False(t, got.MatchStarted, "three of four seats are filled")
	assert.Equal(t, 0, starter.called)
}

func TestQuickPlay_LastFreeSeatBesideBotsStartsAFullLineUp(t *testing.T) {
	starter := &fakeMatchStarter{}
	e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
	quickPlayRoomWithBots(t, repo, 500, 1, 2)

	got := decodeQuickPlay(t, doQuickPlay(e, validToken(501)))

	assert.Equal(t, 3, got.Seat)
	assert.True(t, got.MatchStarted)
	require.Equal(t, 1, starter.called)
	seats := starter.lastPlayers
	assert.Equal(t, uint(500), seats[0].UserID)
	assert.True(t, seats[1].IsBot)
	assert.True(t, seats[2].IsBot)
	assert.Equal(t, uint(501), seats[3].UserID)
	for i, s := range seats {
		assert.Equal(t, i, s.Seat)
	}
}

func TestQuickPlay_SkipsARoomWhoseSeatsBotsAlreadyFill(t *testing.T) {
	starter := &fakeMatchStarter{}
	e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
	full := quickPlayRoomWithBots(t, repo, 500, 1, 2, 3)

	got := decodeQuickPlay(t, doQuickPlay(e, validToken(501)))

	assert.NotEqual(t, full.ID, got.Room.ID, "a room with one human and three bots has no seat left")
	assert.Equal(t, 0, got.Seat)
}

func TestQuickPlay_DropsAMembershipLeftInAPlayingRoomWithoutAMatch(t *testing.T) {
	cases := []struct {
		name       string
		inMatch    bool
		startedAgo time.Duration
		wantStatus int
	}{
		{"no live match for a while", false, 2 * time.Minute, http.StatusOK},
		{"live match", true, 2 * time.Minute, http.StatusConflict},
		{"match still starting", false, time.Second, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			starter := &fakeMatchStarter{inMatch: map[uint]bool{40: tc.inMatch}}
			e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
			stuck := quickPlayRoomWithBots(t, repo, 40, 1, 2, 3)
			stuck.Status = "playing"
			stuck.UpdatedAt = time.Now().Add(-tc.startedAgo)

			rec := doQuickPlay(e, validToken(40))

			assert.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
		})
	}
}

func TestLeaveRoom_LeavesAPlayingRoomWithoutAMatch(t *testing.T) {
	starter := &fakeMatchStarter{}
	e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
	stuck := quickPlayRoomWithBots(t, repo, 40, 1, 2, 3)
	stuck.Status = "playing"
	stuck.UpdatedAt = time.Now().Add(-2 * time.Minute)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rooms/%d/leave", stuck.ID), nil)
	req.Header.Set("Authorization", "Bearer "+validToken(40))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	still, err := repo.FindPlayerRoom(40)
	require.NoError(t, err)
	assert.Nil(t, still)
}

func TestMyRoom_NamesTheCallersRoom(t *testing.T) {
	starter := &fakeMatchStarter{}
	e, repo := setupTestWithStarter(starter, &mockBroadcaster{})
	qp := quickPlayRoomWithBots(t, repo, 40, 1)

	get := func(userID uint) map[string]json.RawMessage {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/rooms/mine", nil)
		req.Header.Set("Authorization", "Bearer "+validToken(userID))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var resp map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		return resp
	}

	var mine struct {
		RoomID      uint   `json:"roomId"`
		Status      string `json:"status"`
		IsQuickPlay bool   `json:"isQuickPlay"`
	}
	require.NoError(t, json.Unmarshal(get(40)["data"], &mine))
	assert.Equal(t, qp.ID, mine.RoomID)
	assert.Equal(t, "waiting", mine.Status)
	assert.True(t, mine.IsQuickPlay)

	assert.Equal(t, "null", string(get(41)["data"]))
}
