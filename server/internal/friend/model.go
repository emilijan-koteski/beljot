package friend

import "time"

// Friendship is a friend request from UserID (requester) to FriendID (recipient).
// It is pending until accepted; decline and unfriend hard-delete the row.
type Friendship struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"column:user_id" json:"userId"`
	FriendID  uint      `gorm:"column:friend_id" json:"friendId"`
	Status    string    `gorm:"column:status" json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const (
	FriendStatusPending  = "pending"
	FriendStatusAccepted = "accepted"
)
