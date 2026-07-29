package user

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// seedHonorUser inserts a user with a known honor starting state inside the
// test transaction (rolled back by getTestDB's cleanup). Email/username are
// unique per call so multiple seeds in one test don't collide on the unique
// indexes.
func seedHonorUser(t *testing.T, repo *GormUserRepository, username string) *User {
	t.Helper()
	u := &User{
		Email:              username + "@honor.test",
		Username:           username,
		PasswordHash:       "x",
		LanguagePreference: "en",
	}
	require.NoError(t, repo.Create(u))
	return u
}

func TestGormUserRepository_ApplyHonorEvents_RecordsBothBuckets(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	finisher := seedHonorUser(t, repo, "honor_finisher")
	quitter := seedHonorUser(t, repo, "honor_quitter")
	now := time.Now().UTC()

	snaps, err := repo.ApplyHonorEvents(map[uint]HonorEvent{
		finisher.ID: {Abandoned: false},
		quitter.ID:  {Abandoned: true},
	}, now)
	require.NoError(t, err)

	// One completion each way, on top of a blank slate.
	assert.Equal(t, int64(1), snaps[finisher.ID].CompletedTotal)
	assert.Equal(t, int64(0), snaps[finisher.ID].AbandonedTotal)
	assert.Equal(t, int64(0), snaps[quitter.ID].CompletedTotal)
	assert.Equal(t, int64(1), snaps[quitter.ID].AbandonedTotal)

	// 100*(1+4)/(1+4+0+1) = 83.3 -> 83; 100*(0+4)/(0+4+4+1) = 44.4 -> 44.
	assert.Equal(t, 83, snaps[finisher.ID].Score)
	assert.Equal(t, HonorTierFair, snaps[finisher.ID].Tier)
	assert.Equal(t, 44, snaps[quitter.ID].Score)
	assert.Equal(t, HonorTierProblematic, snaps[quitter.ID].Tier)

	// Both are still under the floor of 5 matches PLAYED (one event each).
	assert.True(t, snaps[finisher.ID].IsNewPlayer)
	assert.True(t, snaps[quitter.ID].IsNewPlayer)

	// Persisted columns match the returned snapshot, including the
	// denormalized honor_score and the decay reference stamp.
	reloaded, err := repo.FindByID(finisher.ID)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, reloaded.HonorCompletedWeight, 1e-6)
	assert.InDelta(t, 0.0, reloaded.HonorAbandonedWeight, 1e-6)
	assert.Equal(t, int64(1), reloaded.HonorCompletedTotal)
	assert.Equal(t, 83, reloaded.HonorScoreSnapshot)
	require.NotNil(t, reloaded.HonorDecayedAt)
	assert.WithinDuration(t, now, *reloaded.HonorDecayedAt, time.Second)
}

// TestGormUserRepository_ApplyHonorEvents_DecayForwardIsExact is the load-bearing
// correctness proof for storing running weights instead of summing per-match
// weights on every read (Story 9.7 D3): write, advance `now` by two half-lives,
// write again — and assert the stored weight equals the from-scratch sum of the
// two matches' own decayed weights.
func TestGormUserRepository_ApplyHonorEvents_DecayForwardIsExact(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := seedHonorUser(t, repo, "honor_decay_exact")

	// Two matches 180 days apart; we read the result as of the later one.
	first := time.Now().UTC().Add(-180 * 24 * time.Hour)
	second := time.Now().UTC()

	_, err := repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: false}}, first)
	require.NoError(t, err)

	snaps, err := repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: false}}, second)
	require.NoError(t, err)

	// From scratch: 0.5^(180/90) + 0.5^(0/90) = 0.25 + 1 = 1.25.
	fromScratch := math.Pow(0.5, 2) + 1.0

	reloaded, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	// NUMERIC(14,6) storage, so compare at the column's precision.
	assert.InDelta(t, fromScratch, reloaded.HonorCompletedWeight, 1e-6,
		"decay-forward-then-add must equal the from-scratch weighted sum")

	// Raw totals are UNDECAYED: two matches played is two matches played.
	assert.Equal(t, int64(2), reloaded.HonorCompletedTotal)
	assert.Equal(t, int64(2), snaps[u.ID].CompletedTotal)

	// And the score is computed from the decayed weight, not the raw count.
	assert.Equal(t, HonorScore(fromScratch, 0, nil, second), snaps[u.ID].Score)
}

