package room_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

// TestFindQuickPlayRoomExcluding_SkipsNonCroatianRooms runs the real `AND variant = ?`
// query against a database, skipping when none is reachable.
func TestFindQuickPlayRoomExcluding_SkipsNonCroatianRooms(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	owner := &user.User{Email: "qpvariant@room.test", Username: "qpvowner", PasswordHash: "x"}
	require.NoError(t, db.Create(owner).Error)

	const buyIn = 500

	bitola := &room.Room{
		Name: "QP Bitola", Code: "QPBIT1", OwnerID: owner.ID,
		Variant: "bitola", MatchMode: "1001", TimerStyle: "per-move",
		Status: "waiting", PlayerCount: 1, IsQuickPlay: true, CoinBuyIn: buyIn,
	}
	require.NoError(t, repo.Create(bitola))

	got, err := repo.FindQuickPlayRoomExcluding(nil, buyIn)
	require.NoError(t, err)
	if got != nil {
		assert.NotEqual(t, bitola.ID, got.ID,
			"matchmaking must not seat a player into a Bitola quick-play room")
		assert.Equal(t, "croatia", got.Variant,
			"Quick Play is Croatian-only — the query must never return another variant")
	}

	croatian := &room.Room{
		Name: "QP Croatian", Code: "QPCRO1", OwnerID: owner.ID,
		Variant: "croatia", MatchMode: "501", TimerStyle: "per-move",
		Status: "waiting", PlayerCount: 1, IsQuickPlay: true, CoinBuyIn: buyIn,
	}
	require.NoError(t, repo.Create(croatian))

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
