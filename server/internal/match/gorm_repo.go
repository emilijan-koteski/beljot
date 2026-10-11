package match

import (
	"errors"
	"math"
	"time"

	"gorm.io/gorm"
)

const viewerTeamCase = `(CASE
	WHEN player1_id = ? THEN 0
	WHEN player2_id = ? THEN 1
	WHEN player3_id = ? THEN 0
	WHEN player4_id = ? THEN 1
END)`

const viewerTeamCaseM = `(CASE
	WHEN m.player1_id = ? THEN 0
	WHEN m.player2_id = ? THEN 1
	WHEN m.player3_id = ? THEN 0
	WHEN m.player4_id = ? THEN 1
END)`

// GormMatchRepository implements MatchRepository using GORM.
type GormMatchRepository struct {
	db *gorm.DB
}

// NewGormMatchRepository creates a new GORM-backed match repository.
func NewGormMatchRepository(db *gorm.DB) *GormMatchRepository {
	return &GormMatchRepository{db: db}
}

func (r *GormMatchRepository) Create(match *Match) error {
	return r.db.Create(match).Error
}

func (r *GormMatchRepository) CreateWithHands(match *Match, hands []HandResult) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(match).Error; err != nil {
			return err
		}
		if len(hands) == 0 {
			return nil
		}
		for i := range hands {
			hands[i].MatchID = match.ID
		}
		return tx.Create(&hands).Error
	})
}

// GetStatsForUser counts the user's wins, losses, and abandonments. Another
// player's abandonment counts as a win or loss; the user's own or an
// unattributed one counts as abandoned.
func (r *GormMatchRepository) GetStatsForUser(userID uint) (wins, losses, abandoned int, err error) {
	var row struct {
		Wins      int
		Losses    int
		Abandoned int
	}
	err = r.db.Raw(`
SELECT
  COUNT(*) FILTER (
    WHERE (status = 'completed' OR (status = 'abandoned' AND abandoned_by IS NOT NULL AND abandoned_by <> ?))
      AND winner_team = `+viewerTeamCase+`
  ) AS wins,
  COUNT(*) FILTER (
    WHERE (status = 'completed' OR (status = 'abandoned' AND abandoned_by IS NOT NULL AND abandoned_by <> ?))
      AND winner_team <> `+viewerTeamCase+`
  ) AS losses,
  COUNT(*) FILTER (
    WHERE status = 'abandoned' AND (abandoned_by = ? OR abandoned_by IS NULL)
  ) AS abandoned
FROM matches
WHERE player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?
`,
		userID, userID, userID, userID, userID,
		userID, userID, userID, userID, userID,
		userID,
		userID, userID, userID, userID,
	).Scan(&row).Error
	if err != nil {
		return 0, 0, 0, err
	}
	return row.Wins, row.Losses, row.Abandoned, nil
}

// GetHonorTrendWindowsForUser counts completions and abandonments in the user's
// newest `limit` matches and in the `limit` matches before those.
func (r *GormMatchRepository) GetHonorTrendWindowsForUser(userID uint, limit int) (HonorTrendWindows, error) {
	var w HonorTrendWindows
	if limit <= 0 {
		return w, nil
	}
	var row struct {
		RecentCompleted int
		RecentAbandoned int
		PriorCompleted  int
		PriorAbandoned  int
	}
	err := r.db.Raw(`
SELECT
  COUNT(*) FILTER (WHERE rn <= ? AND (status = 'completed' OR (abandoned_by IS NOT NULL AND abandoned_by <> ?))) AS recent_completed,
  COUNT(*) FILTER (WHERE rn <= ? AND abandoned_by = ?)                                                           AS recent_abandoned,
  COUNT(*) FILTER (WHERE rn >  ? AND (status = 'completed' OR (abandoned_by IS NOT NULL AND abandoned_by <> ?))) AS prior_completed,
  COUNT(*) FILTER (WHERE rn >  ? AND abandoned_by = ?)                                                           AS prior_abandoned
FROM (
  SELECT
    status,
    abandoned_by,
    ROW_NUMBER() OVER (ORDER BY completed_at DESC, id DESC) AS rn
  FROM matches
  WHERE (player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?)
    AND (status = 'completed' OR (status = 'abandoned' AND abandoned_by IS NOT NULL))
  ORDER BY completed_at DESC, id DESC
  LIMIT ?
) AS window_rows
`,
		limit, userID,
		limit, userID,
		limit, userID,
		limit, userID,
		userID, userID, userID, userID,
		2*limit,
	).Scan(&row).Error
	if err != nil {
		return w, err
	}
	w.RecentCompleted = row.RecentCompleted
	w.RecentAbandoned = row.RecentAbandoned
	w.PriorCompleted = row.PriorCompleted
	w.PriorAbandoned = row.PriorAbandoned
	return w, nil
}