// TestGormUserRepository_ApplyHonorEvents_StampNeverMovesBackwards covers the
// 2026-07-29 code-review finding: honor_decayed_at was written unconditionally as
// `now`, so a stored stamp from the FUTURE (clock skew between app instances, an
// NTP step, or the 000017 backfill's Postgres NOW() running ahead of this host)
// was rolled backwards. DecayFactor correctly refuses to inflate weights across a
// negative interval, but the rolled-back reference meant the NEXT write decayed
// across the skew a SECOND time — silently over-decaying the row toward the 80
// prior and forgiving abandonments early.
func TestGormUserRepository_ApplyHonorEvents_StampNeverMovesBackwards(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := seedHonorUser(t, repo, "honor_skew")

	// First write lands a stamp one hour in the future, as a skewed peer would.
	future := time.Now().UTC().Add(time.Hour)
	_, err := repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: false}}, future)
	require.NoError(t, err)

	// Second write from a host whose clock is behind that stamp.
	behind := time.Now().UTC()
	require.True(t, behind.Before(future), "fixture precondition: the second clock is behind")
	_, err = repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: false}}, behind)
	require.NoError(t, err)

	reloaded, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.HonorDecayedAt)

	// Compared with a microsecond tolerance: timestamptz stores microseconds, so
	// the round-tripped stamp is truncated a few hundred nanoseconds below the
	// Go value that was written.
	assert.WithinDuration(t, future, *reloaded.HonorDecayedAt, time.Microsecond,
		"the decay reference must be clamped forward to the later stamp, not rolled back")
	assert.True(t, reloaded.HonorDecayedAt.After(behind),
		"the stamp must not regress to the behind-clock value")

	// Neither write decayed anything (both intervals are non-positive), so the
	// weight is exactly the two events at full strength. A rolled-back stamp
	// would not show here — it shows on the NEXT write — which is precisely why
	// the stamp itself is the thing asserted above.
	assert.InDelta(t, 2.0, reloaded.HonorCompletedWeight, 1e-6)
	assert.Equal(t, int64(2), reloaded.HonorCompletedTotal)
}

// TestGormUserRepository_ApplyHonorEvents_ImplausibleFutureStampIsReset is the
// other half of the clamp. Clamping forward without a ceiling traded a
// double-decay bug for an unbounded NO-decay window: DecayFactor returns 1.0 for
// any future stamp, so one wildly wrong value would freeze decay — and therefore
// freeze forgiveness — indefinitely and silently. Beyond honorMaxClockSkew the
// stamp is treated as corrupt and reset to now, which resumes decay. (Review
// pass 2.)
func TestGormUserRepository_ApplyHonorEvents_ImplausibleFutureStampIsReset(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := seedHonorUser(t, repo, "honor_farskew")

	// A stamp a year ahead — far past any plausible clock skew.
	absurd := time.Now().UTC().Add(365 * 24 * time.Hour)
	_, err := repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: false}}, absurd)
	require.NoError(t, err)

	now := time.Now().UTC()
	_, err = repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: false}}, now)
	require.NoError(t, err)

	reloaded, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.HonorDecayedAt)

	assert.WithinDuration(t, now, *reloaded.HonorDecayedAt, time.Second,
		"a stamp beyond the skew ceiling is discarded and the reference resumes at now")
	assert.True(t, reloaded.HonorDecayedAt.Before(absurd),
		"the absurd stamp must NOT be carried forward — that would freeze decay for a year")

	// Sanity: with the reference back at now, decay is live again.
	assert.Less(t, DecayFactor(reloaded.HonorDecayedAt, now.Add(90*24*time.Hour)), 0.51)
}

// TestGormUserRepository_ApplyHonorEvents_ForgivesAnOldAbandon is the PO's
// headline requirement expressed end-to-end through the repository.
func TestGormUserRepository_ApplyHonorEvents_ForgivesAnOldAbandon(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := seedHonorUser(t, repo, "honor_forgiven")

	longAgo := time.Now().UTC().Add(-365 * 24 * time.Hour)
	_, err := repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: true}}, longAgo)
	require.NoError(t, err)

	// Then twenty clean matches today.
	now := time.Now().UTC()
	var last HonorSnapshot
	for i := 0; i < 20; i++ {
		snaps, err := repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: false}}, now)
		require.NoError(t, err)
		last = snaps[u.ID]
	}

	assert.Equal(t, HonorTierExemplary, last.Tier,
		"a year-old abandonment must not keep a clean player out of Exemplary")
	assert.Equal(t, int64(20), last.CompletedTotal)
	assert.Equal(t, int64(1), last.AbandonedTotal)
	assert.False(t, last.IsNewPlayer)
}

