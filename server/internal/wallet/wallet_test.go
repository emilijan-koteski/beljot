package wallet

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/user"
)

func datePtr(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

// --- Pure streak/bonus rule tests (DB-free, table-driven) ---

func TestEvaluateDailyLogin(t *testing.T) {
	today := time.Date(2026, time.June, 18, 9, 30, 0, 0, time.UTC)

	tests := []struct {
		name          string
		lastLoginAt   *time.Time
		currentStreak int
		today         time.Time
		wantGrant     bool
		wantStreak    int
		wantAmount    int
	}{
		{
			name:        "first session (nil last login) grants day-1",
			lastLoginAt: nil, currentStreak: 0, today: today,
			wantGrant: true, wantStreak: 1, wantAmount: 1000,
		},
		{
			name:        "same UTC day does not grant",
			lastLoginAt: datePtr(2026, time.June, 18), currentStreak: 5, today: today,
			wantGrant: false, wantStreak: 5, wantAmount: 0,
		},
		{
			name:        "consecutive day increments streak",
			lastLoginAt: datePtr(2026, time.June, 17), currentStreak: 1, today: today,
			wantGrant: true, wantStreak: 2, wantAmount: 1162,
		},
		{
			name:        "registration (streak 0, stamped yesterday) yields day-1",
			lastLoginAt: datePtr(2026, time.June, 17), currentStreak: 0, today: today,
			wantGrant: true, wantStreak: 1, wantAmount: 1000,
		},
		{
			name:        "gap greater than one day resets streak to 1",
			lastLoginAt: datePtr(2026, time.June, 15), currentStreak: 9, today: today,
			wantGrant: true, wantStreak: 1, wantAmount: 1000,
		},
		{
			name:        "clock skew (last login in the future) resets to 1",
			lastLoginAt: datePtr(2026, time.June, 19), currentStreak: 4, today: today,
			wantGrant: true, wantStreak: 1, wantAmount: 1000,
		},
		{
			name:        "consecutive day past cap keeps incrementing streak but caps amount",
			lastLoginAt: datePtr(2026, time.June, 17), currentStreak: 14, today: today,
			wantGrant: true, wantStreak: 15, wantAmount: 3100,
		},
		{
			name:        "same-day check ignores time-of-day (late last login, early today)",
			lastLoginAt: datePtr(2026, time.June, 18), currentStreak: 3,
			today:     time.Date(2026, time.June, 18, 0, 0, 1, 0, time.UTC),
			wantGrant: false, wantStreak: 3, wantAmount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			grant, streak, amount := evaluateDailyLogin(tc.lastLoginAt, tc.currentStreak, tc.today)
			assert.Equal(t, tc.wantGrant, grant, "grant")
			assert.Equal(t, tc.wantStreak, streak, "streak")
			assert.Equal(t, tc.wantAmount, amount, "amount")
		})
	}
}

func TestBonusForStreak_Curve(t *testing.T) {
	tests := []struct {
		streak int
		want   int
	}{
		{1, 1000},
		{2, 1162},
		{13, 2944},
		{14, 3100}, // 3106 capped to 3100 — cap first reached on day 14
		{15, 3100},
		{50, 3100},
	}
	for _, tc := range tests {
		assert.Equalf(t, tc.want, bonusForStreak(tc.streak), "streak day %d", tc.streak)
	}
}

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

	// Per-test transaction rolled back on cleanup — tests create their own data
	// and never depend on seed data.
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	return tx
}

