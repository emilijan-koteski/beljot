package room_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/ws"
)

// mockRepoForLobby is a minimal mock of RoomRepository for lobby disconnect tests.
type mockRepoForLobby struct {
	rooms       map[uint]*room.Room
	players     map[uint][]room.RoomPlayer // roomID → players
	playerRooms map[uint]*room.RoomPlayer  // userID → RoomPlayer
	removed     []uint                     // userIDs that were removed
}

func newMockRepoForLobby() *mockRepoForLobby {
	return &mockRepoForLobby{
		rooms:       make(map[uint]*room.Room),
		players:     make(map[uint][]room.RoomPlayer),
		playerRooms: make(map[uint]*room.RoomPlayer),
	}
}

func (r *mockRepoForLobby) FindPlayerRoom(userID uint) (*room.RoomPlayer, error) {
	rp := r.playerRooms[userID]
	return rp, nil
}

func (r *mockRepoForLobby) FindByID(id uint) (*room.Room, error) {
	rm := r.rooms[id]
	return rm, nil
}

func (r *mockRepoForLobby) FindByIDForUpdate(id uint) (*room.Room, error) {
	return r.FindByID(id)
}

func (r *mockRepoForLobby) FindPlayersByRoomID(roomID uint) ([]room.RoomPlayer, error) {
	return r.players[roomID], nil
}

func (r *mockRepoForLobby) RemovePlayer(roomID uint, userID uint) error {
	r.removed = append(r.removed, userID)
	// Remove from playerRooms and players list
	delete(r.playerRooms, userID)
	if ps, ok := r.players[roomID]; ok {
		var remaining []room.RoomPlayer
		for _, p := range ps {
			if p.UserID != userID {
				remaining = append(remaining, p)
			}
		}
		r.players[roomID] = remaining
	}
	return nil
}

func (r *mockRepoForLobby) DecrementPlayerCount(roomID uint) error {
	if rm, ok := r.rooms[roomID]; ok {
		rm.PlayerCount--
	}
	return nil
}

func (r *mockRepoForLobby) RunInTransaction(fn func(room.RoomRepository) error) error {
	return fn(r)
}

func (r *mockRepoForLobby) LoadOwnerUsernames(_ []*room.Room) error { return nil }
func (r *mockRepoForLobby) FindPlayersByRoomIDs(_ []uint) (map[uint][]room.RoomPlayer, error) {
	return map[uint][]room.RoomPlayer{}, nil
}

// Unused but required by interface
func (r *mockRepoForLobby) Create(_ *room.Room) error                                { return nil }
func (r *mockRepoForLobby) Update(rm *room.Room) error                               { r.rooms[rm.ID] = rm; return nil }
func (r *mockRepoForLobby) FindByCode(_ string) (*room.Room, error)                  { return nil, nil }
func (r *mockRepoForLobby) FindByStatus(_ string) ([]room.Room, error)               { return nil, nil }
func (r *mockRepoForLobby) AddPlayer(_ *room.RoomPlayer) error                       { return nil }
func (r *mockRepoForLobby) IncrementPlayerCount(_ uint) error                        { return nil }
func (r *mockRepoForLobby) UpdatePlayerSeat(_ uint, _ uint, _ int, _ string) error   { return nil }
func (r *mockRepoForLobby) ClearPlayerSeat(_ uint, _ uint) error                     { return nil }
func (r *mockRepoForLobby) FindPlayerBySeat(_ uint, _ int) (*room.RoomPlayer, error) { return nil, nil }
func (r *mockRepoForLobby) FindQuickPlayRoom(_ int) (*room.Room, error)              { return nil, nil }
func (r *mockRepoForLobby) FindQuickPlayRoomExcluding(_ map[uint]bool, _ int) (*room.Room, error) {
	return nil, nil
}
func (r *mockRepoForLobby) UpdateStatus(_ uint, _ string) error              { return nil }
func (r *mockRepoForLobby) FindUserIDsByRoomStatus(_ string) ([]uint, error) { return nil, nil }
func (r *mockRepoForLobby) AddBot(_ uint, _ int) error                       { return nil }
func (r *mockRepoForLobby) RemoveBot(_ uint, _ int) error                    { return nil }
func (r *mockRepoForLobby) UpdateBotSeat(_ uint, _ int, _ int) error         { return nil }
func (r *mockRepoForLobby) FindBotsByRoomID(_ uint) ([]room.RoomBot, error)  { return nil, nil }
func (r *mockRepoForLobby) FindBotsByRoomIDs(_ []uint) (map[uint][]room.RoomBot, error) {
	return map[uint][]room.RoomBot{}, nil
}

