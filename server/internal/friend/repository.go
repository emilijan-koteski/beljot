package friend

// Repository is the persistence boundary for friendships. Handlers depend on
// this interface, never on GORM directly — which also keeps every existing
// user.UserRepository mock across the tree untouched (this is a NEW interface,
// not an extension of the shared user repo).
type Repository interface {
	// Create inserts a new pending friendship row. A unique-constraint violation
	// on (user_id, friend_id) surfaces as apperr.ErrFriendRequestExists — but
	// that index only covers ONE direction, so the reverse duplicate must be
	// caught by the caller via FindByPair before Create is reached.
	Create(f *Friendship) error
	// FindByID returns the row with the given id, or (nil, nil) when none match.
	FindByID(id uint) (*Friendship, error)
	// FindByPair returns the single friendship between a and b in EITHER
	// direction ((a->b) OR (b->a)), or (nil, nil) when none exists. This is the
	// direction-agnostic duplicate/status check the single-direction unique
	// index cannot provide.
	FindByPair(a, b uint) (*Friendship, error)
	// Accept atomically flips a pending row to accepted, but ONLY when the caller
	// is the recipient: UPDATE ... WHERE id=? AND friend_id=? AND status='pending'.
	// Returns rows affected (0 = not the recipient, already accepted, or missing).
	Accept(id, recipientID uint) (int64, error)
	// Delete removes a pending row under the same recipient-only guard (decline).
	// Returns rows affected (0 = not the recipient, not pending, or missing).
	Delete(id, recipientID uint) (int64, error)
	// Unfriend removes an ACCEPTED row under a party-agnostic guard — either side
	// may end an accepted friendship: DELETE ... WHERE id=? AND (user_id=? OR
	// friend_id=?) AND status='accepted'. Returns rows affected (0 = not a party,
	// not accepted, or missing).
	Unfriend(id, userID uint) (int64, error)
	// ListAccepted returns the user's accepted friendships in EITHER direction
	// ((user_id=? OR friend_id=?) AND status='accepted'). Returns a non-nil empty
	// slice when the user has no friends.
	ListAccepted(userID uint) ([]Friendship, error)
	// ListIncomingPending returns the user's INCOMING pending requests
	// (friend_id=? AND status='pending'), newest first. Returns a non-nil empty
	// slice when there are none.
	ListIncomingPending(userID uint) ([]Friendship, error)
	// AreFriends reports whether a and b have an accepted friendship in either
	// direction. Convenience for Story 11.4 (whisper friend-check) and 11.5
	// (available-friend list), exposed now so those stories need not re-touch
	// this interface.
	AreFriends(a, b uint) (bool, error)
}
