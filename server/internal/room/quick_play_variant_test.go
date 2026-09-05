package room_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

// TestFindQuickPlayRoomExcluding_SkipsNonCroatianRooms executes the REAL query
// behind Quick Play's Croatian-only limit.
//
// The limit has two halves and only one of them is otherwise verified: the
// mechanism — QuickPlay's room synthesis hardcoding Variant "croatia" — and the
// backstop, an `AND variant = ?` predicate in
// GormRepository.FindQuickPlayRoomExcluding. The backstop's only other coverage
// is mockRoomRepo restating the same condition in Go, which is a copy of the
// predicate rather than a test of it: deleting the SQL clause left every test
// green.
//
// DB-backed and skipped when no database is reachable, per the wallet/friend/
// room integration-test convention; the transaction rolls back on cleanup.
func TestFindQuickPlayRoomExcluding_SkipsNonCroatianRooms(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "qpvariant@room.test", Username: "qpvowner", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	const buyIn = 500

	// A Bitola quick-play row: matches every other predicate, so only the
	// variant clause can exclude it.
	bitola := &room.Room{
		Name: "QP Bitola", Code: "QPBIT1", OwnerID: owner.ID,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "per-move",
		Status: "waiting", PlayerCount: 1, IsQuickPlay: true, CoinBuyIn: buyIn,
	}
	require.NoError(t, repo.Create(bitola))

	got, err := repo.FindQuickPlayRoomExcluding(nil, buyIn)
	require.NoError(t, err)
	if got != nil {
		// The dev database is shared, so another Croatian quick-play row may
		// legitimately match. What must never happen is matching THIS one.
		assert.NotEqual(t, bitola.ID, got.ID,
			"matchmaking must not seat a player into a Bitola quick-play room")
		assert.Equal(t, "croatia", got.Variant,
			"Quick Play is Croatian-only — the query must never return another variant")
	}

	// Guard the guard: an otherwise identical Croatian row IS matchable, so the
	// assertion above cannot be passing merely because some unrelated predicate
	// excludes both rows.
	croatian := &room.Room{
		Name: "QP Croatian", Code: "QPCRO1", OwnerID: owner.ID,
		Variant: "croatia", MatchMode: "501", TimerStyle: "per-move",
		Status: "waiting", PlayerCount: 1, IsQuickPlay: true, CoinBuyIn: buyIn,
	}
	require.NoError(t, repo.Create(croatian))

	// Exclude everything the shared database might already hold except the two
	// rows this test created, so the result is deterministic.
	var otherIDs []uint
	require.NoError(t, db.Model(&room.Room{}).
		Where("is_quick_play = ? AND id NOT IN ?", true, []uint{croatian.ID, bitola.ID}).
		Pluck("id", &otherIDs).Error)
	excluded := make(map[uint]bool, len(otherIDs))
	for _, id := range otherIDs {
		excluded[id] = true
	}

	got, err = repo.FindQuickPlayRoomExcluding(excluded, buyIn)
	require.NoError(t, err)
	require.NotNil(t, got, "the Croatian row must be matchable")
	assert.Equal(t, croatian.ID, got.ID,
		"with only these two rows in scope the query must pick the Croatian one, never the Bitola one")
}
