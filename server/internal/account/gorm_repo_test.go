package account_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/account"
	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/match"
)

// testDB opens a transaction on BELJOT_TEST_DB_URL that is rolled back on cleanup.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BELJOT_TEST_DB_URL")
	if dsn == "" {
		t.Skip("BELJOT_TEST_DB_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { tx.Rollback() })
	return tx
}

func suffix() string {
	return fmt.Sprintf("%08d", time.Now().UnixNano()%1e8)
}

func insertID(t *testing.T, db *gorm.DB, sql string, args ...any) uint {
	t.Helper()
	var id uint
	require.NoError(t, db.Raw(sql, args...).Scan(&id).Error)
	require.NotZero(t, id)
	return id
}

func seedUser(t *testing.T, db *gorm.DB, tag string) uint {
	t.Helper()
	return insertID(t, db, `
INSERT INTO users (email, username, password_hash, avatar_key, total_xp, wallet_balance,
                   honor_completed_weight, honor_completed_total, honor_score)
VALUES (?, ?, 'bcrypt-hash', 'avatars/'||?, 900, 7000, 3.5, 4, 90) RETURNING id`, tag+"@erase.test", tag, tag)
}

func seedRoom(t *testing.T, db *gorm.DB, ownerID uint, sfx, tag, status string) uint {
	t.Helper()
	return insertID(t, db, `
INSERT INTO rooms (name, code, owner_id, status, password_hash) VALUES (?, ?, ?, ?, 'room-secret-hash') RETURNING id`,
		"Kafana kaj "+tag+sfx, tag+sfx[len(sfx)-5:], ownerID, status)
}

func exec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	require.NoError(t, db.Exec(sql, args...).Error)
}

func count(t *testing.T, db *gorm.DB, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(sql, args...).Scan(&n).Error)
	return n
}

type playerStats struct {
	Wins, Losses, Abandoned int
	Trend                   match.HonorTrendWindows
	MatchTotal              int64
	Honor                   struct {
		HonorCompletedTotal int64
		HonorAbandonedTotal int64
		HonorScore          int
		TotalXP             int
	}
}

func statsOf(t *testing.T, db *gorm.DB, repo *match.GormMatchRepository, userID uint) playerStats {
	t.Helper()
	var s playerStats
	var err error
	s.Wins, s.Losses, s.Abandoned, err = repo.GetStatsForUser(userID)
	require.NoError(t, err)
	s.Trend, err = repo.GetHonorTrendWindowsForUser(userID, 20)
	require.NoError(t, err)
	_, s.MatchTotal, err = repo.GetMatchesForUser(userID, 50, 0, "", "")
	require.NoError(t, err)
	require.NoError(t, db.Raw(`SELECT honor_completed_total, honor_abandoned_total, honor_score, total_xp FROM users WHERE id = ?`, userID).
		Scan(&s.Honor).Error)
	return s
}

type eraseFixture struct {
	db                   *gorm.DB
	repo                 *account.GormRepository
	matches              *match.GormMatchRepository
	erased, a, b, c      uint
	ownedRoom, otherRoom uint
}