func setupLobbyTest() (*mockRepoForLobby, *room.LobbyDisconnectHandler) {
	repo := newMockRepoForLobby()
	hub := ws.NewHub()
	go hub.Run()
	handler := room.NewLobbyDisconnectHandler(repo, hub, nil)
	return repo, handler
}

func TestLobbyDisconnect_FreesAfterTimeout(t *testing.T) {
	repo, handler := setupLobbyTest()

	// Set up a player in a waiting room
	repo.rooms[1] = &room.Room{ID: 1, Status: "waiting", OwnerID: 100, PlayerCount: 2}
	repo.playerRooms[42] = &room.RoomPlayer{RoomID: 1, UserID: 42, Username: "TestPlayer"}
	repo.players[1] = []room.RoomPlayer{
		{RoomID: 1, UserID: 100, Username: "Owner"},
		{RoomID: 1, UserID: 42, Username: "TestPlayer"},
	}

	handler.HandleDisconnect(42)

	// Before timeout — player should still be in the room
	time.Sleep(5 * time.Second)
	assert.Len(t, repo.removed, 0, "player should not be removed yet (before 10s timeout)")

	// After timeout — player should be removed
	time.Sleep(6 * time.Second)
	assert.Contains(t, repo.removed, uint(42), "player should be removed after 10s timeout")
}

func TestLobbyDisconnect_CancelledByReconnect(t *testing.T) {
	repo, handler := setupLobbyTest()

	repo.rooms[1] = &room.Room{ID: 1, Status: "waiting", OwnerID: 100, PlayerCount: 2}
	repo.playerRooms[42] = &room.RoomPlayer{RoomID: 1, UserID: 42, Username: "TestPlayer"}
	repo.players[1] = []room.RoomPlayer{
		{RoomID: 1, UserID: 100, Username: "Owner"},
		{RoomID: 1, UserID: 42, Username: "TestPlayer"},
	}

	handler.HandleDisconnect(42)
	time.Sleep(3 * time.Second) // Reconnect before timeout

	handler.HandleReconnect(42)

	// Wait past the original timeout
	time.Sleep(8 * time.Second)
	assert.Len(t, repo.removed, 0, "player should NOT be removed (reconnected before timeout)")
}

func TestLobbyDisconnect_NoOpForPlayingRoom(t *testing.T) {
	repo, handler := setupLobbyTest()

	// Room is in "playing" status — session manager handles this, not lobby handler
	repo.rooms[1] = &room.Room{ID: 1, Status: "playing", OwnerID: 100, PlayerCount: 4}
	repo.playerRooms[42] = &room.RoomPlayer{RoomID: 1, UserID: 42}

	handler.HandleDisconnect(42)
	time.Sleep(100 * time.Millisecond)

	// No timer should fire — the handler returns immediately for playing rooms
	time.Sleep(11 * time.Second)
	assert.Len(t, repo.removed, 0, "player should NOT be removed from a playing room")
}

func TestLobbyDisconnect_NoOpForPlayerNotInRoom(t *testing.T) {
	_, handler := setupLobbyTest()

	// User 99 is not in any room
	handler.HandleDisconnect(99)
	time.Sleep(100 * time.Millisecond)

	// Should not crash, no timer started
	handler.HandleReconnect(99)
	// No assertions needed — just verifying no panic
}
