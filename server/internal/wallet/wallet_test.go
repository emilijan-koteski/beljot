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

	"github.com/emilijan/beljot/server/internal/apperr"
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

// --- Story 9.2 wallet primitives: ChargeStakes / ApplySettlement / balances ---

func TestGormRepository_ChargeStakes_HappyPathDebitsAll(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := createTestUser(t, db, "csa@w.t", 1000, 0, nil)
	b := createTestUser(t, db, "csb@w.t", 1000, 0, nil)

	insolvent, err := repo.ChargeStakes([]uint{a.ID, b.ID}, 500)
	require.NoError(t, err)
	assert.Equal(t, uint(0), insolvent)

	ba, _ := repo.GetBalance(a.ID)
	bb, _ := repo.GetBalance(b.ID)
	assert.Equal(t, 500, ba)
	assert.Equal(t, 500, bb)
}

func TestGormRepository_ChargeStakes_InsolventRollsBackWholeTx(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	// rich is created first → lower ID → locked/debited first; poor then fails,
	// which must roll back rich's debit too (all-or-nothing).
	rich := createTestUser(t, db, "csrich@w.t", 1000, 0, nil)
	poor := createTestUser(t, db, "cspoor@w.t", 100, 0, nil)

	insolvent, err := repo.ChargeStakes([]uint{rich.ID, poor.ID}, 500)
	require.ErrorIs(t, err, apperr.ErrInsufficientCoins)
	assert.Equal(t, poor.ID, insolvent)

	rb, _ := repo.GetBalance(rich.ID)
	pb, _ := repo.GetBalance(poor.ID)
	assert.Equal(t, 1000, rb, "rich must be untouched (rollback)")
	assert.Equal(t, 100, pb, "poor must be untouched")
}

func TestGormRepository_ChargeStakes_NoOp(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := createTestUser(t, db, "csnoop@w.t", 1000, 0, nil)

	insolvent, err := repo.ChargeStakes([]uint{u.ID}, 0) // amount 0
	require.NoError(t, err)
	assert.Equal(t, uint(0), insolvent)

	insolvent, err = repo.ChargeStakes(nil, 500) // empty list
	require.NoError(t, err)
	assert.Equal(t, uint(0), insolvent)

	b, _ := repo.GetBalance(u.ID)
	assert.Equal(t, 1000, b)
}

func TestGormRepository_ApplySettlement_CreditsWinners(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := createTestUser(t, db, "asa@w.t", 500, 0, nil)
	b := createTestUser(t, db, "asb@w.t", 500, 0, nil)

	require.NoError(t, repo.ApplySettlement(map[uint]int{a.ID: 1000, b.ID: 0}))

	ba, _ := repo.GetBalance(a.ID)
	bb, _ := repo.GetBalance(b.ID)
	assert.Equal(t, 1500, ba)
	assert.Equal(t, 500, bb, "zero-amount credit is a no-op")

	// Empty credits map is a no-op success (no-human-winner sink).
	require.NoError(t, repo.ApplySettlement(map[uint]int{}))
}

func TestGormRepository_GetBalances(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	a := createTestUser(t, db, "gba@w.t", 700, 0, nil)
	b := createTestUser(t, db, "gbb@w.t", 800, 0, nil)

	m, err := repo.GetBalances([]uint{a.ID, b.ID})
	require.NoError(t, err)
	assert.Equal(t, 700, m[a.ID])
	assert.Equal(t, 800, m[b.ID])

	empty, err := repo.GetBalances(nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// TestGormRepository_ChargeStakes_ConcurrentNoDeadlock verifies the ascending
// userID lock order makes concurrent charges over the same two users
// deadlock-free even when the caller passes the IDs in opposite orders. It needs
// committed rows two SEPARATE transactions can contend on, so it uses a raw
// connection and hard-deletes its rows on cleanup.
func TestGormRepository_ChargeStakes_ConcurrentNoDeadlock(t *testing.T) {
	dsn := os.Getenv("BELJOT_DB_URL")
	if dsn == "" {
		dsn = "postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skip("skipping integration test: database not available")
	}

	u1 := &user.User{Email: "deadlock1@w.t", Username: "dl1_u", PasswordHash: "x", WalletBalance: 1000}
	u2 := &user.User{Email: "deadlock2@w.t", Username: "dl2_u", PasswordHash: "x", WalletBalance: 1000}
	require.NoError(t, db.Create(u1).Error)
	require.NoError(t, db.Create(u2).Error)
	t.Cleanup(func() {
		db.Unscoped().Delete(&user.User{}, u1.ID)
		db.Unscoped().Delete(&user.User{}, u2.ID)
	})

	repo := NewGormRepository(db)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	// Opposite input orders; ChargeStakes sorts internally so both lock u1→u2.
	inputs := [][]uint{{u1.ID, u2.ID}, {u2.ID, u1.ID}}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = repo.ChargeStakes(inputs[idx], 100)
		}(i)
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	b1, _ := repo.GetBalance(u1.ID)
	b2, _ := repo.GetBalance(u2.ID)
	assert.Equal(t, 800, b1, "both charges of 100 applied")
	assert.Equal(t, 800, b2)
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