func TestGormUserRepository_ApplyHonorEvents_EmptyMapIsNoOp(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	snaps, err := repo.ApplyHonorEvents(map[uint]HonorEvent{}, time.Now())
	require.NoError(t, err)
	assert.Empty(t, snaps)
}

// TestGormUserRepository_ApplyHonorEvents_SkipsBotPlaceholder pins that user id
// 0 — the bot seat placeholder — never reaches the users table.
func TestGormUserRepository_ApplyHonorEvents_SkipsBotPlaceholder(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	snaps, err := repo.ApplyHonorEvents(map[uint]HonorEvent{0: {Abandoned: true}}, time.Now())
	require.NoError(t, err)
	assert.Empty(t, snaps, "bot placeholder id 0 must never be written")
}

func TestGormUserRepository_ApplyHonorEvents_MissingUserRollsBack(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	existing := seedHonorUser(t, repo, "honor_missing")
	const missingID = uint(999999)
	now := time.Now().UTC()

	// The existing (lower) ID is locked and updated first; the missing (higher)
	// ID then fails, so the whole transaction must roll back.
	_, err := repo.ApplyHonorEvents(map[uint]HonorEvent{
		existing.ID: {Abandoned: false},
		missingID:   {Abandoned: false},
	}, now)
	require.ErrorIs(t, err, apperr.ErrUserNotFound)

	reloaded, err := repo.FindByID(existing.ID)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, reloaded.HonorCompletedWeight, 1e-6,
		"existing user must be unchanged after rollback")
	assert.Equal(t, int64(0), reloaded.HonorCompletedTotal)
	assert.Nil(t, reloaded.HonorDecayedAt)
}

// TestGormUserRepository_ResetHonor covers the operator forgiveness hook
// (Story 9.7 AC9). A pardon clears the PENALTY but preserves the EXPERIENCE:
// honor_completed_total survives, because it is the only input to IsNewPlayer and
// zeroing it turned a pardoned veteran into a "New Player" (review pass 2).
func TestGormUserRepository_ResetHonor(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	u := seedHonorUser(t, repo, "honor_reset")
	now := time.Now().UTC()

	// Six clean matches so the account is comfortably past the floor of 5, then
	// three abandonments to give the pardon something to forgive.
	for i := 0; i < 6; i++ {
		_, err := repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: false}}, now)
		require.NoError(t, err)
	}
	for i := 0; i < 3; i++ {
		_, err := repo.ApplyHonorEvents(map[uint]HonorEvent{u.ID: {Abandoned: true}}, now)
		require.NoError(t, err)
	}
	dirty, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	require.Equal(t, int64(6), dirty.HonorCompletedTotal)
	require.Equal(t, int64(3), dirty.HonorAbandonedTotal)
	require.Less(t, dirty.HonorScoreSnapshot, 80)

	require.NoError(t, repo.ResetHonor(u.ID))

	clean, err := repo.FindByID(u.ID)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, clean.HonorCompletedWeight, 1e-6, "the penalty weight is cleared")
	assert.InDelta(t, 0.0, clean.HonorAbandonedWeight, 1e-6, "the penalty weight is cleared")
	assert.Equal(t, int64(0), clean.HonorAbandonedTotal, "the abandonment record is forgiven")
	assert.Nil(t, clean.HonorDecayedAt, "reset must clear the decay reference stamp")
	assert.Equal(t, 80, clean.HonorScoreSnapshot, "score returns to the Beta(4,1) prior")

	assert.Equal(t, int64(6), clean.HonorCompletedTotal,
		"experience SURVIVES a pardon — zeroing this relabels a forgiven veteran a New Player")
	assert.False(t, IsNewPlayer(clean.HonorCompletedTotal, clean.HonorAbandonedTotal),
		"a pardoned veteran must read as a clean veteran, not a newcomer")

	// And the authoritative recompute agrees with the snapshot.
	assert.Equal(t, 80, HonorScore(clean.HonorCompletedWeight, clean.HonorAbandonedWeight, clean.HonorDecayedAt, time.Now()))
}

func TestGormUserRepository_ResetHonor_MissingUser(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormUserRepository(db)

	require.ErrorIs(t, repo.ResetHonor(999999), apperr.ErrUserNotFound)
}
