package friend

import "time"

// Friendship is a directional friend-relationship row (migration 000019).
// UserID is the requester (who sent the request); FriendID is the recipient.
// SendRequest inserts a 'pending' row; Accept flips it to 'accepted'; decline
// and unfriend hard-delete it (there is no soft-delete column).
//
// Exactly one row exists per ordered pair (unique index idx_friendships_pair on
// (user_id, friend_id)). That index does NOT catch the reverse-direction
// duplicate (B->A while A->B already exists) — the repo/handler block it via a
// direction-agnostic FindByPair, so the DB index alone must never be trusted as
// the duplicate guard.
//
// GORM auto-pluralizes the struct name to "friendships", which matches the
// table, so no TableName() override is needed.
type Friendship struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"column:user_id" json:"userId"`
	FriendID  uint      `gorm:"column:friend_id" json:"friendId"`
	Status    string    `gorm:"column:status" json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Friendship status values (the friendships.status column). A row is only ever
// in one of these two states; decline/unfriend remove the row rather than
// introducing a third state.
const (
	FriendStatusPending  = "pending"
	FriendStatusAccepted = "accepted"
)
