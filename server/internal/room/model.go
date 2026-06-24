package room

import (
	"time"

	"gorm.io/gorm"
)

type Room struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Name    string `gorm:"size:100;not null" json:"name"`
	Code    string `gorm:"size:6;uniqueIndex;not null" json:"code"`
	OwnerID uint   `gorm:"not null;index" json:"ownerId"`
	// OwnerUsername is populated at the handler layer via a JOIN to `users`
	// before serialization. Not persisted on rooms (no migration), avoids the
	// extra write-path responsibility a denormalized column would create.
	OwnerUsername string `gorm:"-" json:"ownerUsername"`
	// Players is populated by the list-rooms handler so the lobby grid can
	// render seat chips inline without an extra round-trip per card. Marked
	// `omitempty` so the GET /rooms/:id detail endpoint, which returns players
	// via its own envelope, doesn't accidentally double-serialize them.
	Players              []RoomPlayer `gorm:"-" json:"players,omitempty"`
	Variant              string       `gorm:"size:20;not null;default:bitola" json:"variant"`
	MatchMode            string       `gorm:"size:10;not null;default:1001" json:"matchMode"`
	TimerStyle           string       `gorm:"size:20;not null;default:relaxed" json:"timerStyle"`
	TimerDurationSeconds *int         `json:"timerDurationSeconds"`
	ReconnectWindowSec   *int         `json:"reconnectWindowSec"`
	// CoinBuyIn is the per-human stake (coins) each seated human pays at match
	// start (Story 9.2). min 0, no maximum (owner freedom); create-room defaults
	// to 500, quick-play rooms persist 0. DB CHECK enforces >= 0.
	CoinBuyIn int `gorm:"not null;default:0" json:"coinBuyIn"`
	// PasswordHash is the bcrypt hash of a private room's password (Story 9.6,
	// FR60). NULL (nil pointer) is the public-room sentinel; a non-nil hash means
	// the room is private. `json:"-"` keeps the hash strictly server-side — it is
	// NEVER serialized to any client, WS payload, or log line.
	PasswordHash *string `gorm:"size:60" json:"-"`
	// IsPrivate is the derived, wire-facing privacy flag (Story 9.6). It is NOT a
	// column (`gorm:"-"`) — it is computed from PasswordHash != nil so the boolean
	// can never drift from the hash. Auto-populated on every DB read by the
	// AfterFind hook below; CreateRoom and the hand-built roomLifecyclePayload set
	// it explicitly because those paths don't read back through GORM.
	IsPrivate   bool           `gorm:"-" json:"isPrivate"`
	Status      string         `gorm:"size:20;not null;default:waiting;index" json:"status"`
	PlayerCount int            `gorm:"not null;default:1" json:"playerCount"`
	IsQuickPlay bool           `gorm:"not null;default:false" json:"isQuickPlay"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// AfterFind derives the wire-facing IsPrivate flag from PasswordHash on every
// GORM read (FindByID / FindByCode / FindByStatus / FindPlayerRoom, list and
// detail queries), so no per-handler edit is needed to surface privacy. Paths
// that don't read back through GORM — the freshly-Created room in CreateRoom and
// the hand-built roomLifecyclePayload map — set IsPrivate explicitly (Story 9.6).
func (r *Room) AfterFind(tx *gorm.DB) error {
	r.IsPrivate = r.PasswordHash != nil
	return nil
}

type RoomPlayer struct {
	ID       uint    `gorm:"primaryKey" json:"id"`
	RoomID   uint    `gorm:"not null;index" json:"roomId"`
	UserID   uint    `gorm:"not null;index" json:"userId"`
	Username string  `gorm:"-" json:"username"`
	Seat     *int    `json:"seat"`
	Team     *string `gorm:"size:10" json:"team"`
	// IsBot marks synthetic bot entries merged into wire payloads. Bots are
	// NOT room_players rows (the user_id FK forbids it) — they live in
	// room_bots and enter players arrays only via mergeBotPlayers as
	// {id:0, userId:0, username:"", seat, team, isBot:true}. Humans always
	// serialize isBot:false.
	IsBot     bool      `gorm:"-" json:"isBot"`
	CreatedAt time.Time `json:"createdAt"`
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
