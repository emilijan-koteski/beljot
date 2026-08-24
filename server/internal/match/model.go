package match

import "time"

// PlayerSeatInfo holds the player info needed for live-match initialization.
// Defined here (not in room) to avoid an import cycle: match←→room via auth←user.
// Bot seats carry IsBot=true with UserID 0 and an empty Username — bot
// identity is seat-derived and rendered client-side.
type PlayerSeatInfo struct {
	UserID   uint
	Username string
	Seat     int
	IsBot    bool
}

// Match represents a completed game match record persisted to the database.
// Player IDs are nullable: bot seats persist NULL (the users FK forbids fake
// accounts) plus a per-seat IsBot flag; HasBots marks the match bot-inclusive
// for previews/history (Story 10.3) and is the rule Epic 9 inherits as
// "ignore bot seats" for XP/coins/honor/stats (honor shipped in Story 9.7).
type Match struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	RoomID       uint   `gorm:"not null;index" json:"roomId"`
	Player1ID    *uint  `gorm:"index" json:"player1Id"`
	Player2ID    *uint  `gorm:"index" json:"player2Id"`
	Player3ID    *uint  `gorm:"index" json:"player3Id"`
	Player4ID    *uint  `gorm:"index" json:"player4Id"`
	Player1IsBot bool   `gorm:"not null;default:false" json:"player1IsBot"`
	Player2IsBot bool   `gorm:"not null;default:false" json:"player2IsBot"`
	Player3IsBot bool   `gorm:"not null;default:false" json:"player3IsBot"`
	Player4IsBot bool   `gorm:"not null;default:false" json:"player4IsBot"`
	HasBots      bool   `gorm:"not null;default:false;index" json:"hasBots"`
	TeamAScore   int    `gorm:"column:team_a_score;not null" json:"teamAScore"`
	TeamBScore   int    `gorm:"column:team_b_score;not null" json:"teamBScore"`
	WinnerTeam   int    `gorm:"not null" json:"winnerTeam"`
	Variant      string `gorm:"size:20;not null" json:"variant"`
	MatchMode    string `gorm:"size:10;not null" json:"matchMode"`
	// The two per-room RULE flags this match was actually played under, captured
	// from the resolved rule config at match end.
	//
	// They are recorded on the MATCH and not just read back from the room because
	// a room is mutable and reusable: it hosts match after match and its settings
	// can be changed between them, so the room no longer tells you what a finished
	// match was played under. History reads the match.
	//
	// NO `default` TAG on either, the trap documented on room.Room.AllowNewPlayers:
	// GORM omits zero-valued fields from an INSERT when they declare a default,
	// which would make declarations_enabled = false and stop_at_target = true
	// unwritable — the two values that actually need recording. The DB-side
	// defaults in migration 000023 cover the backfill and any raw insert.
	DeclarationsEnabled bool      `gorm:"not null" json:"declarationsEnabled"`
	StopAtTarget        bool      `gorm:"not null" json:"stopAtTarget"`
	StartedAt           time.Time `gorm:"not null" json:"startedAt"`
	CompletedAt         time.Time `gorm:"not null" json:"completedAt"`
	Status              string    `gorm:"size:20;not null;default:completed" json:"status"`
	AbandonedBy         *uint     `gorm:"index" json:"abandonedBy,omitempty"`
	SurrenderedBy       *uint     `gorm:"index" json:"surrenderedBy,omitempty"`
	// Coin economy (Story 9.2). CoinBuyIn is the per-human stake captured at
	// StartMatch (0 for no-economy / quick-play matches). Player{N}CoinDelta is
	// the net wallet change for that seat this match (winner: share - buyIn;
	// loser: -buyIn; bot seat: 0). Mirrors the per-seat player{N}_is_bot style.
	CoinBuyIn        int          `gorm:"not null;default:0" json:"coinBuyIn"`
	Player1CoinDelta int          `gorm:"not null;default:0" json:"player1CoinDelta"`
	Player2CoinDelta int          `gorm:"not null;default:0" json:"player2CoinDelta"`
	Player3CoinDelta int          `gorm:"not null;default:0" json:"player3CoinDelta"`
	Player4CoinDelta int          `gorm:"not null;default:0" json:"player4CoinDelta"`
	CreatedAt        time.Time    `json:"createdAt"`
	Hands            []HandResult `gorm:"foreignKey:MatchID;constraint:OnDelete:CASCADE" json:"-"`
}

// matchSeatColumns projects a session's seat array into the persistence
// shape: nil IDs + IsBot flags for bot seats, plus the aggregate HasBots.
// botSeats comes from the final game state (PlayerState.IsBot).
func matchSeatColumns(playerIDs [4]uint, botSeats [4]bool) (ids [4]*uint, flags [4]bool, hasBots bool) {
	for i := range playerIDs {
		if botSeats[i] {
			flags[i] = true
			hasBots = true
			continue
		}
		uid := playerIDs[i]
		ids[i] = &uid
	}
	return ids, flags, hasBots
}
