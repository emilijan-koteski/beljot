package friend

// Repository is the persistence boundary for friendships; handlers depend on it,
// never on GORM directly.
type Repository interface {
	Create(f *Friendship) error
	FindByID(id uint) (*Friendship, error)
	FindByPair(a, b uint) (*Friendship, error)
	Accept(id, recipientID uint) (int64, error)
	Delete(id, recipientID uint) (int64, error)
	Unfriend(id, userID uint) (int64, error)
	ListAccepted(userID uint) ([]Friendship, error)
	ListIncomingPending(userID uint) ([]Friendship, error)
	AreFriends(a, b uint) (bool, error)
}
