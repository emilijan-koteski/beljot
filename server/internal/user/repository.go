package user

import "time"

// PreferencesUpdate is a partial write to the player's preference columns.
// A nil pointer leaves that column untouched, so false and 0 can still be written explicitly.
type PreferencesUpdate struct {
	LanguagePreference *string
	CardDeckPreference *string
	SoundEnabled       *bool
	MusicEnabled       *bool
	SoundVolume        *int
	MusicVolume        *int
}

// IsEmpty reports whether the update names no column at all.
func (p PreferencesUpdate) IsEmpty() bool {
	return p.LanguagePreference == nil && p.CardDeckPreference == nil &&
		p.SoundEnabled == nil && p.MusicEnabled == nil &&
		p.SoundVolume == nil && p.MusicVolume == nil
}

// UserRepository persists users and their per-user counters.
// Batch mutations lock rows FOR UPDATE in ascending userID order inside one transaction.
type UserRepository interface {
	Create(user *User) error
	Delete(id uint) error
	FindByEmail(email string) (*User, error)
	FindByUsername(username string) (*User, error)
	SearchByUsername(query string, excludeUserID uint, limit int) ([]User, error)
	FindByID(id uint) (*User, error)
	FindManyByIDs(ids []uint) ([]User, error)
	Count() (int64, error)
	UpdatePreferences(id uint, prefs PreferencesUpdate) error
	UpdatePasswordHash(id uint, hash string) error
	UpdateUsername(id uint, username string) (time.Time, error)
	AddXP(awards map[uint]int) (map[uint]int, error)
	TotalXPForUsers(ids []uint) (map[uint]int, error)
	ApplyHonorEvents(events map[uint]HonorEvent, now time.Time) (map[uint]HonorSnapshot, error)
	ResetHonor(userID uint) error
}
