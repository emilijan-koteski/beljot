package user

import (
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/apperr"
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

// --- Audio preferences (migration 000025) ---

// Registration never names the two switches, so this is the path every new
// account takes: a zero-value false must land as the column's TRUE default,
// both in the row and in the struct the auth handler echoes.
func TestGormUserRepository_Create_AudioPreferencesDefaultOn(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := &User{
		Email:              "audio-default@test.com",
		Username:           "audiodefault",
		PasswordHash:       "hashedpassword",
		LanguagePreference: "en",
	}
	require.NoError(t, repo.Create(u))
	assert.True(t, u.SoundEnabled)
	assert.True(t, u.MusicEnabled)

	reloaded, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.SoundEnabled)
	assert.True(t, reloaded.MusicEnabled)
}

// The migration's column DEFAULT is what every pre-existing row and every
// non-GORM writer relies on. GORM's Create sends the tag default explicitly, so
// the test above never exercises it: insert a row naming neither column in raw
// SQL, the way a row that predates migration 000025 was written, and read both.
func TestGormUserRepository_AudioPreferencesDefaultTrueOnRawInsert(t *testing.T) {
	db := getTestDB(t)

	require.NoError(t, db.Exec(
		`INSERT INTO users (email, username, password_hash, created_at, updated_at)
		 VALUES ('audio-raw@test.com', 'audioraw', 'x', NOW(), NOW())`,
	).Error)

	var got struct {
		SoundEnabled bool
		MusicEnabled bool
	}
	require.NoError(t, db.Raw(
		"SELECT sound_enabled, music_enabled FROM users WHERE username = 'audioraw'",
	).Scan(&got).Error)
	assert.True(t, got.SoundEnabled, "a row inserted without sound_enabled must land on TRUE")
	assert.True(t, got.MusicEnabled, "a row inserted without music_enabled must land on TRUE")
}

// false is a real value on the preferences path: the map-based Updates must
// write it rather than skip it as a zero value, and must leave every column the
// update did not name exactly as it was.
func TestGormUserRepository_UpdatePreferences_AudioSwitches(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := &User{
		Email:              "audio-update@test.com",
		Username:           "audioupdate",
		PasswordHash:       "hashedpassword",
		LanguagePreference: "mk",
	}
	require.NoError(t, repo.Create(u))

	off, on := false, true
	require.NoError(t, repo.UpdatePreferences(u.ID, PreferencesUpdate{SoundEnabled: &off}))
	reloaded, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.False(t, reloaded.SoundEnabled)
	assert.True(t, reloaded.MusicEnabled, "a sound-only update must not touch music")
	assert.Equal(t, "mk", reloaded.LanguagePreference)
	assert.Equal(t, CardDeckFrench, reloaded.CardDeckPreference)

	require.NoError(t, repo.UpdatePreferences(u.ID, PreferencesUpdate{SoundEnabled: &on, MusicEnabled: &off}))
	reloaded, err = repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.SoundEnabled)
	assert.False(t, reloaded.MusicEnabled)
}

func TestGormUserRepository_UpdatePreferences_EmptyAndMissing(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	assert.ErrorIs(t, repo.UpdatePreferences(1, PreferencesUpdate{}), apperr.ErrBadRequest)

	off := false
	assert.ErrorIs(t, repo.UpdatePreferences(999999, PreferencesUpdate{MusicEnabled: &off}), apperr.ErrUserNotFound)
}

// --- Audio volumes (migration 000026) ---

// Registration never names the two volumes, so this is the path every new
// account takes: a zero value must land as the tag default of 70, both in the
// row and in the struct the auth handler echoes.
func TestGormUserRepository_Create_AudioVolumesDefault70(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := &User{
		Email:              "volume-default@test.com",
		Username:           "volumedefault",
		PasswordHash:       "hashedpassword",
		LanguagePreference: "en",
	}
	require.NoError(t, repo.Create(u))
	assert.Equal(t, 70, u.SoundVolume)
	assert.Equal(t, 70, u.MusicVolume)

	reloaded, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, 70, reloaded.SoundVolume)
	assert.Equal(t, 70, reloaded.MusicVolume)
}

