package user

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func getTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("BELJOT_DB_URL")
	if dsn == "" {
		dsn = "postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skip("skipping integration test: database not available")
	}

	// Use a transaction that will be rolled back after the test
	tx := db.Begin()
	t.Cleanup(func() {
		tx.Rollback()
	})

	return tx
}

func TestGormUserRepository_Create(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := &User{
		Email:              "create@test.com",
		Username:           "createuser",
		PasswordHash:       "hashedpassword",
		LanguagePreference: "en",
	}

	err := repo.Create(u)
	require.NoError(t, err)
	assert.NotZero(t, u.ID)
	assert.NotZero(t, u.CreatedAt)
}

func TestGormUserRepository_FindByEmail_Found(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := &User{
		Email:              "find@test.com",
		Username:           "finduser",
		PasswordHash:       "hashedpassword",
		LanguagePreference: "en",
	}
	require.NoError(t, repo.Create(u))

	found, err := repo.FindByEmail("find@test.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, found.ID)
	assert.Equal(t, "find@test.com", found.Email)
}

func TestGormUserRepository_FindByEmail_NotFound(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	found, err := repo.FindByEmail("nonexistent@test.com")
	assert.NoError(t, err)
	assert.Nil(t, found)
}

func TestGormUserRepository_FindByUsername_Found(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := &User{
		Email:              "username@test.com",
		Username:           "findbyname",
		PasswordHash:       "hashedpassword",
		LanguagePreference: "en",
	}
	require.NoError(t, repo.Create(u))

	found, err := repo.FindByUsername("findbyname")
	require.NoError(t, err)
	assert.Equal(t, u.ID, found.ID)
	assert.Equal(t, "findbyname", found.Username)
}

func TestGormUserRepository_FindByUsername_NotFound(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	found, err := repo.FindByUsername("nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, found)
}

// TestGormUserRepository_SearchByUsername exercises the Story 11.1 ILIKE search
// against a real Postgres inside the rolled-back test transaction. Everything
// runs off one shared seed so the wildcard-escape and ordering guarantees are
// checked against the same population.
func TestGormUserRepository_SearchByUsername(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	// Every seeded username carries this prefix, and every query below searches
	// for it. getTestDB hands out a rolled-back transaction, which isolates this
	// test from OTHER TESTS but not from rows already in the database it runs
	// against — a developer DB holds real accounts, and a genuine "alice" made
	// the exact-set assertions below fail. Namespacing the fixtures means no
	// real username can land in a result set, so the assertions can stay exact
	// (and therefore keep proving escaping and ordering) instead of being
	// loosened to "contains".
	const pfx = "sbufx_"

	// mk inserts a user with a unique email derived from the username (email is
	// itself uniquely indexed). Usernames here intentionally bypass the app-level
	// [a-zA-Z0-9_] charset (there is no DB CHECK) so the % / _ literal-match
	// probes can be seeded at all.
	mk := func(username string) *User {
		u := &User{
			Email:              username + "@search.test",
			Username:           pfx + username,
			PasswordHash:       "x",
			LanguagePreference: "en",
		}
		require.NoError(t, repo.Create(u))
		return u
	}

	searcher := mk("aliSearcher") // contains "ali" — proves self-exclusion
	mk("Alice")                   // case-insensitive: found by "ali"
	mk("alicia")                  // substring: found by "ali"
	bob := mk("Bob")              // control: never matches "ali" or a literal "%"
	mk("car_ol")                  // literal underscore probe
	mk("carXol")                  // must NOT match a "car_ol" query if _ were a wildcard
	mk("pct%name")                // literal percent probe
	mk("zzalpha")
	mk("zzbeta")
	mk("zzgamma")
	ghost := mk("ghostali") // soft-deleted below — must never appear in "ali"
	require.NoError(t, repo.Delete(ghost.ID))

	usernames := func(us []User) []string {
		out := make([]string, len(us))
		for i, u := range us {
			out[i] = u.Username
		}
		return out
	}

	t.Run("case-insensitive substring, self + soft-deleted excluded", func(t *testing.T) {
		res, err := repo.SearchByUsername(pfx+"ali", searcher.ID, 10)
		require.NoError(t, err)
		// "Alice" + "alicia"; NOT the searcher (self), NOT "ghostali" (soft-deleted).
		names := usernames(res)
		assert.ElementsMatch(t, []string{pfx + "Alice", pfx + "alicia"}, names)
		assert.NotContains(t, names, pfx+"aliSearcher")
		assert.NotContains(t, names, pfx+"ghostali")
	})

	t.Run("self is excluded even on an exact match", func(t *testing.T) {
		res, err := repo.SearchByUsername(pfx+"aliSearcher", searcher.ID, 10)
		require.NoError(t, err)
		assert.Empty(t, res)
	})

	t.Run("underscore matches literally, not as a wildcard", func(t *testing.T) {
		res, err := repo.SearchByUsername(pfx+"car_ol", searcher.ID, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{pfx + "car_ol"}, usernames(res),
			"an unescaped _ would also match carXol")
	})

	t.Run("percent matches literally, not as match-everything", func(t *testing.T) {
		// This one query cannot be prefix-scoped: the whole point is to pass a
		// bare "%" and prove it is not treated as match-everything. So assert
		// on the seeded control instead of on an exact set — Bob has no "%" in
		// his name and would be returned if the wildcard escaped the escaping.
		res, err := repo.SearchByUsername("%", searcher.ID, 10)
		require.NoError(t, err)
		names := usernames(res)
		assert.Contains(t, names, pfx+"pct%name", "the literal-% user must still be found")
		assert.NotContains(t, names, pfx+"Bob", "an unescaped % would return every user")
		assert.NotContains(t, names, bob.Username)
	})

	t.Run("results are ordered by username ascending", func(t *testing.T) {
		res, err := repo.SearchByUsername(pfx+"zz", searcher.ID, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{pfx + "zzalpha", pfx + "zzbeta", pfx + "zzgamma"}, usernames(res))
	})

	t.Run("limit caps the result count", func(t *testing.T) {
		res, err := repo.SearchByUsername(pfx+"zz", searcher.ID, 2)
		require.NoError(t, err)
		assert.Equal(t, []string{pfx + "zzalpha", pfx + "zzbeta"}, usernames(res))
	})
}
