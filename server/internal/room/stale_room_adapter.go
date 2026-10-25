package room

import "github.com/emilijan/beljot/server/internal/match"

// StaleRoomRepositoryAdapter wraps a RoomRepository to satisfy match.StaleRoomRepository
// without match importing room.
type StaleRoomRepositoryAdapter struct {
	Repo RoomRepository
}

func (a *StaleRoomRepositoryAdapter) FindByStatus(status string) ([]match.StaleRoom, error) {
	rooms, err := a.Repo.FindByStatus(status)
	if err != nil {
		return nil, err
	}
	out := make([]match.StaleRoom, len(rooms))
	for i, r := range rooms {
		out[i] = match.StaleRoom{
			ID:                   r.ID,
			Variant:              r.Variant,
			MatchMode:            r.MatchMode,
			DeclarationsEnabled:  r.DeclarationsEnabled,
			StopAtTarget:         r.StopAtTarget,
			DeclarationsTransfer: r.DeclarationsTransfer,
			UpdatedAt:            r.UpdatedAt,
		}
	}
	return out, nil
}

func (a *StaleRoomRepositoryAdapter) FindPlayersByRoomID(roomID uint) ([]match.StaleRoomPlayer, error) {
	players, err := a.Repo.FindPlayersByRoomID(roomID)
	if err != nil {
		return nil, err
	}
	out := make([]match.StaleRoomPlayer, len(players))
	for i, p := range players {
		out[i] = match.StaleRoomPlayer{
			Seat:   p.Seat,
			UserID: p.UserID,
		}
	}
	return out, nil
}

func (a *StaleRoomRepositoryAdapter) FindBotSeatsByRoomID(roomID uint) ([]int, error) {
	bots, err := a.Repo.FindBotsByRoomID(roomID)
	if err != nil {
		return nil, err
	}
	seats := make([]int, len(bots))
	for i, b := range bots {
		seats[i] = b.Seat
	}
	return seats, nil
}

func (a *StaleRoomRepositoryAdapter) UpdateStatus(roomID uint, status string) error {
	return a.Repo.UpdateStatus(roomID, status)
}
