package match

import "time"

// HandResult persists the scoring breakdown for a single completed hand of a match.
// It is written alongside the Match row by MatchRepository.CreateWithHands.
type HandResult struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	MatchID         uint      `gorm:"not null;index;uniqueIndex:idx_hand_results_match_hand" json:"matchId"`
	HandNumber      int       `gorm:"not null;uniqueIndex:idx_hand_results_match_hand" json:"handNumber"`
	TeamACardPoints int       `gorm:"column:team_a_card_points;not null" json:"teamACardPoints"`
	TeamBCardPoints int       `gorm:"column:team_b_card_points;not null" json:"teamBCardPoints"`
	TeamADeclPoints int       `gorm:"column:team_a_decl_points;not null" json:"teamADeclPoints"`
	TeamBDeclPoints int       `gorm:"column:team_b_decl_points;not null" json:"teamBDeclPoints"`
	LastTrickTeam   int       `gorm:"not null" json:"lastTrickTeam"`
	LastTrickBonus  int       `gorm:"not null" json:"lastTrickBonus"`
	Capot           bool      `gorm:"not null" json:"capot"`
	CapotTeam       *int      `json:"capotTeam,omitempty"`
	CapotBonus      int       `gorm:"not null" json:"capotBonus"`
	FailedContract  bool      `gorm:"not null" json:"failedContract"`
	ContractingTeam int       `gorm:"not null" json:"contractingTeam"`
	TeamAHandTotal  int       `gorm:"column:team_a_hand_total;not null" json:"teamAHandTotal"`
	TeamBHandTotal  int       `gorm:"column:team_b_hand_total;not null" json:"teamBHandTotal"`
	CreatedAt       time.Time `json:"createdAt"`
}
