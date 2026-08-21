package user

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/match"
)

// fakeHonorRepo is a minimal UserRepository stub exercising only the two
// methods HonorService touches. All other methods panic to surface accidental
// coupling (same convention as fakeLevelRepo).
type fakeHonorRepo struct {
	gotEvents map[uint]HonorEvent
	gotNow    time.Time
	snapshots map[uint]HonorSnapshot
	applyErr  error

	resetCalledFor uint
	resetErr       error

	// Story 9.8: rows returned by FindManyByIDs for HonorForUsers, plus the ids
	// it was asked for and a call counter (so "empty input does no DB round-trip"
	// is provable).
	rows        []User
	findIDs     [][]uint
	findCalls   int
	findManyErr error
}

func (f *fakeHonorRepo) ApplyHonorEvents(events map[uint]HonorEvent, now time.Time) (map[uint]HonorSnapshot, error) {
	f.gotEvents = events
	f.gotNow = now
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	return f.snapshots, nil
}

func (f *fakeHonorRepo) ResetHonor(userID uint) error {
	f.resetCalledFor = userID
	return f.resetErr
}

func (f *fakeHonorRepo) Create(*User) error                   { panic("unused") }
func (f *fakeHonorRepo) Delete(uint) error                    { panic("unused") }
func (f *fakeHonorRepo) FindByEmail(string) (*User, error)    { panic("unused") }
func (f *fakeHonorRepo) FindByUsername(string) (*User, error) { panic("unused") }
func (f *fakeHonorRepo) SearchByUsername(string, uint, int) ([]User, error) {
	panic("unused")
}
func (f *fakeHonorRepo) FindByID(uint) (*User, error) { panic("unused") }
func (f *fakeHonorRepo) FindManyByIDs(ids []uint) ([]User, error) {
	f.findCalls++
	f.findIDs = append(f.findIDs, append([]uint(nil), ids...))
	if f.findManyErr != nil {
		return nil, f.findManyErr
	}
	return f.rows, nil
}

func (f *fakeHonorRepo) Count() (int64, error)                          { panic("unused") }
func (f *fakeHonorRepo) UpdatePreferences(uint, *string, *string) error { panic("unused") }
func (f *fakeHonorRepo) UpdatePasswordHash(uint, string) error          { panic("unused") }
func (f *fakeHonorRepo) UpdateUsername(uint, string) (time.Time, error) { panic("unused") }
func (f *fakeHonorRepo) AddXP(map[uint]int) (map[uint]int, error)       { panic("unused") }
func (f *fakeHonorRepo) TotalXPForUsers([]uint) (map[uint]int, error)   { panic("unused") }

// TestHonorService_ApplyHonorEvents_TranslatesBothDirections covers the only
// logic this adapter has: mapping match-package DTOs onto their user-package
// equivalents and back, so that match never has to import user (Story 9.7 D4).
func TestHonorService_ApplyHonorEvents_TranslatesBothDirections(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	repo := &fakeHonorRepo{
		snapshots: map[uint]HonorSnapshot{
			7: {Score: 96, Tier: HonorTierExemplary, CompletedTotal: 40, AbandonedTotal: 0, IsNewPlayer: false},
			9: {Score: 44, Tier: HonorTierProblematic, CompletedTotal: 1, AbandonedTotal: 1, IsNewPlayer: true},
		},
	}
	svc := NewHonorService(repo)

	out, err := svc.ApplyHonorEvents(map[uint]match.HonorEvent{
		7: {Abandoned: false},
		9: {Abandoned: true},
	}, now)
	require.NoError(t, err)

	// Inbound translation reached the repo intact.
	assert.Equal(t, map[uint]HonorEvent{7: {Abandoned: false}, 9: {Abandoned: true}}, repo.gotEvents)
	assert.Equal(t, now, repo.gotNow)

	// Outbound translation preserves every field.
	assert.Equal(t, match.HonorSnapshot{
		Score: 96, Tier: HonorTierExemplary, CompletedTotal: 40, AbandonedTotal: 0, IsNewPlayer: false,
	}, out[7])
	assert.Equal(t, match.HonorSnapshot{
		Score: 44, Tier: HonorTierProblematic, CompletedTotal: 1, AbandonedTotal: 1, IsNewPlayer: true,
	}, out[9])
}

func TestHonorService_ApplyHonorEvents_EmptyIsNoOp(t *testing.T) {
	repo := &fakeHonorRepo{}
	svc := NewHonorService(repo)

	out, err := svc.ApplyHonorEvents(map[uint]match.HonorEvent{}, time.Now())
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Nil(t, repo.gotEvents, "an empty batch must not reach the repository")
}

func TestHonorService_ApplyHonorEvents_PropagatesError(t *testing.T) {
	sentinel := errors.New("boom")
	svc := NewHonorService(&fakeHonorRepo{applyErr: sentinel})

	out, err := svc.ApplyHonorEvents(map[uint]match.HonorEvent{1: {}}, time.Now())
	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, out)
}

func TestHonorService_ResetHonor_DelegatesToRepo(t *testing.T) {
	repo := &fakeHonorRepo{}
	svc := NewHonorService(repo)

	require.NoError(t, svc.ResetHonor(42))
	assert.Equal(t, uint(42), repo.resetCalledFor)
}

