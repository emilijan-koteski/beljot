package user

import (
	"time"

	"github.com/emilijan/beljot/server/internal/match"
)

// HonorService is the thin match-end honor recorder injected into the match
// manager as its HonorRecorder (Story 9.7). It owns no logic beyond delegating
// the atomic write to the repository and translating between the two packages'
// DTOs — the honor math is pure (honor.go) and the per-seat bucketing lives in
// the match package.
//
// Honor lives in internal/user (not a new internal/honor package) for the same
// reason XP does: the profile handler needs the honor math and the counters are
// users columns, so a separate package would create a user<->honor cycle
// (Story 9.7 D4). Mirrors XPService's and wallet.Service's thin pass-through.
//
// The DTO translation below is the whole reason this type exists. `user` may
// import `match` (the profile handler already holds a match.MatchRepository),
// but `match` MUST NEVER import `user` — so the interface types travelling over
// HonorRecorder are declared in match, and this adapter maps them onto the
// repository's user-domain equivalents.
type HonorService struct {
	repo UserRepository
}

// Compile-time proof that the injection in cmd/api/main.go keeps working. The
// match manager takes HonorRecorder structurally, so a signature drift would
// otherwise only surface as a wiring failure at startup.
var _ match.HonorRecorder = (*HonorService)(nil)

func NewHonorService(repo UserRepository) *HonorService {
	return &HonorService{repo: repo}
}

// ApplyHonorEvents records one finished match per listed user and returns each
// user's post-write honor state. A zero/empty map is a no-op. Satisfies the
// match package's HonorRecorder interface.
func (s *HonorService) ApplyHonorEvents(events map[uint]match.HonorEvent, now time.Time) (map[uint]match.HonorSnapshot, error) {
	if len(events) == 0 {
		return map[uint]match.HonorSnapshot{}, nil
	}

	repoEvents := make(map[uint]HonorEvent, len(events))
	for userID, ev := range events {
		repoEvents[userID] = HonorEvent{Abandoned: ev.Abandoned}
	}

	snapshots, err := s.repo.ApplyHonorEvents(repoEvents, now)
	if err != nil {
		return nil, err
	}

	out := make(map[uint]match.HonorSnapshot, len(snapshots))
	for userID, snap := range snapshots {
		out[userID] = match.HonorSnapshot{
			Score:          snap.Score,
			Tier:           snap.Tier,
			CompletedTotal: snap.CompletedTotal,
			AbandonedTotal: snap.AbandonedTotal,
			IsNewPlayer:    snap.IsNewPlayer,
		}
	}
	return out, nil
}

// HonorForUsers returns each requested user's AUTHORITATIVE honor snapshot, keyed
// by user ID. It is the read path Story 9.8's room gate goes through.
//
// Two things it deliberately does NOT do:
//
//  1. It never reads users.honor_score. That column is the lagging, filter-only
//     snapshot (see its comment in model.go and the 000017 header) — a decayed score
//     moves with wall-clock time even when nothing is written. Every row is passed
//     through NewHonorSnapshot instead, recomputing from the three source columns of
//     a row already in hand.
//  2. It adds NOTHING to the UserRepository interface. FindManyByIDs already returns
//     every honor column, so this is a pure addition to the concrete service — which
//     is what keeps the ~20 repository mocks across user/, match/, auth/, chat/ and
//     lobby/ untouched (Story 9.8 D6).
//
// `now` is stamped ONCE for the whole batch so two seats checked in the same request
// cannot be decayed to different instants. Empty input is a no-op with no DB
// round-trip (mirroring LevelsForUsers). Unknown IDs — and bot id 0 — are simply
// absent from the result; the caller decides whether that is an error (the room gate
// treats it as one, because a zero-value snapshot would read as an experienced player
// with a score of 0).
func (s *HonorService) HonorForUsers(userIDs []uint) (map[uint]HonorSnapshot, error) {
	if len(userIDs) == 0 {
		return map[uint]HonorSnapshot{}, nil
	}
	users, err := s.repo.FindManyByIDs(userIDs)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make(map[uint]HonorSnapshot, len(users))
	for _, u := range users {
		out[u.ID] = NewHonorSnapshot(
			u.HonorCompletedWeight,
			u.HonorAbandonedWeight,
			u.HonorDecayedAt,
			u.HonorCompletedTotal,
			u.HonorAbandonedTotal,
			now,
		)
	}
	return out, nil
}

// ResetHonor is the operator forgiveness hook (Story 9.7 AC9 / D7). It is
// deliberately NOT reachable over HTTP: there is no admin system in this
// project, so 9.7 ships the capability and the migration header documents the
// equivalent SQL recipe. Exposed on the service so a future admin story wires a
// handler rather than reinventing the transaction.
func (s *HonorService) ResetHonor(userID uint) error {
	return s.repo.ResetHonor(userID)
}
