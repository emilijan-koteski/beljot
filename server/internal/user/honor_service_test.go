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

func (f *fakeHonorRepo) Create(*User) error                             { panic("unused") }
func (f *fakeHonorRepo) Delete(uint) error                              { panic("unused") }
func (f *fakeHonorRepo) FindByEmail(string) (*User, error)              { panic("unused") }
func (f *fakeHonorRepo) FindByUsername(string) (*User, error)           { panic("unused") }
func (f *fakeHonorRepo) FindByID(uint) (*User, error)                   { panic("unused") }
func (f *fakeHonorRepo) FindManyByIDs([]uint) ([]User, error)           { panic("unused") }
func (f *fakeHonorRepo) Count() (int64, error)                          { panic("unused") }
func (f *fakeHonorRepo) UpdateLanguagePreference(uint, string) error    { panic("unused") }
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
