package account

// Erased is what an erasure leaves for the caller to clean up outside the database.
type Erased struct {
	AvatarKey *string
}

// Repository erases a user by anonymising the users row in place and deleting every satellite row.
// Matches are never touched, so other players' history, stats and honor stay correct.
type Repository interface {
	InActiveRoom(userID uint) (bool, error)
	Erase(userID uint) (Erased, error)
}
