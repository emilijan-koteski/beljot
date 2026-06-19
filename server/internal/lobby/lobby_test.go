package lobby_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/lobby"
	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

type fakeHub struct {
	connected []uint
}

func (h *fakeHub) ConnectedUserIDs() []uint {
	out := make([]uint, len(h.connected))
	copy(out, h.connected)
	return out
}

type fakeSessions struct {
	inMatch []uint
}

func (s *fakeSessions) InMatchUserIDs() []uint {
	out := make([]uint, len(s.inMatch))
	copy(out, s.inMatch)
	return out
}

type fakeRoomRepo struct {
	usersByStatus   map[string][]uint
	roomsByStatus   map[string][]room.Room
	findByStatusErr error
}

func (r *fakeRoomRepo) FindUserIDsByRoomStatus(status string) ([]uint, error) {
	return r.usersByStatus[status], nil
}

// All other RoomRepository methods are unused by GetStats — panic loudly so a
// regression that touches them shows up in test output.
func (r *fakeRoomRepo) Create(*room.Room) error                    { panic("unused") }
func (r *fakeRoomRepo) Update(*room.Room) error                    { panic("unused") }
func (r *fakeRoomRepo) FindByID(uint) (*room.Room, error)          { panic("unused") }
func (r *fakeRoomRepo) FindByIDForUpdate(uint) (*room.Room, error) { panic("unused") }
func (r *fakeRoomRepo) FindByCode(string) (*room.Room, error)      { panic("unused") }
func (r *fakeRoomRepo) FindByStatus(status string) ([]room.Room, error) {
	return r.roomsByStatus[status], r.findByStatusErr
}
func (r *fakeRoomRepo) AddPlayer(*room.RoomPlayer) error { panic("unused") }
func (r *fakeRoomRepo) RemovePlayer(uint, uint) error    { panic("unused") }
func (r *fakeRoomRepo) FindPlayersByRoomID(uint) ([]room.RoomPlayer, error) {
	panic("unused")
}
func (r *fakeRoomRepo) FindPlayerRoom(uint) (*room.RoomPlayer, error)  { panic("unused") }
func (r *fakeRoomRepo) IncrementPlayerCount(uint) error                { panic("unused") }
func (r *fakeRoomRepo) DecrementPlayerCount(uint) error                { panic("unused") }
func (r *fakeRoomRepo) UpdatePlayerSeat(uint, uint, int, string) error { panic("unused") }
func (r *fakeRoomRepo) ClearPlayerSeat(uint, uint) error               { panic("unused") }
func (r *fakeRoomRepo) FindPlayerBySeat(uint, int) (*room.RoomPlayer, error) {
	panic("unused")
}
func (r *fakeRoomRepo) FindQuickPlayRoom(int) (*room.Room, error) { panic("unused") }
func (r *fakeRoomRepo) FindQuickPlayRoomExcluding(map[uint]bool, int) (*room.Room, error) {
	panic("unused")
}
func (r *fakeRoomRepo) UpdateStatus(uint, string) error                        { panic("unused") }
func (r *fakeRoomRepo) RunInTransaction(func(room.RoomRepository) error) error { panic("unused") }
func (r *fakeRoomRepo) LoadOwnerUsernames([]*room.Room) error                  { return nil }
func (r *fakeRoomRepo) FindPlayersByRoomIDs([]uint) (map[uint][]room.RoomPlayer, error) {
	return map[uint][]room.RoomPlayer{}, nil
}
func (r *fakeRoomRepo) AddBot(uint, int) error                        { panic("unused") }
func (r *fakeRoomRepo) RemoveBot(uint, int) error                     { panic("unused") }
func (r *fakeRoomRepo) UpdateBotSeat(uint, int, int) error            { panic("unused") }
func (r *fakeRoomRepo) FindBotsByRoomID(uint) ([]room.RoomBot, error) { return nil, nil }
func (r *fakeRoomRepo) FindBotsByRoomIDs([]uint) (map[uint][]room.RoomBot, error) {
	return map[uint][]room.RoomBot{}, nil
}

type fakeUserRepo struct {
	count    int64
	countErr error
}

func (u *fakeUserRepo) Count() (int64, error) {
	return u.count, u.countErr
}

