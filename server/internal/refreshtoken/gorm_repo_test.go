package refreshtoken

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

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

func makeToken(userID uint, family, hash string) *RefreshToken {
	now := time.Now()
	return &RefreshToken{
		UserID:          userID,
		FamilyID:        family,
		TokenHash:       hash,
		ExpiresAt:       now.Add(30 * 24 * time.Hour),
		FamilyExpiresAt: now.Add(180 * 24 * time.Hour),
	}
}

func TestGormRepo_CreateAndFindByHash(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := makeUser(t, db, "find@rt.test")

	require.NoError(t, repo.Create(makeToken(u.ID, "fam1", "hash-abc")))

	got, err := repo.FindByHash("hash-abc")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "fam1", got.FamilyID)
	assert.Nil(t, got.RotatedAt)
	assert.Nil(t, got.RevokedAt)

	missing, err := repo.FindByHash("nope")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestGormRepo_RotateAndReplace(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := makeUser(t, db, "rot@rt.test")
	require.NoError(t, repo.Create(makeToken(u.ID, "fam2", "hash-rot-1")))

	got, err := repo.FindByHash("hash-rot-1")
	require.NoError(t, err)

	// First rotate wins: the old token is consumed and the successor is created
	// atomically.
	won, err := repo.RotateAndReplace(got.ID, makeToken(u.ID, "fam2", "hash-rot-2"))
	require.NoError(t, err)
	assert.True(t, won, "first rotate consumes the token and mints the successor")

	old, err := repo.FindByHash("hash-rot-1")
	require.NoError(t, err)
	require.NotNil(t, old)
	assert.NotNil(t, old.RotatedAt, "old token is now consumed")

	next, err := repo.FindByHash("hash-rot-2")
	require.NoError(t, err)
	require.NotNil(t, next)
	assert.Nil(t, next.RotatedAt)
	assert.Nil(t, next.RevokedAt)

	// Second rotate of the same (now consumed) token loses and must NOT create
	// a successor row.
	won2, err := repo.RotateAndReplace(got.ID, makeToken(u.ID, "fam2", "hash-rot-3"))
	require.NoError(t, err)
	assert.False(t, won2, "second rotate of an already-consumed token must lose")

	orphan, err := repo.FindByHash("hash-rot-3")
	require.NoError(t, err)
	assert.Nil(t, orphan, "a lost rotation must NOT create a successor")
}

func TestGormRepo_RevokeFamily(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := makeUser(t, db, "rev@rt.test")
	require.NoError(t, repo.Create(makeToken(u.ID, "fam4", "hash-rev-1")))
	require.NoError(t, repo.Create(makeToken(u.ID, "fam4", "hash-rev-2")))
	require.NoError(t, repo.Create(makeToken(u.ID, "other-fam", "hash-other")))

	require.NoError(t, repo.RevokeFamily("fam4"))

	for _, h := range []string{"hash-rev-1", "hash-rev-2"} {
		revoked, err := repo.FindByHash(h)
		require.NoError(t, err)
		require.NotNil(t, revoked)
		assert.NotNil(t, revoked.RevokedAt, "%s should be revoked", h)
	}

	// A different family is untouched.
	other, err := repo.FindByHash("hash-other")
	require.NoError(t, err)
	require.NotNil(t, other)
	assert.Nil(t, other.RevokedAt, "other family must not be revoked")
}