// --- HonorForUsers (Story 9.8) --------------------------------------------

// HonorForUsers is the READ path Story 9.8's room gate goes through. The thing it
// must never do is hand back users.honor_score: that column is the lagging
// filter-only snapshot, and a decayed score moves with wall-clock time even when
// nothing is written. Every row goes through NewHonorSnapshot instead.
func TestHonorService_HonorForUsers_RecomputesRatherThanReadingTheSnapshotColumn(t *testing.T) {
	// A flawless veteran whose stored snapshot column is deliberately WRONG (5).
	// The recomputed value must win: 100*(20+4)/(20+4+0+1) = 2400/25 = 96.
	repo := &fakeHonorRepo{rows: []User{{
		ID:                   7,
		HonorCompletedWeight: 20,
		HonorAbandonedWeight: 0,
		HonorCompletedTotal:  40,
		HonorAbandonedTotal:  0,
		HonorScoreSnapshot:   5,
	}}}
	svc := NewHonorService(repo)

	out, err := svc.HonorForUsers([]uint{7})
	require.NoError(t, err)
	require.Contains(t, out, uint(7))

	assert.Equal(t, 96, out[7].Score, "the score is recomputed from the weights, never read from honor_score")
	assert.NotEqual(t, 5, out[7].Score, "the lagging snapshot column must not leak into the gate")
	assert.Equal(t, HonorTierExemplary, out[7].Tier)
	assert.Equal(t, int64(40), out[7].CompletedTotal)
	assert.Equal(t, int64(0), out[7].AbandonedTotal)
	assert.False(t, out[7].IsNewPlayer)
	assert.Equal(t, [][]uint{{7}}, repo.findIDs)
}

// The New Player flag the gate reads counts EXPERIENCE (completed + abandoned),
// not successes — the floor 9.7's review moved twice. A 0-completed /
// 20-abandoned account is NOT a New Player, and its real score is what the gate
// sees: 100*(0+4)/(0+4+4*20+1) = 400/85 = 5.
func TestHonorForUsers_NewPlayerFloorCountsExperienceNotSuccesses(t *testing.T) {
	repo := &fakeHonorRepo{rows: []User{
		{ID: 1, HonorCompletedWeight: 0, HonorAbandonedWeight: 20, HonorCompletedTotal: 0, HonorAbandonedTotal: 20},
		{ID: 2, HonorCompletedWeight: 0, HonorAbandonedWeight: 0, HonorCompletedTotal: 0, HonorAbandonedTotal: 0},
	}}
	svc := NewHonorService(repo)

	out, err := svc.HonorForUsers([]uint{1, 2})
	require.NoError(t, err)

	assert.False(t, out[1].IsNewPlayer, "20 abandonments is experience, not newness")
	assert.Equal(t, 5, out[1].Score, "the serial abandoner's real score reaches the gate")
	assert.Equal(t, HonorTierProblematic, out[1].Tier)

	assert.True(t, out[2].IsNewPlayer, "a genuine blank slate is a New Player")
	assert.Equal(t, 80, out[2].Score, "and sits on the Beta(4,1) prior")
}

// One `now` for the whole batch, so two seats checked in the same request cannot
// be decayed to different instants.
func TestHonorForUsers_StampsOneClockForTheWholeBatch(t *testing.T) {
	old := time.Now().UTC().Add(-90 * 24 * time.Hour) // exactly one half-life ago
	repo := &fakeHonorRepo{rows: []User{
		{ID: 1, HonorCompletedWeight: 8, HonorDecayedAt: &old, HonorCompletedTotal: 8},
		{ID: 2, HonorCompletedWeight: 8, HonorDecayedAt: &old, HonorCompletedTotal: 8},
	}}
	svc := NewHonorService(repo)

	out, err := svc.HonorForUsers([]uint{1, 2})
	require.NoError(t, err)
	assert.Equal(t, out[1].Score, out[2].Score, "identical rows must decay to an identical score")
	// One half-life halves the weight: 8 -> ~4, so 100*(4+4)/(4+4+0+1) = 89.
	assert.Equal(t, 89, out[1].Score)
}

// An unknown id is simply absent from the result rather than an error — the room
// gate decides what an absent row means (it treats it as a failure).
func TestHonorForUsers_UnknownIDIsAbsentNotAnError(t *testing.T) {
	repo := &fakeHonorRepo{rows: []User{{ID: 7, HonorCompletedTotal: 10}}}
	svc := NewHonorService(repo)

	out, err := svc.HonorForUsers([]uint{7, 999})
	require.NoError(t, err)
	assert.Contains(t, out, uint(7))
	assert.NotContains(t, out, uint(999))
}

func TestHonorForUsers_EmptyInputDoesNoDBRoundTrip(t *testing.T) {
	repo := &fakeHonorRepo{}
	svc := NewHonorService(repo)

	out, err := svc.HonorForUsers(nil)
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Equal(t, 0, repo.findCalls, "an empty batch must not reach the repository")
}

func TestHonorForUsers_PropagatesRepoError(t *testing.T) {
	sentinel := errors.New("db down")
	svc := NewHonorService(&fakeHonorRepo{findManyErr: sentinel})

	out, err := svc.HonorForUsers([]uint{1})
	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, out, "a read failure must never yield a usable map")
}