// Unused — panic to surface accidental coupling.
func (u *fakeUserRepo) Create(*user.User) error                     { panic("unused") }
func (u *fakeUserRepo) FindByEmail(string) (*user.User, error)      { panic("unused") }
func (u *fakeUserRepo) FindByUsername(string) (*user.User, error)   { panic("unused") }
func (u *fakeUserRepo) FindByID(uint) (*user.User, error)           { panic("unused") }
func (u *fakeUserRepo) FindManyByIDs([]uint) ([]user.User, error)   { panic("unused") }
func (u *fakeUserRepo) UpdateLanguagePreference(uint, string) error { panic("unused") }
func (u *fakeUserRepo) UpdatePasswordHash(uint, string) error       { panic("unused") }

func decodeStats(t *testing.T, body []byte) lobby.StatsResponse {
	t.Helper()
	var env struct {
		Data lobby.StatsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	return env.Data
}

func TestGetStats_BucketsConnectedUsers(t *testing.T) {
	// 1, 2, 3, 4, 5 online. 1+2 in a game, 3 in a waiting room, 4+5 idle.
	// 6 in a waiting room but offline → must NOT be counted (connection-aware).
	hub := &fakeHub{connected: []uint{1, 2, 3, 4, 5}}
	sessions := &fakeSessions{inMatch: []uint{1, 2}}
	rooms := &fakeRoomRepo{
		usersByStatus: map[string][]uint{
			"waiting": {3, 6},
		},
	}
	users := &fakeUserRepo{count: 100}

	h := lobby.NewHandler(hub, sessions, rooms, users)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/lobby/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.GetStats(c))
	require.Equal(t, http.StatusOK, rec.Code)

	stats := decodeStats(t, rec.Body.Bytes())
	assert.Equal(t, 2, stats.InMatch, "users 1 and 2")
	assert.Equal(t, 1, stats.InRoom, "user 3 only — 6 is offline")
	assert.Equal(t, 2, stats.InLobby, "users 4 and 5")
	assert.Equal(t, 5, stats.Online)
	assert.Equal(t, int64(100), stats.Registered)
	// Invariant: online == sum of buckets.
	assert.Equal(t, stats.Online, stats.InLobby+stats.InRoom+stats.InMatch)
}

func TestGetStats_InMatchWinsOverInRoom(t *testing.T) {
	// User 7 appears in both the session manager (in game) and the
	// room_players table (waiting). Stale waiting row must not double-count.
	hub := &fakeHub{connected: []uint{7}}
	sessions := &fakeSessions{inMatch: []uint{7}}
	rooms := &fakeRoomRepo{usersByStatus: map[string][]uint{"waiting": {7}}}
	users := &fakeUserRepo{count: 1}

	h := lobby.NewHandler(hub, sessions, rooms, users)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/lobby/stats", nil)
	rec := httptest.NewRecorder()
	require.NoError(t, h.GetStats(e.NewContext(req, rec)))

	stats := decodeStats(t, rec.Body.Bytes())
	assert.Equal(t, 1, stats.InMatch)
	assert.Equal(t, 0, stats.InRoom)
	assert.Equal(t, 0, stats.InLobby)
	assert.Equal(t, 1, stats.Online)
}

func TestGetStats_CountsRequesterBeforeTheirSocketRegisters(t *testing.T) {
	// On login the lobby's first stats fetch races the WS auth handshake: the
	// HTTP request usually lands before the hub has registered the user's
	// socket. A lone player then saw "0 online / 0 in lobby" until the next
	// poll. The requester is online by definition (they just made an
	// authenticated request), so GetStats must count them even when the hub
	// doesn't know them yet.
	hub := &fakeHub{connected: nil} // socket not registered yet
	sessions := &fakeSessions{}
	rooms := &fakeRoomRepo{}
	users := &fakeUserRepo{count: 1}

	h := lobby.NewHandler(hub, sessions, rooms, users)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/lobby/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("userID", uint(9)) // what the auth middleware sets for the requester

	require.NoError(t, h.GetStats(c))

	stats := decodeStats(t, rec.Body.Bytes())
	assert.Equal(t, 1, stats.Online, "the requester counts as online")
	assert.Equal(t, 1, stats.InLobby, "a lone fresh login is in the lobby")
	assert.Equal(t, 0, stats.InRoom)
	assert.Equal(t, 0, stats.InMatch)
}