func seedEraseFixture(t *testing.T) eraseFixture {
	t.Helper()
	db := testDB(t)
	sfx := suffix()
	f := eraseFixture{db: db, repo: account.NewGormRepository(db), matches: match.NewGormMatchRepository(db)}
	f.erased = seedUser(t, db, "ex"+sfx)
	f.a = seedUser(t, db, "ea"+sfx)
	f.b = seedUser(t, db, "eb"+sfx)
	f.c = seedUser(t, db, "ec"+sfx)

	exec(t, db, `INSERT INTO friendships (user_id, friend_id, status) VALUES (?, ?, 'accepted'), (?, ?, 'pending'), (?, ?, 'accepted')`,
		f.erased, f.a, f.b, f.erased, f.a, f.c)
	seasonStart := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(time.Now().UnixNano()%1e8) * time.Minute)
	seasonID := insertID(t, db, `INSERT INTO seasons (name, started_at, ends_at) VALUES (?, ?, ?) RETURNING id`,
		"T"+sfx, seasonStart, seasonStart.Add(time.Hour))
	exec(t, db, `INSERT INTO player_seasons (user_id, season_id, sp) VALUES (?, ?, 120), (?, ?, 80)`, f.erased, seasonID, f.a, seasonID)
	exec(t, db, `INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, family_expires_at) VALUES (?, ?, ?, now(), now())`,
		f.erased, "fam"+sfx, "hash"+sfx)
	exec(t, db, `INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES (?, ?, now())`, f.erased, "reset"+sfx)
	exec(t, db, `INSERT INTO user_identities (user_id, provider, provider_user_id, email) VALUES (?, 'google', ?, ?)`,
		f.erased, "sub"+sfx, "ex"+sfx+"@gmail.test")

	f.ownedRoom = seedRoom(t, db, f.erased, sfx, "X", "completed")
	f.otherRoom = seedRoom(t, db, f.a, sfx, "A", "completed")
	exec(t, db, `INSERT INTO room_players (room_id, user_id, seat, team) VALUES (?, ?, 0, 'A')`, f.ownedRoom, f.erased)

	base := time.Now().Add(-48 * time.Hour)
	newMatch := func(offset time.Duration, roomID uint, status string, winner int, abandonedBy, surrenderedBy *uint) {
		x, a, b, c := f.erased, f.a, f.b, f.c
		require.NoError(t, f.matches.Create(&match.Match{
			RoomID: roomID, Player1ID: &x, Player2ID: &a, Player3ID: &b, Player4ID: &c,
			TeamAScore: 1001, TeamBScore: 700, WinnerTeam: winner, Variant: "bitola", MatchMode: "1001",
			StartedAt: base.Add(offset - 30*time.Minute), CompletedAt: base.Add(offset),
			Status: status, AbandonedBy: abandonedBy, SurrenderedBy: surrenderedBy,
		}))
	}
	erasedID := f.erased
	aID := f.a
	newMatch(0, f.ownedRoom, "completed", 0, nil, nil)
	newMatch(time.Hour, f.otherRoom, "abandoned", 1, &erasedID, nil)
	newMatch(2*time.Hour, f.otherRoom, "completed", 1, nil, &erasedID)
	newMatch(3*time.Hour, f.otherRoom, "abandoned", 0, &aID, nil)
	return f
}

