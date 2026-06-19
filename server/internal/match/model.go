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
// "ignore bot seats" for XP/coins/honor/stats.
type Match struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	RoomID        uint      `gorm:"not null;index" json:"roomId"`
	Player1ID     *uint     `gorm:"index" json:"player1Id"`
	Player2ID     *uint     `gorm:"index" json:"player2Id"`
	Player3ID     *uint     `gorm:"index" json:"player3Id"`
	Player4ID     *uint     `gorm:"index" json:"player4Id"`
	Player1IsBot  bool      `gorm:"not null;default:false" json:"player1IsBot"`
	Player2IsBot  bool      `gorm:"not null;default:false" json:"player2IsBot"`
	Player3IsBot  bool      `gorm:"not null;default:false" json:"player3IsBot"`
	Player4IsBot  bool      `gorm:"not null;default:false" json:"player4IsBot"`
	HasBots       bool      `gorm:"not null;default:false;index" json:"hasBots"`
	TeamAScore    int       `gorm:"column:team_a_score;not null" json:"teamAScore"`
	TeamBScore    int       `gorm:"column:team_b_score;not null" json:"teamBScore"`
	WinnerTeam    int       `gorm:"not null" json:"winnerTeam"`
	Variant       string    `gorm:"size:20;not null" json:"variant"`
	MatchMode     string    `gorm:"size:10;not null" json:"matchMode"`
	StartedAt     time.Time `gorm:"not null" json:"startedAt"`
	CompletedAt   time.Time `gorm:"not null" json:"completedAt"`
	Status        string    `gorm:"size:20;not null;default:completed" json:"status"`
	AbandonedBy   *uint     `gorm:"index" json:"abandonedBy,omitempty"`
	SurrenderedBy *uint     `gorm:"index" json:"surrenderedBy,omitempty"`
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