func TestGetStats_RequesterAlreadyConnectedNotDoubleCounted(t *testing.T) {
	// Once the requester's socket IS registered, counting them explicitly
	// must not inflate the totals.
	hub := &fakeHub{connected: []uint{9}}
	sessions := &fakeSessions{}
	rooms := &fakeRoomRepo{}
	users := &fakeUserRepo{count: 1}

	h := lobby.NewHandler(hub, sessions, rooms, users)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/lobby/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("userID", uint(9))

	require.NoError(t, h.GetStats(c))

	stats := decodeStats(t, rec.Body.Bytes())
	assert.Equal(t, 1, stats.Online)
	assert.Equal(t, 1, stats.InLobby)
}

func TestGetStats_EmptyHub(t *testing.T) {
	hub := &fakeHub{connected: nil}
	sessions := &fakeSessions{inMatch: nil}
	rooms := &fakeRoomRepo{usersByStatus: nil}
	users := &fakeUserRepo{count: 42}

	h := lobby.NewHandler(hub, sessions, rooms, users)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/lobby/stats", nil)
	rec := httptest.NewRecorder()
	require.NoError(t, h.GetStats(e.NewContext(req, rec)))

	stats := decodeStats(t, rec.Body.Bytes())
	assert.Equal(t, 0, stats.Online)
	assert.Equal(t, 0, stats.InLobby)
	assert.Equal(t, 0, stats.InRoom)
	assert.Equal(t, 0, stats.InMatch)
	assert.Equal(t, int64(42), stats.Registered)
}

func TestGetStats_UserCountErrorPropagates(t *testing.T) {
	users := &fakeUserRepo{countErr: errors.New("db down")}
	hub := &fakeHub{connected: []uint{1}}
	sessions := &fakeSessions{}
	rooms := &fakeRoomRepo{}

	h := lobby.NewHandler(hub, sessions, rooms, users)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/lobby/stats", nil)
	rec := httptest.NewRecorder()
	err := h.GetStats(e.NewContext(req, rec))
	assert.Error(t, err)
}

func decodePublicStats(t *testing.T, body []byte) lobby.PublicStatsResponse {
	t.Helper()
	var env struct {
		Data lobby.PublicStatsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	return env.Data
}

func TestGetPublicStats_CountsOnlineAndOpenRooms(t *testing.T) {
	// 3 connected players; 2 rooms in "waiting" status (the "open tables").
	hub := &fakeHub{connected: []uint{1, 2, 3}}
	rooms := &fakeRoomRepo{
		roomsByStatus: map[string][]room.Room{
			"waiting": {{}, {}},
		},
	}
	h := lobby.NewHandler(hub, &fakeSessions{}, rooms, &fakeUserRepo{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()

	require.NoError(t, h.GetPublicStats(e.NewContext(req, rec)))
	require.Equal(t, http.StatusOK, rec.Code)

	stats := decodePublicStats(t, rec.Body.Bytes())
	assert.Equal(t, 3, stats.Online, "all connected players")
	assert.Equal(t, 2, stats.OpenRooms, "rooms in waiting status")
}

func TestGetPublicStats_Empty(t *testing.T) {
	h := lobby.NewHandler(&fakeHub{}, &fakeSessions{}, &fakeRoomRepo{}, &fakeUserRepo{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()

	require.NoError(t, h.GetPublicStats(e.NewContext(req, rec)))

	stats := decodePublicStats(t, rec.Body.Bytes())
	assert.Equal(t, 0, stats.Online)
	assert.Equal(t, 0, stats.OpenRooms)
}

func TestGetPublicStats_RoomRepoErrorPropagates(t *testing.T) {
	rooms := &fakeRoomRepo{findByStatusErr: errors.New("db down")}
	h := lobby.NewHandler(&fakeHub{connected: []uint{1}}, &fakeSessions{}, rooms, &fakeUserRepo{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()

	assert.Error(t, h.GetPublicStats(e.NewContext(req, rec)))
}
