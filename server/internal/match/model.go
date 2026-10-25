package match

import "time"

// PlayerSeatInfo holds the player info needed for live-match initialization.
// Bot seats carry IsBot=true with UserID 0 and an empty Username.
type PlayerSeatInfo struct {
	UserID    uint
	Username  string
	AvatarURL *string
	Seat      int
	IsBot     bool
}

// Match is a completed game match record persisted to the database. Bot seats
// persist a NULL player ID with IsBot set, and HasBots marks the whole match.
type Match struct {
	ID                   uint         `gorm:"primaryKey" json:"id"`
	RoomID               uint         `gorm:"not null;index" json:"roomId"`
	Player1ID            *uint        `gorm:"index" json:"player1Id"`
	Player2ID            *uint        `gorm:"index" json:"player2Id"`
	Player3ID            *uint        `gorm:"index" json:"player3Id"`
	Player4ID            *uint        `gorm:"index" json:"player4Id"`
	Player1IsBot         bool         `gorm:"not null;default:false" json:"player1IsBot"`
	Player2IsBot         bool         `gorm:"not null;default:false" json:"player2IsBot"`
	Player3IsBot         bool         `gorm:"not null;default:false" json:"player3IsBot"`
	Player4IsBot         bool         `gorm:"not null;default:false" json:"player4IsBot"`
	HasBots              bool         `gorm:"not null;default:false;index" json:"hasBots"`
	TeamAScore           int          `gorm:"column:team_a_score;not null" json:"teamAScore"`
	TeamBScore           int          `gorm:"column:team_b_score;not null" json:"teamBScore"`
	WinnerTeam           int          `gorm:"not null" json:"winnerTeam"`
	Variant              string       `gorm:"size:20;not null" json:"variant"`
	MatchMode            string       `gorm:"size:10;not null" json:"matchMode"`
	DeclarationsEnabled  bool         `gorm:"not null" json:"declarationsEnabled"`
	StopAtTarget         bool         `gorm:"not null" json:"stopAtTarget"`
	DeclarationsTransfer bool         `gorm:"not null" json:"declarationsTransfer"`
	StartedAt            time.Time    `gorm:"not null" json:"startedAt"`
	CompletedAt          time.Time    `gorm:"not null" json:"completedAt"`
	Status               string       `gorm:"size:20;not null;default:completed" json:"status"`
	AbandonedBy          *uint        `gorm:"index" json:"abandonedBy,omitempty"`
	SurrenderedBy        *uint        `gorm:"index" json:"surrenderedBy,omitempty"`
	CoinBuyIn            int          `gorm:"not null;default:0" json:"coinBuyIn"`
	Player1CoinDelta     int          `gorm:"not null;default:0" json:"player1CoinDelta"`
	Player2CoinDelta     int          `gorm:"not null;default:0" json:"player2CoinDelta"`
	Player3CoinDelta     int          `gorm:"not null;default:0" json:"player3CoinDelta"`
	Player4CoinDelta     int          `gorm:"not null;default:0" json:"player4CoinDelta"`
	CreatedAt            time.Time    `json:"createdAt"`
	Hands                []HandResult `gorm:"foreignKey:MatchID;constraint:OnDelete:CASCADE" json:"-"`
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