// The migration's column DEFAULT is what every pre-existing row and every
// non-GORM writer relies on, and GORM's Create sends the tag default itself, so
// only a raw insert naming neither column exercises it.
func TestGormUserRepository_AudioVolumesDefault70OnRawInsert(t *testing.T) {
	db := getTestDB(t)

	require.NoError(t, db.Exec(
		`INSERT INTO users (email, username, password_hash, created_at, updated_at)
		 VALUES ('volume-raw@test.com', 'volumeraw', 'x', NOW(), NOW())`,
	).Error)

	var got struct {
		SoundVolume int
		MusicVolume int
	}
	require.NoError(t, db.Raw(
		"SELECT sound_volume, music_volume FROM users WHERE username = 'volumeraw'",
	).Scan(&got).Error)
	assert.Equal(t, 70, got.SoundVolume, "a row inserted without sound_volume must land on 70")
	assert.Equal(t, 70, got.MusicVolume, "a row inserted without music_volume must land on 70")
}

// 0 is a real value on the preferences path: the map-based Updates must write
// it rather than skip it as a zero value, and must leave every column the
// update did not name exactly as it was.
func TestGormUserRepository_UpdatePreferences_AudioVolumes(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := &User{
		Email:              "volume-update@test.com",
		Username:           "volumeupdate",
		PasswordHash:       "hashedpassword",
		LanguagePreference: "mk",
	}
	require.NoError(t, repo.Create(u))

	zero, full := 0, 100
	require.NoError(t, repo.UpdatePreferences(u.ID, PreferencesUpdate{SoundVolume: &zero}))
	reloaded, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, reloaded.SoundVolume)
	assert.Equal(t, 70, reloaded.MusicVolume, "a sound-only update must not touch music")
	assert.True(t, reloaded.SoundEnabled, "a volume update must not touch the switches")
	assert.True(t, reloaded.MusicEnabled)
	assert.Equal(t, "mk", reloaded.LanguagePreference)
	assert.Equal(t, CardDeckFrench, reloaded.CardDeckPreference)

	require.NoError(t, repo.UpdatePreferences(u.ID, PreferencesUpdate{SoundVolume: &full, MusicVolume: &zero}))
	reloaded, err = repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, 100, reloaded.SoundVolume)
	assert.Equal(t, 0, reloaded.MusicVolume)
}

// The column CHECKs are the backstop for any writer that skips the handler's
// validation: each volume column must refuse a level outside 0-100 with a
// Postgres check violation (SQLSTATE 23514) on its own constraint. One subtest
// per case, each in its own transaction: the failed statement aborts it.
func TestGormUserRepository_UpdatePreferences_AudioVolumeOutOfRangeRejectedByDB(t *testing.T) {
	cases := []struct {
		name       string
		value      int
		music      bool
		constraint string
	}{
		{name: "sound above 100", value: 101, constraint: "users_sound_volume_check"},
		{name: "sound below 0", value: -1, constraint: "users_sound_volume_check"},
		{name: "music above 100", value: 101, music: true, constraint: "users_music_volume_check"},
		{name: "music below 0", value: -1, music: true, constraint: "users_music_volume_check"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := getTestDB(t)
			repo := NewGormUserRepository(db)

			u := &User{
				Email:              "volume-check@test.com",
				Username:           "volumecheck",
				PasswordHash:       "hashedpassword",
				LanguagePreference: "en",
			}
			require.NoError(t, repo.Create(u))

			value := tc.value
			prefs := PreferencesUpdate{SoundVolume: &value}
			if tc.music {
				prefs = PreferencesUpdate{MusicVolume: &value}
			}
			err := repo.UpdatePreferences(u.ID, prefs)

			var pgErr *pgconn.PgError
			require.ErrorAs(t, err, &pgErr)
			assert.Equal(t, "23514", pgErr.Code, "must be a check violation")
			assert.Equal(t, tc.constraint, pgErr.ConstraintName)
		})
	}
}