func (r *GormMatchRepository) GetMatchesForUser(userID uint, limit, offset int, outcome, sort string) ([]Match, int64, error) {
	query := r.db.Model(&Match{}).
		Where("status IN ?", []string{"completed", "abandoned"}).
		Where(
			"player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?",
			userID, userID, userID, userID,
		)

	switch outcome {
	case "win":
		query = query.Where(
			"(status = 'completed' OR (status = 'abandoned' AND abandoned_by IS NOT NULL AND abandoned_by <> ?)) AND winner_team = "+viewerTeamCase,
			userID, userID, userID, userID, userID,
		)
	case "loss":
		query = query.Where(
			"(status = 'completed' OR (status = 'abandoned' AND abandoned_by IS NOT NULL AND abandoned_by <> ?)) AND winner_team <> "+viewerTeamCase,
			userID, userID, userID, userID, userID,
		)
	case "abandoned":
		query = query.Where("status = 'abandoned' AND (abandoned_by = ? OR abandoned_by IS NULL)", userID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	dir := "DESC"
	if sort == "old" {
		dir = "ASC"
	}

	var items []Match
	err := query.
		Preload("Hands", func(db *gorm.DB) *gorm.DB {
			return db.Order("hand_number ASC")
		}).
		Order("completed_at " + dir).
		Order("id " + dir).
		Limit(limit).
		Offset(offset).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetLastMatchForRoomAndUser returns the room's most recent finished match with
// recorded hands that userID played, or (nil, nil) when there is none.
func (r *GormMatchRepository) GetLastMatchForRoomAndUser(roomID, userID uint) (*Match, error) {
	var m Match
	err := r.db.
		Where("room_id = ?", roomID).
		Where("status IN ?", []string{"completed", "abandoned"}).
		Where(
			"player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?",
			userID, userID, userID, userID,
		).
		Where("EXISTS (SELECT 1 FROM hand_results hr WHERE hr.match_id = matches.id)").
		Preload("Hands", func(db *gorm.DB) *gorm.DB {
			return db.Order("hand_number ASC")
		}).
		Order("completed_at DESC").
		Order("id DESC").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// GetCareerAggregatesForUser computes the user's capots won, average match
// duration, last played time, best hand, and current win/loss streak.
func (r *GormMatchRepository) GetCareerAggregatesForUser(userID uint) (CareerAggregates, error) {
	var agg CareerAggregates

	if err := r.db.Raw(`
SELECT COUNT(*)
FROM hand_results h
JOIN matches m ON m.id = h.match_id
WHERE (m.player1_id = ? OR m.player2_id = ? OR m.player3_id = ? OR m.player4_id = ?)
  AND h.capot = true
  AND h.capot_team = `+viewerTeamCaseM,
		userID, userID, userID, userID,
		userID, userID, userID, userID,
	).Scan(&agg.Capots).Error; err != nil {
		return CareerAggregates{}, err
	}

	var avg float64
	if err := r.db.Raw(`
SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0)
FROM matches
WHERE status = 'completed'
  AND (player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?)`,
		userID, userID, userID, userID,
	).Scan(&avg).Error; err != nil {
		return CareerAggregates{}, err
	}
	agg.AvgMatchSeconds = int(math.Round(avg))

	var lp struct{ LastPlayed *time.Time }
	if err := r.db.Raw(`
SELECT MAX(completed_at) AS last_played
FROM matches
WHERE status IN ('completed', 'abandoned')
  AND (player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?)`,
		userID, userID, userID, userID,
	).Scan(&lp).Error; err != nil {
		return CareerAggregates{}, err
	}
	if lp.LastPlayed != nil {
		agg.HasLastPlayed = true
		agg.LastPlayedAt = *lp.LastPlayed
	}

	var bh struct {
		Points      int
		HandNumber  int
		CompletedAt time.Time
	}
	bestRes := r.db.Raw(`
SELECT
  (CASE WHEN `+viewerTeamCaseM+` = 0 THEN h.team_a_hand_total ELSE h.team_b_hand_total END) AS points,
  h.hand_number AS hand_number,
  m.completed_at AS completed_at
FROM hand_results h
JOIN matches m ON m.id = h.match_id
WHERE (m.player1_id = ? OR m.player2_id = ? OR m.player3_id = ? OR m.player4_id = ?)
ORDER BY points DESC
LIMIT 1`,
		userID, userID, userID, userID,
		userID, userID, userID, userID,
	).Scan(&bh)
	if bestRes.Error != nil {
		return CareerAggregates{}, bestRes.Error
	}
	if bestRes.RowsAffected > 0 && bh.Points > 0 {
		agg.HasBestHand = true
		agg.BestHandPoints = bh.Points
		agg.BestHandNumber = bh.HandNumber
		agg.BestHandAt = bh.CompletedAt
	}

	var outcomes []struct{ Won bool }
	if err := r.db.Raw(`
SELECT (winner_team = `+viewerTeamCase+`) AS won
FROM matches
WHERE (status = 'completed' OR (status = 'abandoned' AND abandoned_by IS NOT NULL AND abandoned_by <> ?))
  AND (player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?)
ORDER BY completed_at DESC, id DESC
LIMIT 200`,
		userID, userID, userID, userID,
		userID,
		userID, userID, userID, userID,
	).Scan(&outcomes).Error; err != nil {
		return CareerAggregates{}, err
	}
	if len(outcomes) == 0 {
		agg.StreakKind = "none"
	} else {
		leading := outcomes[0].Won
		length := 0
		for _, o := range outcomes {
			if o.Won != leading {
				break
			}
			length++
		}
		if leading {
			agg.StreakKind = "win"
		} else {
			agg.StreakKind = "loss"
		}
		agg.StreakLength = length
	}

	return agg, nil
}

// GetCareerPointsForUser sums the user's own team score across their completed matches.
func (r *GormMatchRepository) GetCareerPointsForUser(userID uint) (int64, error) {
	var total int64
	err := r.db.Raw(`
SELECT COALESCE(SUM(
  CASE WHEN `+viewerTeamCase+` = 0 THEN team_a_score ELSE team_b_score END
), 0)
FROM matches
WHERE status = 'completed'
  AND (player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?)`,
		userID, userID, userID, userID,
		userID, userID, userID, userID,
	).Scan(&total).Error
	if err != nil {
		return 0, err
	}
	return total, nil
}

// GetTopPartnersForUser returns the user's most frequent teammates, ordered by
// matches played together, with the wins they shared.
func (r *GormMatchRepository) GetTopPartnersForUser(userID uint, limit int) ([]PartnerAggregate, error) {
	var rows []PartnerAggregate
	err := r.db.Raw(`
WITH um AS (
  SELECT
    (CASE
       WHEN player1_id = ? THEN player3_id
       WHEN player2_id = ? THEN player4_id
       WHEN player3_id = ? THEN player1_id
       WHEN player4_id = ? THEN player2_id
     END) AS teammate_id,
    `+viewerTeamCase+` AS viewer_team,
    status,
    abandoned_by,
    winner_team
  FROM matches
  WHERE (player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?)
    AND status IN ('completed', 'abandoned')
)
SELECT teammate_id AS user_id,
       COUNT(*) AS played,
       COUNT(*) FILTER (
         WHERE (status = 'completed' OR (status = 'abandoned' AND abandoned_by IS NOT NULL AND abandoned_by <> ?))
           AND winner_team = viewer_team
       ) AS wins
FROM um
WHERE teammate_id IS NOT NULL
GROUP BY teammate_id
ORDER BY played DESC, wins DESC
LIMIT ?`,
		userID, userID, userID, userID,
		userID, userID, userID, userID,
		userID, userID, userID, userID,
		userID,
		limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetTopRivalsForUser returns the user's most frequent opponents with viewer-relative
// wins and losses. The user's own and unattributed abandonments are excluded.
func (r *GormMatchRepository) GetTopRivalsForUser(userID uint, limit int) ([]RivalAggregate, error) {
	var rows []RivalAggregate
	err := r.db.Raw(`
WITH um AS (
  SELECT
    `+viewerTeamCase+` AS viewer_team,
    player1_id, player2_id, player3_id, player4_id,
    winner_team
  FROM matches
  WHERE (player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?)
    AND (status = 'completed' OR (status = 'abandoned' AND abandoned_by IS NOT NULL AND abandoned_by <> ?))
),
opp AS (
  SELECT (CASE WHEN viewer_team = 0 THEN player2_id ELSE player1_id END) AS opponent_id,
         (winner_team = viewer_team) AS viewer_won
  FROM um
  UNION ALL
  SELECT (CASE WHEN viewer_team = 0 THEN player4_id ELSE player3_id END) AS opponent_id,
         (winner_team = viewer_team) AS viewer_won
  FROM um
)
SELECT opponent_id AS user_id,
       COUNT(*) FILTER (WHERE viewer_won) AS wins,
       COUNT(*) FILTER (WHERE NOT viewer_won) AS losses
FROM opp
WHERE opponent_id IS NOT NULL
GROUP BY opponent_id
ORDER BY COUNT(*) DESC, wins DESC
LIMIT ?`,
		userID, userID, userID, userID,
		userID, userID, userID, userID,
		userID,
		limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
