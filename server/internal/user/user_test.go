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

// TestGormUserRepository_SearchByUsername exercises the ILIKE search against a real Postgres.
// Fixtures are prefixed so real rows in a developer DB never land in the exact-set assertions.
func TestGormUserRepository_SearchByUsername(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	const pfx = "sbufx_"

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

	searcher := mk("aliSearcher")
	mk("Alice")
	mk("alicia")
	bob := mk("Bob")
	mk("car_ol")
	mk("carXol")
	mk("pct%name")
	mk("zzalpha")
	mk("zzbeta")
	mk("zzgamma")
	ghost := mk("ghostali")
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

// Registration never names the audio switches, so a zero-value false must land as the TRUE default.
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

// A raw insert naming neither audio column exercises the migration's column DEFAULT.
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

// false must be written on the preferences path while unnamed columns stay untouched.
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

func TestGormUserRepository_AllowNonFriendInvitesDefaultTrueOnRawInsert(t *testing.T) {
	db := getTestDB(t)

	require.NoError(t, db.Exec(
		`INSERT INTO users (email, username, password_hash, created_at, updated_at)
		 VALUES ('invite-raw@test.com', 'inviteraw', 'x', NOW(), NOW())`,
	).Error)

	var got struct{ AllowNonFriendInvites bool }
	require.NoError(t, db.Raw(
		"SELECT allow_non_friend_invites FROM users WHERE username = 'inviteraw'",
	).Scan(&got).Error)
	assert.True(t, got.AllowNonFriendInvites, "a row inserted without the column must land on TRUE")
}

func TestGormUserRepository_UpdatePreferences_AllowNonFriendInvites(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := &User{
		Email:                 "invite-update@test.com",
		Username:              "inviteupdate",
		PasswordHash:          "hashedpassword",
		LanguagePreference:    "mk",
		AllowNonFriendInvites: true,
	}
	require.NoError(t, repo.Create(u))

	off, on := false, true
	require.NoError(t, repo.UpdatePreferences(u.ID, PreferencesUpdate{AllowNonFriendInvites: &off}))
	reloaded, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.False(t, reloaded.AllowNonFriendInvites)
	assert.True(t, reloaded.SoundEnabled, "an invite-only update must not touch audio")
	assert.Equal(t, "mk", reloaded.LanguagePreference)

	require.NoError(t, repo.UpdatePreferences(u.ID, PreferencesUpdate{AllowNonFriendInvites: &on}))
	reloaded, err = repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.AllowNonFriendInvites)
}

func TestGormUserRepository_UpdatePreferences_EmptyAndMissing(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	assert.ErrorIs(t, repo.UpdatePreferences(1, PreferencesUpdate{}), apperr.ErrBadRequest)

	off := false
	assert.ErrorIs(t, repo.UpdatePreferences(999999, PreferencesUpdate{MusicEnabled: &off}), apperr.ErrUserNotFound)
}

// Registration never names the volumes, so a zero value must land as the tag default of 70.
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

// A raw insert naming neither volume column exercises the migration's column DEFAULT.
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

// 0 must be written on the preferences path while unnamed columns stay untouched.
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

// Each volume column CHECK must reject a level outside 0-100 with SQLSTATE 23514.
// Each case runs in its own transaction because the failed statement aborts it.
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

// UpdateAvatarKey writes the new prefix and returns the one it replaced, also when clearing.
func TestGormUserRepository_UpdateAvatarKey(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := &User{
		Email:              "avatar@test.com",
		Username:           "avataruser",
		PasswordHash:       "hashedpassword",
		LanguagePreference: "en",
	}
	require.NoError(t, repo.Create(u))

	fresh, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Nil(t, fresh.AvatarKey, "a new account has no avatar")

	p1 := "avatars/11111111-1111-4111-8111-111111111111"
	previous, err := repo.UpdateAvatarKey(u.ID, &p1)
	require.NoError(t, err)
	assert.Nil(t, previous, "first upload replaces nothing")

	p2 := "avatars/22222222-2222-4222-8222-222222222222"
	previous, err = repo.UpdateAvatarKey(u.ID, &p2)
	require.NoError(t, err)
	require.NotNil(t, previous)
	assert.Equal(t, p1, *previous, "replace returns the orphaned prefix")

	stored, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.AvatarKey)
	assert.Equal(t, p2, *stored.AvatarKey)

	previous, err = repo.UpdateAvatarKey(u.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, previous)
	assert.Equal(t, p2, *previous, "clearing returns the removed prefix")

	cleared, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.Nil(t, cleared.AvatarKey, "the column is NULL again")

	previous, err = repo.UpdateAvatarKey(u.ID, nil)
	require.NoError(t, err)
	assert.Nil(t, previous, "clearing an empty column is a no-op")

	_, err = repo.UpdateAvatarKey(u.ID+1_000_000, &p1)
	assert.ErrorIs(t, err, apperr.ErrUserNotFound)
}

func TestAvatarURL(t *testing.T) {
	t.Cleanup(func() { SetPublicAssetsURL("") })
	key := "avatars/33333333-3333-4333-8333-333333333333"
	empty := ""

	SetPublicAssetsURL("")
	assert.Nil(t, AvatarURL(&key, AvatarSizeSmall), "no base configured")

	SetPublicAssetsURL("https://assets.example.com")
	assert.Nil(t, AvatarURL(nil, AvatarSizeSmall), "no avatar")
	assert.Nil(t, AvatarURL(&empty, AvatarSizeSmall), "empty key")

	small := SmallAvatarURL(&key)
	require.NotNil(t, small)
	assert.Equal(t, "https://assets.example.com/avatars/33333333-3333-4333-8333-333333333333/128.webp", *small)

	large := AvatarURL(&key, AvatarSizeLarge)
	require.NotNil(t, large)
	assert.Equal(t, "https://assets.example.com/avatars/33333333-3333-4333-8333-333333333333/256.webp", *large)
}
