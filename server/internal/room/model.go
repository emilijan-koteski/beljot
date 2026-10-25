package room

import (
	"time"

	"gorm.io/gorm"
)

// Room is a game table and its per-room rules. The bool rule columns carry no GORM `default`
// tag on purpose, because GORM would then omit a false from INSERTs; PasswordHash never
// leaves the server.
type Room struct {
	ID                   uint           `gorm:"primaryKey" json:"id"`
	Name                 string         `gorm:"size:100;not null" json:"name"`
	Code                 string         `gorm:"size:6;uniqueIndex;not null" json:"code"`
	OwnerID              uint           `gorm:"not null;index" json:"ownerId"`
	OwnerUsername        string         `gorm:"-" json:"ownerUsername"`
	OwnerAvatarURL       *string        `gorm:"-" json:"ownerAvatarUrl"`
	Players              []RoomPlayer   `gorm:"-" json:"players,omitempty"`
	Variant              string         `gorm:"size:20;not null;default:bitola" json:"variant"`
	MatchMode            string         `gorm:"size:10;not null;default:1001" json:"matchMode"`
	TimerStyle           string         `gorm:"size:20;not null;default:relaxed" json:"timerStyle"`
	TimerDurationSeconds *int           `json:"timerDurationSeconds"`
	ReconnectWindowSec   *int           `json:"reconnectWindowSec"`
	CoinBuyIn            int            `gorm:"not null;default:0" json:"coinBuyIn"`
	PasswordHash         *string        `gorm:"size:60" json:"-"`
	IsPrivate            bool           `gorm:"-" json:"isPrivate"`
	MinHonor             int            `gorm:"not null;default:0" json:"minHonor"`
	AllowNewPlayers      bool           `gorm:"not null" json:"allowNewPlayers"`
	DeclarationsEnabled  bool           `gorm:"not null" json:"declarationsEnabled"`
	StopAtTarget         bool           `gorm:"not null" json:"stopAtTarget"`
	DeclarationsTransfer bool           `gorm:"not null" json:"declarationsTransfer"`
	Clockwise            bool           `gorm:"not null" json:"clockwise"`
	Status               string         `gorm:"size:20;not null;default:waiting;index" json:"status"`
	PlayerCount          int            `gorm:"not null;default:1" json:"playerCount"`
	IsQuickPlay          bool           `gorm:"not null;default:false" json:"isQuickPlay"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

// AfterFind derives IsPrivate from PasswordHash on every GORM read. Paths that skip GORM reads,
// like CreateRoom and roomLifecyclePayload, set it explicitly.
func (r *Room) AfterFind(tx *gorm.DB) error {
	r.IsPrivate = r.PasswordHash != nil
	return nil
}

// RoomPlayer is a seated human, or a synthetic bot entry when IsBot is set. The honour and level
// fields are pointers so nil (not read, or a bot) stays distinct from a real 0.
type RoomPlayer struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RoomID     uint      `gorm:"not null;index" json:"roomId"`
	UserID     uint      `gorm:"not null;index" json:"userId"`
	Username   string    `gorm:"-" json:"username"`
	AvatarURL  *string   `gorm:"-" json:"avatarUrl"`
	Seat       *int      `json:"seat"`
	Team       *string   `gorm:"size:10" json:"team"`
	IsBot      bool      `gorm:"-" json:"isBot"`
	HonorScore *int      `gorm:"-" json:"honorScore,omitempty"`
	HonorTier  *string   `gorm:"-" json:"honorTier,omitempty"`
	Level      *int      `gorm:"-" json:"level,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// RoomBot is a bot occupying a seat in a waiting room. Bots have no user
// account; identity is seat-derived and rendered client-side (localized
// "Bot N"), so only the seat is persisted.
type RoomBot struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RoomID    uint      `gorm:"not null;index" json:"roomId"`
	Seat      int       `gorm:"not null" json:"seat"`
	CreatedAt time.Time `json:"createdAt"`
}
