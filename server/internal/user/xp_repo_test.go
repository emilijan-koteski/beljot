package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// seedXPUser inserts a user with a known starting total_xp inside the test
// transaction (rolled back by getTestDB's cleanup). Email/username are unique
// per call so multiple seeds in one test don't collide on the unique indexes.
func seedXPUser(t *testing.T, repo *GormUserRepository, username string, startXP int) *User {
	t.Helper()
	u := &User{
		Email:              username + "@xp.test",
		Username:           username,
		PasswordHash:       "x",
		LanguagePreference: "en",
		TotalXP:            startXP,
	}
	require.NoError(t, repo.Create(u))
	return u
}

func TestGormUserRepository_AddXP_AddsAndReturnsNewTotals(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	a := seedXPUser(t, repo, "xp_add_a", 100)
	b := seedXPUser(t, repo, "xp_add_b", 0)

	newTotals, err := repo.AddXP(map[uint]int{a.ID: 50, b.ID: 12})
	require.NoError(t, err)

	assert.Equal(t, 150, newTotals[a.ID])
	assert.Equal(t, 12, newTotals[b.ID])

	// Persisted values reflect the additions.
	reloadedA, err := repo.FindByID(a.ID)
	require.NoError(t, err)
	assert.Equal(t, 150, reloadedA.TotalXP)
	reloadedB, err := repo.FindByID(b.ID)
	require.NoError(t, err)
	assert.Equal(t, 12, reloadedB.TotalXP)
}

func TestGormUserRepository_AddXP_SkipsZeroDeltas(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	a := seedXPUser(t, repo, "xp_zero_a", 200)
	b := seedXPUser(t, repo, "xp_zero_b", 7)

	newTotals, err := repo.AddXP(map[uint]int{a.ID: 0, b.ID: 3})
	require.NoError(t, err)

	// Zero-delta user is neither updated nor returned.
	_, present := newTotals[a.ID]
	assert.False(t, present, "zero-delta user must be skipped")
	assert.Equal(t, 10, newTotals[b.ID])

	reloadedA, err := repo.FindByID(a.ID)
	require.NoError(t, err)
	assert.Equal(t, 200, reloadedA.TotalXP, "zero-delta user unchanged")
}

func TestGormUserRepository_AddXP_EmptyMapIsNoOp(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	newTotals, err := repo.AddXP(map[uint]int{})
	require.NoError(t, err)
	assert.Empty(t, newTotals)
}

func TestGormUserRepository_AddXP_MissingUserRollsBack(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	existing := seedXPUser(t, repo, "xp_missing_existing", 500)
	const missingID = uint(999999)

	// The existing (lower) ID is locked + would be updated first; the missing
	// (higher) ID then fails, so the whole transaction must roll back.
	_, err := repo.AddXP(map[uint]int{existing.ID: 40, missingID: 40})
	require.ErrorIs(t, err, apperr.ErrUserNotFound)

	reloaded, err := repo.FindByID(existing.ID)
	require.NoError(t, err)
	assert.Equal(t, 500, reloaded.TotalXP, "existing user must be unchanged after rollback")
}
