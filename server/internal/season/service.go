package season

import (
	"fmt"
	"time"

	"github.com/emilijan/beljot/server/internal/match"
)

// Service is the thin match-end SP awarder injected into the match manager as
// its SPAwarder, plus the read path GET /api/v1/seasons/current goes through.
//
// It owns no ladder logic: the tier math is pure (tier.go), the quarter math is
// pure (quarter.go), the atomic write is the repository's, and the per-seat
// bucketing lives in the match package. What this type exists for is the DTO
// TRANSLATION below.
//
// THE IMPORT DIRECTION IS THE WHOLE POINT. `season` may import `match`; `match`
// MUST NEVER IMPORT `season` -- the same rule XPService and HonorService call
// out in capitals (Story 9.5 D1 / 9.7 D4, restated as Story 13.1 D8). So
// match.SPAward and match.PlayerSeasonSnapshot are declared in MATCH, and this
// adapter maps them onto the season-domain equivalents. It also means the
// snapshot the manager receives is fully PRECOMPUTED -- season name, derived
// tier and tieredUp all resolved here -- so the manager never runs ladder
// arithmetic it cannot see.
type Service struct {
	repo Repository
}

// Compile-time proof that the injection in cmd/api/main.go keeps working. The
// match manager takes SPAwarder structurally, so a signature drift would
// otherwise only surface as a wiring failure at startup.
var _ match.SPAwarder = (*Service)(nil)

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// resolveSeason wraps Repository.CurrentSeason with the nil check its own
// contract does not enforce.
//
// The interface documents "Never returns (nil, nil)" and the GORM implementation
// honours it, but nothing in the type system does — and a nil here would be
// dereferenced for `.ID` and `.Name` INSIDE A MATCH FINALIZER, where a panic
// takes down the goroutine that is mid-way through settling four players'
// coins, XP and honor. A wrapped error instead degrades to awardSeasonPoints'
// best-effort path: the SP events are skipped and logged, and match_end /
// match_state still fire.
//
// Both public methods go through here so neither can regain the dereference.
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

// ApplySeasonPoints resolves the season covering `now` (creating it if needed --
// see Repository.CurrentSeason), applies every seat's award in one transaction,
// and returns each player's post-write snapshot. Satisfies match.SPAwarder.
//
// `now` is a parameter rather than a clock read so the season a match lands in
// is decided by the finalizer's own stamp, and so this is testable at a quarter
// boundary. An empty map is a no-op that never touches the DB -- notably it does
// NOT create a season row for a match with no human seats.
func (s *Service) ApplySeasonPoints(awards map[uint]match.SPAward, now time.Time) (map[uint]match.PlayerSeasonSnapshot, error) {
	if len(awards) == 0 {
		return map[uint]match.PlayerSeasonSnapshot{}, nil
	}

	current, err := s.resolveSeason(now)
	if err != nil {
		return nil, err
	}

	repoAwards := make(map[uint]SPAward, len(awards))
	for userID, a := range awards {
		repoAwards[userID] = SPAward{SP: a.SP, Completed: a.Completed}
	}

	snapshots, err := s.repo.ApplySeasonPoints(current.ID, repoAwards)
	if err != nil {
		return nil, fmt.Errorf("applying season points: %w", err)
	}

	out := make(map[uint]match.PlayerSeasonSnapshot, len(snapshots))
	for userID, snap := range snapshots {
		out[userID] = match.PlayerSeasonSnapshot{
			SeasonName: current.Name,
			SP:         snap.SP,
			RankTier:   snap.Tier,
			// Derived from the pre-award total the repository returned, so no
			// second read is needed. SP is monotonic, so this can only ever be a
			// climb -- but it is still computed rather than assumed, because a
			// zero-SP award (an absent seat) must report false.
			TieredUp: TierForSP(snap.PreviousSP) != snap.Tier,
		}
	}
	return out, nil
}

// CurrentSeasonView is the read path behind GET /api/v1/seasons/current: the
// active window plus the viewer's own record, decomposed for the RankBanner.
//
// A player with no player_seasons row yet gets the ZERO STATE (0 SP, Iron, a
// full Iron band to climb) rather than a 404 or a lazily created row: this path
// never creates a PLAYER record.
//
// It can, however, create the SEASON row -- resolveSeason is the lazy resolver,
// so a GET that lands in a quarter with no window yet inserts it. That is
// deliberate and safe (idempotent, one row per quarter, identical to what the
// write path would create). What must never happen is a read materialising a
// player_seasons row, which would seed Story 13.2's leaderboard with everyone
// who merely opened the lobby.
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

	// Derived, never read off the rank_tier column (D7).
	tier, intoTier, forNextTier := TierProgress(sp)

	return &CurrentSeasonView{
		SeasonName: current.Name,
		// ABSOLUTE timestamp, never a "daysRemaining" duration: a relative value
		// is stale the moment it is serialised and cannot survive a cached
		// response. The client owns the countdown.
		EndsAt:         current.EndsAt,
		SP:             sp,
		RankTier:       tier,
		SPIntoTier:     intoTier,
		SPForNextTier:  forNextTier,
		GamesPlayed:    gamesPlayed,
		GamesCompleted: gamesCompleted,
	}, nil
}
