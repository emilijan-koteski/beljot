package season

import (
	"fmt"
	"time"

	"github.com/emilijan/beljot/server/internal/apperr"
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

// LeaderboardView is the read path behind GET /api/v1/leaderboard (Story 13.2,
// prior-season selector added by Story 13.3): one SP-ordered page of one season
// plus the viewer's own position under that same order.
//
// `seasonID` selects the window: 0 means THE CURRENT SEASON (the handler maps
// the absent / "current" selector to it), any other value is a by-id lookup of
// a specific — usually ended — season, and a miss is apperr.ErrSeasonNotFound
// (404), never a silent fallback to the active window, which would show the
// wrong standings under the right heading.
//
// The viewer block runs under the SAME rules for a prior season as for the
// current one (the sp > 0 membership predicate included) — a season's ladder is
// frozen when it ends, but the question "where did I finish" has the same
// answer shape either way.
//
// PULL-ONLY. There is deliberately no WebSocket event for standings (epic
// decision, restated as a Story 13.2 boundary): the client loads this on mount
// and re-reads it on a poll. Nothing here is pushed and nothing invalidates it
// from the socket.
//
// Like CurrentSeasonView this never creates a PLAYER record -- it only reads
// player_seasons -- while resolveSeason may still lazily create the SEASON
// window, which is idempotent and bounded to one row per quarter. The BY-ID
// path creates nothing at all: FindSeasonByID is a pure read.
//
// `limit` and `offset` arrive already validated by the handler
// (parseLeaderboardQuery); the service does not re-clamp them, so a caller that
// bypasses the handler gets exactly what it asked for.
func (s *Service) LeaderboardView(userID, seasonID uint, limit, offset int, now time.Time) (*LeaderboardView, error) {
	var window *Season
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
	}

	entries, total, err := s.repo.LeaderboardPage(window.ID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("reading leaderboard page: %w", err)
	}

	items := make([]LeaderboardRowView, 0, len(entries))
	for i, e := range entries {
		items = append(items, LeaderboardRowView{
			// The list's own numbering. Derived from the page window rather than
			// read back from SQL, which is only correct because the repository
			// guarantees a TOTAL order (sp DESC, user_id ASC) -- with a partial
			// order, row 41 of one request need not be row 41 of the next.
			Position: offset + i + 1,
			UserID:   e.UserID,
			Username: e.Username,
			SP:       e.SP,
			// DERIVED, never the stored rank_tier column (Story 13.1 D7). The
			// repository does not even select that column.
			Tier:        TierForSP(e.SP),
			GamesPlayed: e.GamesPlayed,
		})
	}

	viewer, err := s.viewerPosition(userID, window.ID)
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

// viewerPosition resolves the caller's own standing, or nil when there is
// nothing to pin.
//
// NIL IN THREE CASES, deliberately indistinguishable on the wire:
//
//	no row       the viewer has not played this season at all.
//	sp == 0      the viewer has a row (they played, and were absent at every
//	             terminal end, or the formula paid nothing) but earned no SP.
//	soft-deleted the account is gone but still holds an unexpired JWT.
//
// ALL THREE ARE DECIDED BY THE REPOSITORY, not re-derived here, and that is the
// point: FindLeaderboardEntry applies the LIST'S OWN visibility predicate. The
// earlier version of this function called FindPlayerSeason and checked
// `record.SP <= 0` in Go, which got the first two cases right and the third
// wrong -- FindPlayerSeason has no `users` join, so a deleted account received a
// viewer block and a pinned row while being absent from the list, with a position
// counted against a population that excluded it.
//
// Keeping the rule in ONE place also means the answer cannot drift: if a row is
// listable, it has a standing; if it is not, it has none. There is no third
// state for this function to invent.
//
// The AC marks the viewer's own row only when they have ANY SP. Note this is NOT
// the same as saying 0 SP is unranked -- a 0-SP player is Iron and the RankBanner
// renders them normally; they simply have no meaningful ladder position to point
// at, and every 0-SP player would otherwise share the same last-place block.
//
// The position comes from CountAhead under the LIST'S OWN ORDER, so a viewer who
// is on the page they are looking at reads the same number twice.
func (s *Service) viewerPosition(userID, seasonID uint) (*LeaderboardViewerView, error) {
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

	return &LeaderboardViewerView{
		Position: int(ahead) + 1,
		UserID:   userID,
		SP:       entry.SP,
		Tier:     TierForSP(entry.SP),
		// entry.Username is deliberately DROPPED rather than forwarded: the viewer
		// IS the authenticated caller, so the client already holds their name in
		// authStore, and the wire block stays name-free (Story 13.2 D3). It is
		// selected only because it rides the shared scope's SELECT list.
		GamesPlayed: entry.GamesPlayed,
	}, nil
}

// --- Story 13.3: seasons list, prior-season archive, profile rank ---

// SeasonsView is the read path behind GET /api/v1/seasons: every window,
// newest-first, feeding the leaderboard's season picker.
//
// It resolves the CURRENT season first — through the same lazy resolver every
// other read uses — so the listing always contains the window covering `now`
// even on a zero-traffic deployment the nightly job has not reached yet. That
// is the one write this path can cause (a season row, idempotent, one per
// quarter); it never touches player_seasons.
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

// ArchiveView is the read path behind GET /api/v1/users/:id/seasons: the
// subject's ENDED, PLAYED seasons, newest-first, with the tier DERIVED per row
// (TierForSP over the immutable SP total — never the stored rank_tier column,
// 13.1 D7).
//
// An unknown subject is an EMPTY archive, not a 404 — the profile query owns
// user existence, and this endpoint answers the narrower question "which ended
// seasons did this id play". A pure read: no season row, no player row.
func (s *Service) ArchiveView(userID uint, now time.Time) (*ArchiveView, error) {
	entries, err := s.repo.PlayerSeasonArchive(userID, now)
	if err != nil {
		return nil, fmt.Errorf("reading season archive: %w", err)
	}

	items := make([]ArchiveRowView, 0, len(entries))
	for _, e := range entries {
		items = append(items, ArchiveRowView{
			SeasonID:    e.SeasonID,
			SeasonName:  e.SeasonName,
			SP:          e.SP,
			Tier:        TierForSP(e.SP),
			GamesPlayed: e.GamesPlayed,
			StartedAt:   e.StartedAt,
			EndsAt:      e.EndsAt,
		})
	}
	return &ArchiveView{Items: items}, nil
}

// CurrentSeasonRank is the narrow read the user package's profile assembly
// injects (Story 13.3): the subject's standing in the ACTIVE season, or nil
// when they have not played in it — the profile serializes that nil as
// `seasonRank: null` and the client hides the chip.
//
// nil MEANS "NO ROW", NOT "NO SP": a played season at 0 SP still has a rank
// (Iron — there is no unranked state), so the row's existence is the gate, not
// leaderboardScope's sp > 0 membership rule. Satisfies user.SeasonRankReader
// structurally; `season` never imports `user` (the same one-way discipline as
// match.SPAwarder, mirrored).
//
// Like every read: may lazily create the SEASON window via resolveSeason,
// never a player_seasons row.
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

	return &SeasonRankView{
		SeasonName: current.Name,
		// Derived, never the stored rank_tier column (D7).
		Tier: TierForSP(record.SP),
		SP:   record.SP,
	}, nil
}