func TestErase_AnonymisesInPlaceAndKeepsOpponentsStats(t *testing.T) {
	f := seedEraseFixture(t)
	opponents := []uint{f.a, f.b, f.c}
	before := map[uint]playerStats{}
	for _, id := range opponents {
		before[id] = statsOf(t, f.db, f.matches, id)
	}
	require.Equal(t, 4, before[f.b].Wins+before[f.b].Losses, "fixture sanity: every match, including the erased user's abandonment, is a result for b")

	erased, err := f.repo.Erase(f.erased)
	require.NoError(t, err)
	require.NotNil(t, erased.AvatarKey)
	assert.Contains(t, *erased.AvatarKey, "avatars/ex")

	for _, table := range []string{"refresh_tokens", "password_reset_tokens", "user_identities", "player_seasons", "room_players"} {
		assert.Zero(t, count(t, f.db, `SELECT COUNT(*) FROM `+table+` WHERE user_id = ?`, f.erased), table)
	}
	assert.Zero(t, count(t, f.db, `SELECT COUNT(*) FROM friendships WHERE user_id = ? OR friend_id = ?`, f.erased, f.erased))
	assert.Equal(t, int64(1), count(t, f.db, `SELECT COUNT(*) FROM friendships WHERE user_id = ? AND friend_id = ?`, f.a, f.c),
		"friendships between other players stay")
	assert.Equal(t, int64(1), count(t, f.db, `SELECT COUNT(*) FROM player_seasons WHERE user_id = ?`, f.a))

	var u struct {
		Email, Username, PasswordHash string
		AvatarKey                     *string
		TotalXP, WalletBalance        int
		HonorCompletedTotal           int64
		HonorScore                    int
		DeletedAt                     *time.Time
	}
	require.NoError(t, f.db.Raw(`SELECT email, username, password_hash, avatar_key, total_xp, wallet_balance,
		honor_completed_total, honor_score, deleted_at FROM users WHERE id = ?`, f.erased).Scan(&u).Error)
	assert.Equal(t, fmt.Sprintf("erased+%d@invalid", f.erased), u.Email)
	assert.Equal(t, fmt.Sprintf("erased_%d", f.erased), u.Username)
	assert.Empty(t, u.PasswordHash)
	assert.Nil(t, u.AvatarKey)
	assert.Zero(t, u.TotalXP)
	assert.Zero(t, u.WalletBalance)
	assert.Zero(t, u.HonorCompletedTotal)
	assert.Equal(t, 80, u.HonorScore)
	assert.NotNil(t, u.DeletedAt)

	var room struct {
		Name         string
		PasswordHash *string
	}
	require.NoError(t, f.db.Raw(`SELECT name, password_hash FROM rooms WHERE id = ?`, f.ownedRoom).Scan(&room).Error)
	assert.Equal(t, fmt.Sprintf("room-%d", f.ownedRoom), room.Name)
	assert.Nil(t, room.PasswordHash)
	require.NoError(t, f.db.Raw(`SELECT name, password_hash FROM rooms WHERE id = ?`, f.otherRoom).Scan(&room).Error)
	assert.NotEqual(t, fmt.Sprintf("room-%d", f.otherRoom), room.Name, "rooms of other owners keep their names")

	assert.Equal(t, int64(1), count(t, f.db, `SELECT COUNT(*) FROM matches WHERE abandoned_by = ?`, f.erased),
		"abandoned_by must keep the tombstone id, never NULL")
	assert.Equal(t, int64(4), count(t, f.db, `SELECT COUNT(*) FROM matches WHERE player1_id = ?`, f.erased))

	for _, id := range opponents {
		assert.Equal(t, before[id], statsOf(t, f.db, f.matches, id), "opponent %d stats changed", id)
	}

	_, err = f.repo.Erase(f.erased)
	assert.ErrorIs(t, err, apperr.ErrUserNotFound, "an erased user cannot be erased twice")
}

func TestErase_RefusedWhileSeatedInAnActiveRoom(t *testing.T) {
	for _, status := range []string{"waiting", "playing"} {
		t.Run(status, func(t *testing.T) {
			f := seedEraseFixture(t)
			active := seedRoom(t, f.db, f.a, suffix(), "W", status)
			require.NoError(t, f.db.Exec(`INSERT INTO room_players (room_id, user_id) VALUES (?, ?)`, active, f.erased).Error)

			seated, err := f.repo.InActiveRoom(f.erased)
			require.NoError(t, err)
			assert.True(t, seated)

			_, err = f.repo.Erase(f.erased)
			assert.ErrorIs(t, err, apperr.ErrAccountDeleteInRoom)
			assert.Equal(t, int64(1), count(t, f.db, `SELECT COUNT(*) FROM user_identities WHERE user_id = ?`, f.erased),
				"a refused erase changes nothing")
			assert.Equal(t, int64(1), count(t, f.db, `SELECT COUNT(*) FROM users WHERE id = ? AND deleted_at IS NULL`, f.erased))
		})
	}
}

func TestInActiveRoom_FalseForCompletedRooms(t *testing.T) {
	f := seedEraseFixture(t)
	seated, err := f.repo.InActiveRoom(f.erased)
	require.NoError(t, err)
	assert.False(t, seated)
}
