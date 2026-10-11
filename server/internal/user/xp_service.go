package user

// XPService is the match manager's XPAwarder, delegating total_xp mutations to the repository.
// It lets the match package read levels without importing user.
type XPService struct {
	repo UserRepository
}

func NewXPService(repo UserRepository) *XPService {
	return &XPService{repo: repo}
}

// ApplyXPAwards atomically adds each (userID -> delta) to total_xp and returns
// each user's new total.
func (s *XPService) ApplyXPAwards(awards map[uint]int) (map[uint]int, error) {
	return s.repo.AddXP(awards)
}

// LevelForXP derives the level from a lifetime total via the pure curve.
func (s *XPService) LevelForXP(totalXP int) int {
	return LevelForXP(totalXP)
}

// LevelsForUsers returns each given userID's current lifetime level from their total_xp.
// Bot id 0 and unknown ids are absent from the result.
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
