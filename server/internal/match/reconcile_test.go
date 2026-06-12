package match_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// stubStaleRoomRepo implements match.StaleRoomRepository with only the surface
// area that ReconcileStaleRooms needs.
type stubStaleRoomRepo struct {
	rooms         []match.StaleRoom
	players       map[uint][]match.StaleRoomPlayer
	botSeats      map[uint][]int
	statusUpdates map[uint]string
	findByStatus  func(string) ([]match.StaleRoom, error)
	updateStatus  func(uint, string) error
}

func newStubRoomRepo() *stubStaleRoomRepo {
	return &stubStaleRoomRepo{
		players:       make(map[uint][]match.StaleRoomPlayer),
		botSeats:      make(map[uint][]int),
		statusUpdates: make(map[uint]string),
	}
}

func (s *stubStaleRoomRepo) FindByStatus(status string) ([]match.StaleRoom, error) {
	if s.findByStatus != nil {
		return s.findByStatus(status)
	}
	// StaleRoom doesn't carry Status — "playing" is the only query we
	// exercise. Return everything in the slice as-is (tests only populate
	// rooms with the status they want FindByStatus("playing") to return).
	out := make([]match.StaleRoom, 0, len(s.rooms))
	out = append(out, s.rooms...)
	return out, nil
}

func (s *stubStaleRoomRepo) FindPlayersByRoomID(roomID uint) ([]match.StaleRoomPlayer, error) {
	return s.players[roomID], nil
}

func (s *stubStaleRoomRepo) FindBotSeatsByRoomID(roomID uint) ([]int, error) {
	return s.botSeats[roomID], nil
}

func (s *stubStaleRoomRepo) UpdateStatus(roomID uint, status string) error {
	if s.updateStatus != nil {
		return s.updateStatus(roomID, status)
	}
	s.statusUpdates[roomID] = status
	return nil
}

func intp(i int) *int { return &i }

// Server restart leaves rooms in status="playing" with no live session,
// stranding players because FindPlayerRoom counts "playing" as active and
// blocks them from quick-play / create-room. ReconcileStaleRooms must close
// those rows on boot so the lobby unblocks; for fully-seated rooms it must
// also persist a match row with status="abandoned" so player history matches
// what actually happened.
func TestReconcileStaleRooms_FlipsPlayingToCompleted(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	matchRepo := newMockMatchRepo()
	mgr := match.NewManager(hub, matchRepo)

	stub := newStubRoomRepo()
	// Only "playing" rooms are passed to FindByStatus("playing") — the stub
	// returns whatever is in stub.rooms unconditionally.
	stub.rooms = []match.StaleRoom{
		{ID: 100, Variant: "bitola", MatchMode: "1001"},
		{ID: 101, Variant: "bitola", MatchMode: "1001"},
	}
	stub.players[100] = []match.StaleRoomPlayer{
		{UserID: 10, Seat: intp(0)},
		{UserID: 20, Seat: intp(1)},
		{UserID: 30, Seat: intp(2)},
		{UserID: 40, Seat: intp(3)},
	}
	// Room 101 is short a player — flip status anyway, just skip the match row.
	stub.players[101] = []match.StaleRoomPlayer{
		{UserID: 50, Seat: intp(0)},
		{UserID: 60, Seat: intp(1)},
	}

	err := mgr.ReconcileStaleRooms(stub)
	require.NoError(t, err)

	assert.Equal(t, "completed", stub.statusUpdates[100], "fully-seated room flipped to completed")
	assert.Equal(t, "completed", stub.statusUpdates[101], "under-seated room flipped to completed too")

	matches := matchRepo.getMatches()
	require.Len(t, matches, 1, "only the fully-seated stale room persists a match record")
	assert.Equal(t, uint(100), matches[0].RoomID)
	assert.Equal(t, "abandoned", matches[0].Status)
	require.NotNil(t, matches[0].Player1ID)
	assert.Equal(t, uint(10), *matches[0].Player1ID)
	require.NotNil(t, matches[0].Player4ID)
	assert.Equal(t, uint(40), *matches[0].Player4ID)
	assert.Equal(t, 0, matches[0].WinnerTeam, "no winner for reconciled abandons")
}

// Bot-inclusive stale rooms persist the abandoned record exactly like a live
// bot match end: nil player IDs + per-seat IsBot flags + HasBots — a restart
// must not silently drop bot matches from history while keeping human ones.
func TestReconcileStaleRooms_BotInclusiveRoomPersistsFlaggedRecord(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	matchRepo := newMockMatchRepo()
	mgr := match.NewManager(hub, matchRepo)

	stub := newStubRoomRepo()
	stub.rooms = []match.StaleRoom{
		{ID: 200, Variant: "bitola", MatchMode: "501"},
	}
	stub.players[200] = []match.StaleRoomPlayer{
		{UserID: 10, Seat: intp(0)},
	}
	stub.botSeats[200] = []int{1, 2, 3}

	err := mgr.ReconcileStaleRooms(stub)
	require.NoError(t, err)

	assert.Equal(t, "completed", stub.statusUpdates[200], "bot-inclusive room flipped to completed")

	matches := matchRepo.getMatches()
	require.Len(t, matches, 1, "bot-covered seats count as seated for the abandoned record")
	assert.Equal(t, uint(200), matches[0].RoomID)
	assert.Equal(t, "abandoned", matches[0].Status)
	require.NotNil(t, matches[0].Player1ID)
	assert.Equal(t, uint(10), *matches[0].Player1ID)
	assert.False(t, matches[0].Player1IsBot)
	assert.Nil(t, matches[0].Player2ID, "bot seats persist NULL player IDs")
	assert.Nil(t, matches[0].Player3ID)
	assert.Nil(t, matches[0].Player4ID)
	assert.True(t, matches[0].Player2IsBot)
	assert.True(t, matches[0].Player3IsBot)
	assert.True(t, matches[0].Player4IsBot)
	assert.True(t, matches[0].HasBots)
}

func TestReconcileStaleRooms_NoOpWhenNothingStale(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	matchRepo := newMockMatchRepo()
	mgr := match.NewManager(hub, matchRepo)

	stub := newStubRoomRepo()
	// Empty rooms slice — FindByStatus("playing") returns nothing.
	stub.rooms = []match.StaleRoom{}

	err := mgr.ReconcileStaleRooms(stub)
	require.NoError(t, err)
	assert.Empty(t, stub.statusUpdates, "no status updates when no playing rooms")
	assert.Empty(t, matchRepo.getMatches(), "no match records persisted")
}
