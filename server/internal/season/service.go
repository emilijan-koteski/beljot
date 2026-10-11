package season

import (
	"fmt"
	"time"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// Service awards match-end Season Points and serves the season read paths.
// A running season derives ranks from SP; an ended season shows its stored rank snapshot.
type Service struct {
	repo      Repository
	avatarURL func(key *string) *string
}

var _ match.SPAwarder = (*Service)(nil)

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// SetAvatarURLResolver injects the avatar-prefix-to-URL function used by leaderboard rows.
func (s *Service) SetAvatarURLResolver(resolve func(key *string) *string) {
	s.avatarURL = resolve
}

// avatarURLFor resolves one stored prefix, or nil with no resolver wired.
func (s *Service) avatarURLFor(key *string) *string {
	if s.avatarURL == nil {
		return nil
	}
	return s.avatarURL(key)
}

// resolveSeason wraps Repository.CurrentSeason and turns a nil season into an error,
// so a match finalizer never dereferences nil.
func (s *Service) resolveSeason(now time.Time) (*Season, error) {
	current, err := s.repo.CurrentSeason(now)
	if err != nil {
		return nil, fmt.Errorf("resolving current season: %w", err)
	}
	if current == nil {
		return nil, fmt.Errorf("resolving current season: repository returned no season covering %s",
			now.UTC().Format(time.RFC3339))
	}
	return current, nil
}

// ApplySeasonPoints scores the match for every human seat in one repository transaction
// and returns each player's post-write snapshot. An outcome with no human seat never touches the DB.
func (s *Service) ApplySeasonPoints(outcome match.MatchOutcome, now time.Time) (map[uint]match.PlayerSeasonSnapshot, error) {
	completed := make(map[uint]bool, len(outcome.Seats))
	for _, seat := range outcome.Seats {
		if seat.IsBot || seat.UserID == 0 {
			continue
		}
		completed[seat.UserID] = seat.Completed
	}
	if len(completed) == 0 {
		return map[uint]match.PlayerSeasonSnapshot{}, nil
	}

	current, err := s.resolveSeason(now)
	if err != nil {
		return nil, err
	}

	snapshots, err := s.repo.ApplySeasonPoints(current.ID, completed, func(sp map[uint]int) (map[uint]int, error) {
		return ComputeSPChanges(outcome, sp)
	})
	if err != nil {
		return nil, fmt.Errorf("applying season points: %w", err)
	}

	reasons := spReasons(outcome)
	out := make(map[uint]match.PlayerSeasonSnapshot, len(snapshots))
	for userID, snap := range snapshots {
		reason, ok := reasons[userID]
		if !ok {
			reason = ws.SPReasonNormal
		}
		out[userID] = match.PlayerSeasonSnapshot{
			SeasonName:   current.Name,
			SP:           snap.SP,
			SPChange:     snap.SP - snap.PreviousSP,
			RankTier:     snap.Tier,
			RankDivision: divisionOrNil(snap.Division),
			RankChange:   RankChange(snap.PreviousSP, snap.SP),
			Reason:       reason,
		}
	}
	return out, nil
}

// spReasons maps each human seat's user ID to its SP reason: abandoned, partner_abandoned or normal.
func spReasons(outcome match.MatchOutcome) map[uint]string {
	abandonedTeam := -1
	if a := outcome.AbandonedSeat; a >= 0 && a < len(outcome.Seats) {
		abandonedTeam = outcome.Seats[a].Team
	}
	out := make(map[uint]string, len(outcome.Seats))
	for seat, s := range outcome.Seats {
		if !isHumanSeat(s) {
			continue
		}
		switch {
		case seat == outcome.AbandonedSeat:
			out[s.UserID] = ws.SPReasonAbandoned
		case s.Team == abandonedTeam:
			out[s.UserID] = ws.SPReasonPartnerAbandoned
		default:
			out[s.UserID] = ws.SPReasonNormal
		}
	}
	return out
}

// derivedRank is a RUNNING season's rank: the live ladder over the SP total,
// with Master and Grandmaster's missing division as nil.
func derivedRank(sp int) (string, *int) {
	tier, division := RankForSP(sp)
	return tier, divisionOrNil(division)
}

// seasonRank returns the stored rank for an ended season and the SP-derived rank for a running one.
func seasonRank(ended bool, sp int, storedTier string, storedDivision *int) (string, *int) {
	if ended {
		return storedTier, storedDivision
	}
	return derivedRank(sp)
}

// CurrentSeasonView returns the active season and the viewer's rank progress, or the zero state without a row.
// It may lazily create the season row but never a player_seasons row.
func (s *Service) CurrentSeasonView(userID uint, now time.Time) (*CurrentSeasonView, error) {
	current, err := s.resolveSeason(now)
	if err != nil {
		return nil, err
	}

	record, err := s.repo.FindPlayerSeason(userID, current.ID)
	if err != nil {
		return nil, fmt.Errorf("reading player season: %w", err)
	}

	sp, gamesPlayed, gamesCompleted := 0, 0, 0
	if record != nil {
		sp = record.SP
		gamesPlayed = record.GamesPlayed
		gamesCompleted = record.GamesCompleted
	}

	tier, division, intoStep, forNextStep := RankProgress(sp)

	return &CurrentSeasonView{
		SeasonName:        current.Name,
		EndsAt:            current.EndsAt,
		SP:                sp,
		RankTier:          tier,
		RankDivision:      divisionOrNil(division),
		SPIntoDivision:    intoStep,
		SPForNextDivision: forNextStep,
		GamesPlayed:       gamesPlayed,
		GamesCompleted:    gamesCompleted,
	}, nil
}

// LeaderboardView returns one SP-ordered page of a season plus the viewer's position; seasonID 0 is the current season.
// An unknown seasonID returns ErrSeasonNotFound, never a fallback to the active window.
func (s *Service) LeaderboardView(userID, seasonID uint, limit, offset int, now time.Time) (*LeaderboardView, error) {
	var window *Season
	ended := false
	if seasonID == 0 {
		current, err := s.resolveSeason(now)
		if err != nil {
			return nil, err
		}
		window = current
	} else {
		found, err := s.repo.FindSeasonByID(seasonID)
		if err != nil {
			return nil, fmt.Errorf("reading season %d: %w", seasonID, err)
		}
		if found == nil {
			return nil, apperr.ErrSeasonNotFound
		}
		window = found
		ended = !found.EndsAt.After(now)
	}

	entries, total, err := s.repo.LeaderboardPage(window.ID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("reading leaderboard page: %w", err)
	}

	items := make([]LeaderboardRowView, 0, len(entries))
	for i, e := range entries {
		tier, division := seasonRank(ended, e.SP, e.RankTier, e.RankDivision)
		items = append(items, LeaderboardRowView{
			Position:    offset + i + 1,
			UserID:      e.UserID,
			Username:    e.Username,
			AvatarURL:   s.avatarURLFor(e.AvatarKey),
			SP:          e.SP,
			Tier:        tier,
			Division:    division,
			GamesPlayed: e.GamesPlayed,
		})
	}

	viewer, err := s.viewerPosition(userID, window.ID, ended)
	if err != nil {
		return nil, err
	}

	return &LeaderboardView{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
		Viewer: viewer,
	}, nil
}

// viewerPosition returns the caller's standing under the leaderboard's own visibility predicate and order,
// or nil when the list would not show them.
func (s *Service) viewerPosition(userID, seasonID uint, ended bool) (*LeaderboardViewerView, error) {
	entry, err := s.repo.FindLeaderboardEntry(seasonID, userID)
	if err != nil {
		return nil, fmt.Errorf("reading viewer leaderboard entry: %w", err)
	}
	if entry == nil {
		return nil, nil
	}

	ahead, err := s.repo.CountAhead(seasonID, entry.SP, userID)
	if err != nil {
		return nil, fmt.Errorf("counting leaderboard rows ahead: %w", err)
	}

	tier, division := seasonRank(ended, entry.SP, entry.RankTier, entry.RankDivision)
	return &LeaderboardViewerView{
		Position:    int(ahead) + 1,
		UserID:      userID,
		SP:          entry.SP,
		Tier:        tier,
		Division:    division,
		GamesPlayed: entry.GamesPlayed,
	}, nil
}

// SeasonsView lists every season newest-first, resolving the current one first so it is always present.
func (s *Service) SeasonsView(now time.Time) (*SeasonsListView, error) {
	if _, err := s.resolveSeason(now); err != nil {
		return nil, err
	}

	seasons, err := s.repo.ListSeasons()
	if err != nil {
		return nil, fmt.Errorf("listing seasons: %w", err)
	}

	items := make([]SeasonListItemView, 0, len(seasons))
	for _, se := range seasons {
		items = append(items, SeasonListItemView{
			ID:        se.ID,
			Name:      se.Name,
			StartedAt: se.StartedAt,
			EndsAt:    se.EndsAt,
		})
	}
	return &SeasonsListView{Items: items}, nil
}

// ArchiveView returns the user's ended, played seasons newest-first with the stored rank each finished on.
// An unknown user gets an empty archive.
func (s *Service) ArchiveView(userID uint, now time.Time) (*ArchiveView, error) {
	entries, err := s.repo.PlayerSeasonArchive(userID, now)
	if err != nil {
		return nil, fmt.Errorf("reading season archive: %w", err)
	}

	items := make([]ArchiveRowView, 0, len(entries))
	for _, e := range entries {
		tier, division := seasonRank(true, e.SP, e.RankTier, e.RankDivision)
		items = append(items, ArchiveRowView{
			SeasonID:    e.SeasonID,
			SeasonName:  e.SeasonName,
			SP:          e.SP,
			Tier:        tier,
			Division:    division,
			GamesPlayed: e.GamesPlayed,
			StartedAt:   e.StartedAt,
			EndsAt:      e.EndsAt,
		})
	}
	return &ArchiveView{Items: items}, nil
}

// CurrentSeasonRank returns the user's rank in the active season, or nil when they have no row in it.
func (s *Service) CurrentSeasonRank(userID uint, now time.Time) (*SeasonRankView, error) {
	current, err := s.resolveSeason(now)
	if err != nil {
		return nil, err
	}

	record, err := s.repo.FindPlayerSeason(userID, current.ID)
	if err != nil {
		return nil, fmt.Errorf("reading player season: %w", err)
	}
	if record == nil {
		return nil, nil
	}

	tier, division := derivedRank(record.SP)
	return &SeasonRankView{
		SeasonName: current.Name,
		Tier:       tier,
		Division:   division,
		SP:         record.SP,
	}, nil
}