func createTestUser(t *testing.T, db *gorm.DB, email string, balance, streak int, lastLogin *time.Time) *user.User {
	t.Helper()
	u := &user.User{
		Email:           email,
		Username:        email[:min(len(email), 8)] + "_u",
		PasswordHash:    "x",
		WalletBalance:   balance,
		LoginStreakDays: streak,
		LastLoginAt:     lastLogin,
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

func TestGormRepository_ProcessDailyLogin_FirstSessionGrants(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := createTestUser(t, db, "first@wallet.test", 5000, 0, nil)

	today := time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC)
	res, err := repo.ProcessDailyLogin(u.ID, today)
	require.NoError(t, err)
	assert.True(t, res.Granted)
	assert.Equal(t, 1000, res.Amount)
	assert.Equal(t, 1, res.StreakDay)
	assert.Equal(t, 6000, res.NewBalance)
	assert.Equal(t, 1, res.LoginStreakDays)
}

func TestGormRepository_ProcessDailyLogin_SameDayNoGrant(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := createTestUser(t, db, "sameday@wallet.test", 5000, 0, nil)

	today := time.Date(2026, time.June, 18, 8, 0, 0, 0, time.UTC)
	first, err := repo.ProcessDailyLogin(u.ID, today)
	require.NoError(t, err)
	require.True(t, first.Granted)

	// Second call, same UTC day — no further grant.
	later := time.Date(2026, time.June, 18, 23, 59, 0, 0, time.UTC)
	second, err := repo.ProcessDailyLogin(u.ID, later)
	require.NoError(t, err)
	assert.False(t, second.Granted)
	assert.Equal(t, 0, second.Amount)
	assert.Equal(t, first.NewBalance, second.NewBalance)
	assert.Equal(t, 1, second.LoginStreakDays)
}

func TestGormRepository_ProcessDailyLogin_ConsecutiveDayIncrements(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := createTestUser(t, db, "consec@wallet.test", 5000, 0, nil)

	day1 := time.Date(2026, time.June, 18, 10, 0, 0, 0, time.UTC)
	_, err := repo.ProcessDailyLogin(u.ID, day1)
	require.NoError(t, err)

	day2 := time.Date(2026, time.June, 19, 10, 0, 0, 0, time.UTC)
	res, err := repo.ProcessDailyLogin(u.ID, day2)
	require.NoError(t, err)
	assert.True(t, res.Granted)
	assert.Equal(t, 2, res.StreakDay)
	assert.Equal(t, 1162, res.Amount)
	assert.Equal(t, 5000+1000+1162, res.NewBalance)
}

// TestGormRepository_ProcessDailyLogin_ConcurrentGrantsOnce verifies the row
// lock makes the once-per-day guard race-free (AC #3). It needs a committed row
// that two SEPARATE transactions can contend on, so it uses a raw (non-tx)
// connection and hard-deletes its own row on cleanup — a rolled-back outer
// transaction cannot host two concurrent FOR UPDATE waiters.
func TestGormRepository_ProcessDailyLogin_ConcurrentGrantsOnce(t *testing.T) {
	dsn := os.Getenv("BELJOT_DB_URL")
	if dsn == "" {
		dsn = "postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skip("skipping integration test: database not available")
	}

	u := &user.User{
		Email:           "concurrent@wallet.test",
		Username:        "concur_u",
		PasswordHash:    "x",
		WalletBalance:   5000,
		LoginStreakDays: 0,
	}
	require.NoError(t, db.Create(u).Error)
	t.Cleanup(func() { db.Unscoped().Delete(&user.User{}, u.ID) })

	repo := NewGormRepository(db)
	today := time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC)

	const n = 4
	var wg sync.WaitGroup
	results := make([]DailyLoginResult, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = repo.ProcessDailyLogin(u.ID, today)
		}(i)
	}
	wg.Wait()

	grantCount := 0
	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
		if results[i].Granted {
			grantCount++
		}
	}
	assert.Equal(t, 1, grantCount, "exactly one concurrent call must grant the daily bonus")

	// Balance must reflect exactly one grant of 1000.
	var reloaded user.User
	require.NoError(t, db.First(&reloaded, u.ID).Error)
	assert.Equal(t, 6000, reloaded.WalletBalance)
	assert.Equal(t, 1, reloaded.LoginStreakDays)
}
