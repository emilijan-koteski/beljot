package refreshtoken

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// purgeTestDB opens a rolled-back transaction on the throwaway BELJOT_TEST_DB_URL database, since the purge deletes across all users.
func purgeTestDB(t *testing.T) *gorm.DB {
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

func TestGormRepo_PurgeStale(t *testing.T) {
	db := purgeTestDB(t)
	repo := NewGormRepository(db)
	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	u := makeUser(t, db, "purge"+sfx[len(sfx)-6:]+"@rt.test")
	now := time.Now()
	ago := func(d time.Duration) *time.Time { at := now.Add(-d); return &at }

	tokens := []struct {
		name     string
		mutate   func(*RefreshToken)
		wantGone bool
	}{
		{name: "live", mutate: func(*RefreshToken) {}},
		{name: "rotated in a live family", mutate: func(rt *RefreshToken) { rt.RotatedAt = ago(40 * 24 * time.Hour) }},
		{name: "idle-expired in a live family", mutate: func(rt *RefreshToken) { rt.ExpiresAt = now.Add(-time.Hour) }},
		{name: "revoked yesterday", mutate: func(rt *RefreshToken) { rt.RevokedAt = ago(24 * time.Hour) }},
		{name: "revoked 31 days ago", mutate: func(rt *RefreshToken) { rt.RevokedAt = ago(31 * 24 * time.Hour) }, wantGone: true},
		{name: "family past its absolute cap", mutate: func(rt *RefreshToken) {
			rt.ExpiresAt = now.Add(-2 * time.Hour)
			rt.FamilyExpiresAt = now.Add(-time.Hour)
		}, wantGone: true},
	}
	for i, tc := range tokens {
		rt := makeToken(u.ID, fmt.Sprintf("purge-fam-%d-%s", i, sfx), fmt.Sprintf("purge-hash-%d-%s", i, sfx))
		tc.mutate(rt)
		require.NoError(t, repo.Create(rt))
	}

	deleted, err := repo.PurgeStale(now)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, int64(2))

	for i, tc := range tokens {
		got, err := repo.FindByHash(fmt.Sprintf("purge-hash-%d-%s", i, sfx))
		require.NoError(t, err)
		if tc.wantGone {
			assert.Nil(t, got, tc.name)
		} else {
			assert.NotNil(t, got, tc.name)
		}
	}
}
