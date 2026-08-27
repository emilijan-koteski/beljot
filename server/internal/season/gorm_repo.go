package season

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"gorm.io/gorm"
)

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// Compile-time proof that the concrete repo still satisfies the boundary the
// service and handler depend on.
var _ Repository = (*GormRepository)(nil)

// CurrentSeason implements the lazy self-healing resolver (Story 13.1 D1).
//
// Read first: the overwhelmingly common case is a hit, and the read costs one
// indexed lookup. Only on a miss do we compute the calendar quarter and insert
// it, and that insert is idempotent (ON CONFLICT (started_at) DO NOTHING) so two
// match ends racing at a quarter boundary both proceed with the same row rather
// than one failing on a unique violation.
//
// Then re-read rather than trusting the insert: DO NOTHING means "maybe someone
// else won", and the row we want is whichever one is actually in the table.
func (r *GormRepository) CurrentSeason(now time.Time) (*Season, error) {
	existing, err := r.findCovering(now)
	if err != nil {
		return nil, fmt.Errorf("reading current season: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	start, end := QuarterBounds(now)
	if err := r.db.Exec(`
		INSERT INTO seasons (name, started_at, ends_at, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON CONFLICT (started_at) DO NOTHING`,
		QuarterName(start), start, end,
	).Error; err != nil {
		return nil, fmt.Errorf("creating season %s: %w", QuarterName(start), err)
	}

	created, err := r.findCovering(now)
	if err != nil {
		return nil, fmt.Errorf("re-reading current season: %w", err)
	}
	if created == nil {
		// Unreachable unless a concurrent writer inserted a window that does not
		// actually cover `now` under the same started_at. Surfaced rather than
		// returning (nil, nil), which every caller would have to special-case.
		return nil, fmt.Errorf("season: no window covers %s after upsert", now.UTC().Format(time.RFC3339))
	}
	return created, nil
}

// findCovering returns the row whose [started_at, ends_at) window contains now,
// or (nil, nil) on a miss. Ordered started_at DESC so that if overlapping rows
// were ever hand-inserted, the most recently started one wins deterministically
// instead of the result depending on physical row order.
func (r *GormRepository) findCovering(now time.Time) (*Season, error) {
	var s Season
	err := r.db.
		Where("started_at <= ? AND ends_at > ?", now.UTC(), now.UTC()).
		Order("started_at DESC").
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *GormRepository) FindPlayerSeason(userID, seasonID uint) (*PlayerSeason, error) {
	var ps PlayerSeason
	err := r.db.
		Where("user_id = ? AND season_id = ?", userID, seasonID).
		First(&ps).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ps, nil
}

// ApplySeasonPoints accumulates every seat's award into its (user, season) row
// inside ONE transaction. Mirrors the AddXP / ApplySettlement discipline
// (user/gorm_repo.go:212, wallet/gorm_repo.go): users are processed in ASCENDING
// ID order so a concurrent wallet settlement or XP award -- which lock in the
// same order -- cannot deadlock against this.
//
// The write is a single upsert per user rather than read-then-write: the row may
// not exist yet, and SELECT ... FOR UPDATE cannot lock a row that is absent, so
// two concurrent first-ever writes for the same player would both insert. The
// ON CONFLICT (user_id, season_id) target -- backed by the unique index in
// 000024 -- makes the loser an atomic increment instead of a duplicate.
//
// The increment is expressed IN SQL (sp = player_seasons.sp + EXCLUDED.sp), not
// as "read total, add in Go, write total": the latter loses one of two
// concurrent awards even inside a transaction unless the row was locked first.
//
// rank_tier is refreshed in a second statement from the returned total, because
// the tier is Go arithmetic (TierForSP) and D7 keeps that the single source --
// restating the ladder as a SQL CASE would be a second copy that could drift.
//
// WHY THE SECOND STATEMENT IS NOT REDUNDANT, and why you cannot delete it: the
// upsert's own `rank_tier` value, TierForSP(award.SP), is correct ONLY on the
// INSERT branch, where the row starts from zero so award.SP IS the new total. The
// DO UPDATE branch deliberately does not touch rank_tier at all -- it cannot,
// because the new total (player_seasons.sp + EXCLUDED.sp) is not known to Go
// until RETURNING hands it back. So on every award after a player's first, the
// column is still the PREVIOUS tier until the follow-up UPDATE lands. Removing
// that UPDATE, or "simplifying" the two statements into one, silently freezes
// every returning player's stored tier at whatever it was after their first
// match of the season.
func (r *GormRepository) ApplySeasonPoints(seasonID uint, awards map[uint]SPAward) (map[uint]PlayerSeasonSnapshot, error) {
	snapshots := make(map[uint]PlayerSeasonSnapshot, len(awards))
	if len(awards) == 0 {
		return snapshots, nil
	}

	ids := make([]uint, 0, len(awards))
	for id := range awards {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			award := awards[id]
			completed := 0
			if award.Completed {
				completed = 1
			}

			var row struct {
				SP             int
				GamesPlayed    int
				GamesCompleted int
			}
			if err := tx.Raw(`
				INSERT INTO player_seasons
					(user_id, season_id, sp, rank_tier, games_played, games_completed, created_at, updated_at)
				VALUES (?, ?, ?, ?, 1, ?, NOW(), NOW())
				ON CONFLICT (user_id, season_id) DO UPDATE
				SET sp              = player_seasons.sp + EXCLUDED.sp,
					games_played    = player_seasons.games_played + 1,
					games_completed = player_seasons.games_completed + EXCLUDED.games_completed,
					updated_at      = NOW()
				RETURNING sp, games_played, games_completed`,
				id, seasonID, award.SP, TierForSP(award.SP), completed,
			).Scan(&row).Error; err != nil {
				return fmt.Errorf("upserting player_season user=%d season=%d: %w", id, seasonID, err)
			}

			tier := TierForSP(row.SP)
			if err := tx.Exec(
				`UPDATE player_seasons SET rank_tier = ?, updated_at = NOW() WHERE user_id = ? AND season_id = ?`,
				tier, id, seasonID,
			).Error; err != nil {
				return fmt.Errorf("refreshing rank_tier user=%d season=%d: %w", id, seasonID, err)
			}

			snapshots[id] = PlayerSeasonSnapshot{
				SP:             row.SP,
				PreviousSP:     row.SP - award.SP,
				Tier:           tier,
				GamesPlayed:    row.GamesPlayed,
				GamesCompleted: row.GamesCompleted,
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}

// leaderboardScope is THE ONE PLACE the leaderboard's visibility predicate is
// written. Every leaderboard read starts here -- the page, its total, and the
// viewer's CountAhead -- so the three cannot drift apart. Divergence would not
// fail loudly: it would just make the viewer's reported position disagree with
// the slot they are standing in.
//
// A FRESH BUILDER PER CALL, on purpose. The alternative (one *gorm.DB handed to
// Count and then to Scan) reuses a statement across two executions; a helper
// returning a new scope gives the same single-source guarantee with none of that
// coupling, and reads the same at every call site.
//
// `users.deleted_at IS NULL` IS THE LINE TO CHECK FIRST IN REVIEW. Table()/Joins()
// takes GORM out of model-land, so the soft-delete scope that PlayerSeason reads
// get for free does NOT apply here -- exactly the hole left open at
// internal/room/gorm_repo.go:298. Without it a deleted account keeps its slot at
// the top of the ladder forever.
//
// `sp > 0` IS THE SECOND HALF OF THE PREDICATE, and it is a MEMBERSHIP RULE
// rather than an optimisation (owner decision 2026-08-27). A player_seasons row
// is written for every human seat in a finished match, INCLUDING seats that were
// absent at the terminal end and earned nothing -- so 0-SP rows arise in normal
// operation. Listing them would put a player on the ladder at a real position
// while the viewer block tells that same player they have no standing (the AC
// marks an own-row only for a player with ANY SP), and would falsify the empty
// state ("Nobody has earned Season Points yet") the moment one such row lands.
//
// THE LADDER IS SP EARNERS ONLY. Because this lives in the shared scope, the
// exclusion applies identically to `items`, to `total` and to CountAhead -- a
// 0-SP row cannot be listed, cannot inflate the count, and cannot push anyone
// down a slot. Do not move it into the page query alone.
func (r *GormRepository) leaderboardScope(seasonID uint) *gorm.DB {
	return r.db.
		Table("player_seasons").
		Joins("JOIN users ON users.id = player_seasons.user_id").
		Where("player_seasons.season_id = ?", seasonID).
		Where("users.deleted_at IS NULL").
		Where("player_seasons.sp > 0")
}

// allocHintCap bounds the pre-allocation hint taken from `limit`.
//
// make([]LeaderboardEntry, 0, limit) trusts its argument with the process memory
// budget: a limit of 1e9 reserves tens of gigabytes before a single row is read.
// The handler caps requests far below this, so the ceiling never binds in
// practice -- it exists so a future in-process caller (Story 13.3) cannot turn a
// typo into an OOM. It is deliberately NOT a page-size policy: an over-large
// limit still runs, it just grows the slice incrementally instead.
const allocHintCap = 1000

// LeaderboardPage implements Repository.LeaderboardPage.
//
// THE BOUNDS CHECK IS NOT PARANOIA. `limit` reaches both make(..., 0, limit),
// which PANICS on a negative, and GORM Limit(), where a negative means "no
// limit" and quietly returns the entire season. Neither failure mode is one a
// caller would attribute to its own bad argument. The handler validates user
// input and answers 400; this guards the GO boundary against a caller that never
// saw a query string, and returns an ordinary error (a 500) because reaching it
// is a programming bug rather than a bad request.
//
// Count first, then the page: the total drives load-more, and an offset past the
// end must still report the real total rather than 0 (see the story edge-case
// matrix). Both statements run through leaderboardScope, so they apply the SAME
// PREDICATE -- but they are two statements, not one snapshot: under READ
// COMMITTED a write landing between them can leave `total` disagreeing with the
// rows by a row or two. That is accepted (the client re-reads on its next poll).
// What the shared scope guarantees is that the two never describe different
// POPULATIONS -- the failure that would be permanent rather than transient.
//
// Columns are aliased explicitly rather than selected as `*`: the join puts two
// `id` and two `created_at` columns on the result set, and an unaliased scan
// would silently take whichever the driver returned last.
//
// ORDER BY sp DESC, user_id ASC -- the same order CountAhead counts under.
func (r *GormRepository) LeaderboardPage(seasonID uint, limit, offset int) ([]LeaderboardEntry, int64, error) {
	if limit < 1 {
		return nil, 0, fmt.Errorf("season: leaderboard limit must be >= 1, got %d", limit)
	}
	if offset < 0 {
		return nil, 0, fmt.Errorf("season: leaderboard offset must be >= 0, got %d", offset)
	}

	var total int64
	if err := r.leaderboardScope(seasonID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting leaderboard rows season=%d: %w", seasonID, err)
	}

	entries := make([]LeaderboardEntry, 0, min(limit, allocHintCap))
	err := r.leaderboardScope(seasonID).
		Select(`player_seasons.user_id      AS user_id,
		        users.username              AS username,
		        player_seasons.sp           AS sp,
		        player_seasons.games_played AS games_played`).
		Order("player_seasons.sp DESC").
		Order("player_seasons.user_id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&entries).Error
	if err != nil {
		return nil, 0, fmt.Errorf("reading leaderboard page season=%d: %w", seasonID, err)
	}
	return entries, total, nil
}

// CountAhead implements Repository.CountAhead.
//
// One order, two consumers: the predicate below is the strict-ahead form of
// LeaderboardPage's ORDER BY, and if the two ever diverge the viewer's position
// silently drifts away from their own row in the list.
// FindLeaderboardEntry implements Repository.FindLeaderboardEntry.
//
// WHY THIS EXISTS INSTEAD OF REUSING FindPlayerSeason. FindPlayerSeason is a
// MODEL query over player_seasons alone: it carries no `users` join, so it
// happily returns the row of a SOFT-DELETED account. A deleted player holding an
// unexpired JWT would then get a viewer block and a pinned row while being
// absent from the very list they are looking at -- a position counted against a
// population that excludes them. Routing the viewer through leaderboardScope
// closes that by construction: a player who is not listable has no standing.
//
// (nil, nil) on a miss, which now covers three cases the caller treats
// identically: no row at all, a 0-SP row, and a soft-deleted account.
func (r *GormRepository) FindLeaderboardEntry(seasonID, userID uint) (*LeaderboardEntry, error) {
	var entries []LeaderboardEntry
	err := r.leaderboardScope(seasonID).
		Select(`player_seasons.user_id      AS user_id,
		        users.username              AS username,
		        player_seasons.sp           AS sp,
		        player_seasons.games_played AS games_played`).
		Where("player_seasons.user_id = ?", userID).
		Limit(1).
		Scan(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("reading leaderboard entry season=%d user=%d: %w", seasonID, userID, err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return &entries[0], nil
}

func (r *GormRepository) CountAhead(seasonID uint, sp int, userID uint) (int64, error) {
	var ahead int64
	err := r.leaderboardScope(seasonID).
		Where("(player_seasons.sp > ? OR (player_seasons.sp = ? AND player_seasons.user_id < ?))",
			sp, sp, userID).
		Count(&ahead).Error
	if err != nil {
		return 0, fmt.Errorf("counting rows ahead season=%d user=%d: %w", seasonID, userID, err)
	}
	return ahead, nil
}
