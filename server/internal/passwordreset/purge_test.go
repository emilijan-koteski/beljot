package passwordreset

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
	sfx := fmt.Sprintf("%08d", time.Now().UnixNano()%1e8)
	var userID uint
	require.NoError(t, db.Raw(`INSERT INTO users (email, username, password_hash) VALUES (?, ?, 'x') RETURNING id`,
		"prt"+sfx+"@reset.test", "prt"+sfx).Scan(&userID).Error)
	now := time.Now()
	used := now.Add(-time.Minute)

	tokens := []struct {
		name     string
		token    PasswordResetToken
		wantGone bool
	}{
		{name: "unused and valid", token: PasswordResetToken{ExpiresAt: now.Add(time.Hour)}},
		{name: "expired an hour ago", token: PasswordResetToken{ExpiresAt: now.Add(-time.Hour)}},
		{name: "expired two days ago", token: PasswordResetToken{ExpiresAt: now.Add(-48 * time.Hour)}, wantGone: true},
		{name: "used", token: PasswordResetToken{ExpiresAt: now.Add(time.Hour), UsedAt: &used}, wantGone: true},
	}
	for i := range tokens {
		tokens[i].token.UserID = userID
		tokens[i].token.TokenHash = fmt.Sprintf("purge-%d-%s", i, sfx)
		require.NoError(t, repo.Create(&tokens[i].token))
	}

	_, err := repo.PurgeStale(now)
	require.NoError(t, err)

	for _, tc := range tokens {
		var count int64
		require.NoError(t, db.Model(&PasswordResetToken{}).Where("id = ?", tc.token.ID).Count(&count).Error)
		if tc.wantGone {
			assert.Zero(t, count, tc.name)
		} else {
			assert.Equal(t, int64(1), count, tc.name)
		}
	}
}
