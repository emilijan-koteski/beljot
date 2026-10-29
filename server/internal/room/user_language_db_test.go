package room_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

// TestFindUserNameAndLanguage_ReadsTheHostsProfile runs the real users query against a database,
// skipping when none is reachable.
func TestFindUserNameAndLanguage_ReadsTheHostsProfile(t *testing.T) {
	db := getRoomTestDB(t)
	repo := room.NewGormRepository(db)

	host := &user.User{Email: "lang@room.test", Username: "langhost", PasswordHash: "x", LanguagePreference: "mk"}
	require.NoError(t, db.Create(host).Error)

	name, lang, err := repo.FindUserNameAndLanguage(host.ID)
	require.NoError(t, err)
	assert.Equal(t, "langhost", name)
	assert.Equal(t, "mk", lang)

	name, lang, err = repo.FindUserNameAndLanguage(host.ID + 100000)
	require.NoError(t, err)
	assert.Empty(t, name)
	assert.Empty(t, lang)
}
