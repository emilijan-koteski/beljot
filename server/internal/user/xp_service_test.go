package user

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLevelRepo is a minimal UserRepository stub exercising only the methods
// XPService.LevelsForUsers touches. All other methods panic to surface
// accidental coupling.
type fakeLevelRepo struct {
	totals map[uint]int
	err    error
}

func (f *fakeLevelRepo) TotalXPForUsers(ids []uint) (map[uint]int, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[uint]int, len(ids))
	for _, id := range ids {
		if v, ok := f.totals[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

func (f *fakeLevelRepo) Create(*User) error                          { panic("unused") }
func (f *fakeLevelRepo) FindByEmail(string) (*User, error)           { panic("unused") }
func (f *fakeLevelRepo) FindByUsername(string) (*User, error)        { panic("unused") }
func (f *fakeLevelRepo) FindByID(uint) (*User, error)                { panic("unused") }
func (f *fakeLevelRepo) FindManyByIDs([]uint) ([]User, error)        { panic("unused") }
func (f *fakeLevelRepo) Count() (int64, error)                       { panic("unused") }
func (f *fakeLevelRepo) UpdateLanguagePreference(uint, string) error { panic("unused") }
func (f *fakeLevelRepo) UpdatePasswordHash(uint, string) error       { panic("unused") }
func (f *fakeLevelRepo) AddXP(map[uint]int) (map[uint]int, error)    { panic("unused") }

func TestXPService_LevelsForUsers_AppliesCurve(t *testing.T) {
	repo := &fakeLevelRepo{totals: map[uint]int{
		10: 0,    // Level 0
		20: 60,   // Level 1 (>= 50)
		30: 1250, // Level 5
	}}
	svc := NewXPService(repo)

	levels, err := svc.LevelsForUsers([]uint{10, 20, 30, 99})
	require.NoError(t, err)

	assert.Equal(t, 0, levels[10])
	assert.Equal(t, 1, levels[20])
	assert.Equal(t, 5, levels[30])
	// Unknown user is absent (the repo omitted it), not a zero-keyed entry.
	_, present := levels[99]
	assert.False(t, present, "unknown user must be absent from the level map")
}

func TestXPService_LevelsForUsers_PropagatesError(t *testing.T) {
	svc := NewXPService(&fakeLevelRepo{err: errors.New("boom")})

	_, err := svc.LevelsForUsers([]uint{1})
	require.Error(t, err)
}
