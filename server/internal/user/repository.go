package user

type UserRepository interface {
	Create(user *User) error
	// Delete soft-deletes the user (GORM DeletedAt). The users unique indexes
	// are partial (WHERE deleted_at IS NULL), so a soft-deleted row frees its
	// email and username again. Used as the compensating action when SSO
	// registration fails after the user insert. Returns ErrUserNotFound when
	// no live row matches.
	Delete(id uint) error
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
	// AddXP atomically adds each (userID -> delta) to that user's total_xp and
	// returns each user's NEW total (Story 9.5). Mirrors the wallet charge/settle
	// lock discipline (one transaction, FOR UPDATE, ascending userID order) so
	// concurrent match-ends — or one user finishing back-to-back matches — are
	// race-free. Zero-delta entries are skipped (absent from the result). Returns
	// ErrUserNotFound (rolling the whole batch back) if any target row is missing.
	AddXP(awards map[uint]int) (map[uint]int, error)
	// TotalXPForUsers returns the total_xp of each requested user, keyed by ID.
	// A read-only batch lookup used at match start to stamp each seat's static
	// level. Returns an empty map (no DB round-trip) when ids is empty; unknown
	// IDs are simply absent from the result (no error). Soft-deleted users are
	// excluded via GORM's default scope.
	TotalXPForUsers(ids []uint) (map[uint]int, error)
}
