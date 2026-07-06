package identity

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/user"
)

// --- Integration tests (Postgres; skipped when the DB is unavailable) ---

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

	// Per-test transaction rolled back on cleanup — tests create their own data.
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	return tx
}

func makeUser(t *testing.T, db *gorm.DB, email string) *user.User {
	t.Helper()
	u := &user.User{
		Email:              email,
		Username:           email[:min(len(email), 12)],
		PasswordHash:       "x",
		LanguagePreference: "en",
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

func makeIdentity(userID uint, provider, subject, email string) *Identity {
	return &Identity{
		UserID:         userID,
		Provider:       provider,
		ProviderUserID: subject,
		Email:          email,
	}
}

func TestGormRepo_CreateAndFindByProviderSubject(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := makeUser(t, db, "find@id.test")

	require.NoError(t, repo.Create(makeIdentity(u.ID, "google", "sub-find", "find@id.test")))

	got, err := repo.FindByProviderSubject("google", "sub-find")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, u.ID, got.UserID)
	assert.Equal(t, "find@id.test", got.Email)
	assert.False(t, got.CreatedAt.IsZero())

	// Same subject under a different provider is a different identity.
	other, err := repo.FindByProviderSubject("facebook", "sub-find")
	require.NoError(t, err)
	assert.Nil(t, other)

	missing, err := repo.FindByProviderSubject("google", "nope")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestGormRepo_DuplicateProviderSubjectConflicts(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u1 := makeUser(t, db, "dup1@id.test")
	u2 := makeUser(t, db, "dup2@id.test")

	require.NoError(t, repo.Create(makeIdentity(u1.ID, "google", "sub-dup", "dup1@id.test")))

	// The same external identity cannot be linked to a second account.
	err := repo.Create(makeIdentity(u2.ID, "google", "sub-dup", "dup2@id.test"))
	assert.ErrorIs(t, err, apperr.ErrSSOIdentityInUse)
}

func TestGormRepo_DuplicateUserProviderConflicts(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := makeUser(t, db, "twice@id.test")

	require.NoError(t, repo.Create(makeIdentity(u.ID, "google", "sub-a", "twice@id.test")))

	// One identity per provider per account.
	err := repo.Create(makeIdentity(u.ID, "google", "sub-b", "twice@id.test"))
	assert.ErrorIs(t, err, apperr.ErrSSOIdentityInUse)
}

func TestGormRepo_CascadeDeletesWithUser(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := makeUser(t, db, "cascade@id.test")

	require.NoError(t, repo.Create(makeIdentity(u.ID, "google", "sub-cascade", "cascade@id.test")))

	// Hard delete (Unscoped bypasses GORM soft delete) triggers ON DELETE CASCADE.
	require.NoError(t, db.Unscoped().Delete(&user.User{}, u.ID).Error)

	gone, err := repo.FindByProviderSubject("google", "sub-cascade")
	require.NoError(t, err)
	assert.Nil(t, gone, "identity must be cascade-deleted with its user")
}
