package user

// XPService is the thin match-end XP applier injected into the match manager as
// its XPAwarder (Story 9.5). It owns no logic beyond delegating the atomic
// total_xp mutation to the repository — the level curve is pure (level.go) and
// the per-seat award computation lives in the match package. XP lives in
// internal/user (not a new internal/xp package) to avoid a user<->xp import
// cycle: the profile handler needs LevelForXP and total_xp is a users column
// (see Story 9.5 Design Decision D1). Mirrors wallet.Service's thin pass-through.
type XPService struct {
	repo UserRepository
}

func NewXPService(repo UserRepository) *XPService {
	return &XPService{repo: repo}
}

// ApplyXPAwards atomically adds each (userID -> delta) to total_xp and returns
// each user's new total. A zero/empty map is a no-op. Satisfies the match
// package's structural XPAwarder interface.
func (s *XPService) ApplyXPAwards(awards map[uint]int) (map[uint]int, error) {
	return s.repo.AddXP(awards)
}

// LevelForXP derives the level from a lifetime total via the pure curve. It is
// part of the XPAwarder interface so the match manager can compute the
// NewLevel/LeveledUp fields of event:xp_awarded without importing the user
// package (user imports match → match must not import user; Story 9.5 D1).
func (s *XPService) LevelForXP(totalXP int) int {
	return LevelForXP(totalXP)
}

// LevelsForUsers returns each given userID's current lifetime level by reading
// their total_xp and applying the pure LevelForXP curve. The match manager
// calls this at match start to stamp each human seat's static level onto the
// game state (XP only changes at match end, so the level can't drift mid-match).
// Bot id 0 and any unknown id are absent from both the repo lookup and the
// result. A zero/empty input is a no-op. Part of the match-side XPAwarder
// interface (match must not import user; Story 9.5 D1).
func (s *XPService) LevelsForUsers(ids []uint) (map[uint]int, error) {
	totals, err := s.repo.TotalXPForUsers(ids)
	if err != nil {
		return nil, err
	}
	levels := make(map[uint]int, len(totals))
	for id, total := range totals {
		levels[id] = LevelForXP(total)
	}
	return levels, nil
}
