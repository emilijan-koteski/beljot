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
	IsPrivate bool `gorm:"-" json:"isPrivate"`
	// MinHonor is the minimum honor score an EXPERIENCED player must clear to join
	// (Story 9.8, FR57). 0 means no bar, and is the default. The DB CHECK bounds it
	// to [0,100]; the handler range-validates the same interval so client and server
	// agree in the same unit (a plain integer — no rune/byte asymmetry like 9.6's
	// password bounds had).
	//
	// The value the gate compares against is NEVER users.honor_score (the lagging
	// snapshot column) — it is the score recomputed at request time through
	// user.NewHonorSnapshot. See honorGateError in handler.go.
	MinHonor int `gorm:"not null;default:0" json:"minHonor"`
	// AllowNewPlayers is whether players with no established track record
	// (user.IsNewPlayer — fewer than 5 finished-or-abandoned matches) may join at
	// all (Story 9.8, FR57). It is INDEPENDENT of MinHonor (D1): a New Player is
	// never score-checked, so this toggle is the owner's explicit "I'll take an
	// unknown" switch, and it applies even in a MinHonor == 0 room.
	//
	// NO `default` TAG, DELIBERATELY — this is a silent-data-corruption trap, not a
	// style preference. GORM does not send a zero-valued field (0, "", false) in an
	// INSERT when that field declares a `default` tag; it lets the database apply
	// the default instead. With `gorm:"default:true"` it would be IMPOSSIBLE to
	// create a room with allow_new_players = false: the value would silently flip
	// to true. (Per the GORM docs: "any zero value fields like 0, '', false won't
	// be saved into the database for those fields defined default value … you might
	// want to use a pointer type or Scanner/Valuer to avoid this.") Omitting the tag
	// makes GORM send the real boolean every time; the DB-side DEFAULT TRUE in
	// migration 000018 still covers the backfill and any raw insert.
	//
	// The inverse trap is closed by construction: with no GORM default, a hand-built
	// &Room{...} that FORGETS this field inserts false (veterans-only). Both &Room{}
	// sites therefore set it explicitly — CreateRoom and the Quick Play synthesis.
	//
	// Rejected alternatives: *bool (drags nil-handling into every read path for what
	// is only a write-path problem) and inverting the column to `veterans_only`
	// (deviates from the AC-mandated column name and installs a permanent double
	// negative at every call site).
	AllowNewPlayers bool           `gorm:"not null" json:"allowNewPlayers"`
	Status          string         `gorm:"size:20;not null;default:waiting;index" json:"status"`
	PlayerCount     int            `gorm:"not null;default:1" json:"playerCount"`
	IsQuickPlay     bool           `gorm:"not null;default:false" json:"isQuickPlay"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
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
	IsBot bool `gorm:"-" json:"isBot"`
	// Honour, attached for the waiting-room roster (honour redesign R6). Not a
	// column — hydrated per-request from user.HonorService.HonorForUsers, which
	// recomputes from the stored weights.
	//
	// POINTERS so "not read" is distinguishable from a real 0: an honour score of
	// 0 is a legitimate value ("Problematic"), and a plain int would make the
	// two indistinguishable — the exact class of bug the 9.7/9.8 reviews closed
	// repeatedly. nil means "no honour available for this seat" (a bot, an honour
	// service that is not wired, or a read that failed) and the client renders no
	// shield rather than a wrong one.
	//
	// Public-safe: the profile response already exposes the same score and tier
	// to anyone, so the roster reveals nothing new. It is deliberately NOT the
	// counts or the trend — a seatmate needs to know how reliable someone is, not
	// their history.
	HonorScore *int    `gorm:"-" json:"honorScore,omitempty"`
	HonorTier  *string `gorm:"-" json:"honorTier,omitempty"`
	// Level is the seat's lifetime level, hydrated alongside honour from the same
	// read for the roster (level renders before the shield on each seat tile).
	// Same pointer semantics as HonorScore: nil means "not read" (a bot, or the
	// hydration failed), while a real level 0 — a brand-new account — arrives as 0.
	Level     *int      `gorm:"-" json:"level,omitempty"`
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
