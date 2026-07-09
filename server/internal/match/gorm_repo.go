package match

import (
	"math"
	"time"

	"gorm.io/gorm"
)

// viewerTeamCase maps the viewer's seat to their team index — 0 (team A) for
// seats 0/2, 1 (team B) for seats 1/3 — given four userID bindings in
// player1..player4 order. Mirrors game.TeamForSeat without coupling the match
// package to the game engine. viewerTeamCaseM is the same expression with the
// matches table qualified as `m` for use inside joins.
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

func (r *GormMatchRepository) GetStatsForUser(userID uint) (wins, losses, abandoned int, err error) {
	// Single round-trip: FILTER aggregation over matches the user participates
	// in. The viewer's team per row is derived from their seat via a CASE
	// (seat 0/2 → 0 team A; 1/3 → 1 team B) matching game.TeamForSeat.
	//
	// Abandoned matches score per player: the abandoner (and every participant
	// of a NULL-abandoner boot-reconcile row) counts "abandoned"; the other
	// three count win/loss by winner_team — the non-abandoning team, persisted
	// live and backfilled by migration 000015. winner_team on abandoned rows is
	// meaningful ONLY when abandoned_by IS NOT NULL, hence the gate. A user with
	// zero matches yields (0, 0, 0, nil) — GORM's Scan into zero-valued struct.
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

func (r *GormMatchRepository) GetMatchesForUser(userID uint, limit, offset int, outcome, sort string) ([]Match, int64, error) {
	query := r.db.Model(&Match{}).
		Where("status IN ?", []string{"completed", "abandoned"}).
		Where(
			"player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?",
			userID, userID, userID, userID,
		)

	// Viewer-relative outcome filter. The viewer's team per row is derived from
	// their seat via the same CASE used by GetStatsForUser (seats 0/2 → 0,
	// 1/3 → 1). Mirrors GetStatsForUser's per-player abandonment semantics:
	// attributable abandoned rows (abandoned_by IS NOT NULL) count as win/loss
	// by winner_team for the three non-abandoners, while the abandoner and all
	// participants of NULL-abandoner rows keep "abandoned". Unknown / empty
	// outcome leaves the completed+abandoned set.
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

	// "old" → oldest-first; default ("new") → newest-first. Tie-break by id in
	// the same direction for a stable order. The literals are derived from a
	// validated allowlist (see parseMatchesQuery) so there is no injection risk.
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

// GetCareerAggregatesForUser computes the viewer-relative career metrics across
// every match the user participated in: capots won by their team, average
// completed-match duration, the single best hand their team scored, and the
// current win/loss streak. Each metric is a focused query; the streak's leading
// run is counted in Go over a bounded newest-first window.
func (r *GormMatchRepository) GetCareerAggregatesForUser(userID uint) (CareerAggregates, error) {
	var agg CareerAggregates

	// Capots won by the viewer's team (capot_team equals the viewer's team).
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

	// Average completed-match duration in seconds.
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

	// Most recent match (completed or abandoned) for the "last played" line.
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

	// Best single hand the viewer's team ever scored.
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

	// Current streak — leading run of identical outcomes over completed matches,
	// newest first. Abandoned matches are excluded (status filter), matching the
	// client's historical streak rule. The 200-row window is far beyond any real
	// streak length while keeping the query cheap.
	var outcomes []struct{ Won bool }
	if err := r.db.Raw(`
SELECT (winner_team = `+viewerTeamCase+`) AS won
FROM matches
WHERE status = 'completed'
  AND (player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?)
ORDER BY completed_at DESC, id DESC
LIMIT 200`,
		userID, userID, userID, userID,
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

// GetTopPartnersForUser returns the most-played teammates — the same-team seat
// across each match — ordered by matches played together. wins counts only
// completed matches the viewer's team won.
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
    winner_team
  FROM matches
  WHERE (player1_id = ? OR player2_id = ? OR player3_id = ? OR player4_id = ?)
    AND status IN ('completed', 'abandoned')
)
SELECT teammate_id AS user_id,
       COUNT(*) AS played,
       COUNT(*) FILTER (WHERE status = 'completed' AND winner_team = viewer_team) AS wins
FROM um
WHERE teammate_id IS NOT NULL
GROUP BY teammate_id
ORDER BY played DESC, wins DESC
LIMIT ?`,
		userID, userID, userID, userID,
		userID, userID, userID, userID,
		userID, userID, userID, userID,
		limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetTopRivalsForUser returns the most-faced opponents — the two opposite-team
// seats, unpivoted into one row each — ordered by completed matches played
// against them. wins/losses are viewer-relative.
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
    AND status = 'completed'
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
		limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
