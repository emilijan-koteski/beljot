package user

type UserRepository interface {
	Create(user *User) error
	FindByEmail(email string) (*User, error)
	FindByUsername(username string) (*User, error)
	FindByID(id uint) (*User, error)
	// FindManyByIDs returns all users whose ID is in the provided slice, in
	// arbitrary order. Returns an empty slice (no DB round-trip) when ids is
	// empty. Soft-deleted users are excluded via GORM's default scope.
	FindManyByIDs(ids []uint) ([]User, error)
	// Count returns the total number of registered (non-soft-deleted) users.
	Count() (int64, error)
	UpdateLanguagePreference(id uint, lang string) error
	// UpdatePasswordHash replaces the user's bcrypt password hash (used by the
	// password-reset flow). Returns ErrUserNotFound when no row matches.
	UpdatePasswordHash(id uint, hash string) error
}
