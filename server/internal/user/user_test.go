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

	// mk inserts a user with a unique email derived from the username (email is
	// itself uniquely indexed). Usernames here intentionally bypass the app-level
	// [a-zA-Z0-9_] charset (there is no DB CHECK) so the % / _ literal-match
	// probes can be seeded at all.
	mk := func(username string) *User {
		u := &User{
			Email:              username + "@search.test",
			Username:           username,
			PasswordHash:       "x",
			LanguagePreference: "en",
		}
		require.NoError(t, repo.Create(u))
		return u
	}

	searcher := mk("aliSearcher") // contains "ali" — proves self-exclusion
	mk("Alice")                   // case-insensitive: found by "ali"
	mk("alicia")                  // substring: found by "ali"
	mk("Bob")                     // control: never matches "ali"
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
		res, err := repo.SearchByUsername("ali", searcher.ID, 10)
		require.NoError(t, err)
		// "Alice" + "alicia"; NOT the searcher (self), NOT "ghostali" (soft-deleted).
		names := usernames(res)
		assert.ElementsMatch(t, []string{"Alice", "alicia"}, names)
		assert.NotContains(t, names, "aliSearcher")
		assert.NotContains(t, names, "ghostali")
	})

	t.Run("self is excluded even on an exact match", func(t *testing.T) {
		res, err := repo.SearchByUsername("aliSearcher", searcher.ID, 10)
		require.NoError(t, err)
		assert.Empty(t, res)
	})

	t.Run("underscore matches literally, not as a wildcard", func(t *testing.T) {
		res, err := repo.SearchByUsername("car_ol", searcher.ID, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"car_ol"}, usernames(res),
			"an unescaped _ would also match carXol")
	})

	t.Run("percent matches literally, not as match-everything", func(t *testing.T) {
		res, err := repo.SearchByUsername("%", searcher.ID, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"pct%name"}, usernames(res),
			"an unescaped % would return every user")
	})

	t.Run("results are ordered by username ascending", func(t *testing.T) {
		res, err := repo.SearchByUsername("zz", searcher.ID, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"zzalpha", "zzbeta", "zzgamma"}, usernames(res))
	})

	t.Run("limit caps the result count", func(t *testing.T) {
		res, err := repo.SearchByUsername("zz", searcher.ID, 2)
		require.NoError(t, err)
		assert.Equal(t, []string{"zzalpha", "zzbeta"}, usernames(res))
	})
}
