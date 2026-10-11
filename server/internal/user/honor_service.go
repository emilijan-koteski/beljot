package user

import (
	"time"

	"github.com/emilijan/beljot/server/internal/match"
)

// HonorService records match-end honor events for the match manager and translates between
// the match and user honor types.
type HonorService struct {
	repo UserRepository
}

var _ match.HonorRecorder = (*HonorService)(nil)

func NewHonorService(repo UserRepository) *HonorService {
	return &HonorService{repo: repo}
}

// ApplyHonorEvents records one finished match per listed user and returns each user's
// post-write honor state.
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

// HonorForUsers returns each user's honor snapshot, recomputed from the stored weights at one
// shared instant. Unknown IDs are absent from the result.
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
		snap := NewHonorSnapshot(
			u.HonorCompletedWeight,
			u.HonorAbandonedWeight,
			u.HonorDecayedAt,
			u.HonorCompletedTotal,
			u.HonorAbandonedTotal,
			now,
		)
		snap.Level = LevelForXP(u.TotalXP)
		out[u.ID] = snap
	}
	return out, nil
}

// ResetHonor clears a user's honor penalty for operator use; it is not exposed over HTTP.
func (s *HonorService) ResetHonor(userID uint) error {
	return s.repo.ResetHonor(userID)
}
