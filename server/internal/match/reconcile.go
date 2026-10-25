package match

import (
	"fmt"
	"log/slog"
	"time"
)

// StaleRoom is the subset of room data that ReconcileStaleRooms needs from the
// repository. The narrow type keeps the match package free of a room import
// (which would create an import cycle via room→auth→user→match).
type StaleRoom struct {
	ID                   uint
	Variant              string
	MatchMode            string
	DeclarationsEnabled  bool
	StopAtTarget         bool
	DeclarationsTransfer bool
	Clockwise            bool
	UpdatedAt            time.Time
}

// StaleRoomPlayer is the seat-occupancy data ReconcileStaleRooms needs.
type StaleRoomPlayer struct {
	Seat   *int
	UserID uint
}

// StaleRoomRepository is the narrow subset of room.RoomRepository that
// ReconcileStaleRooms uses. *room.GormRepository satisfies it at the call
// site in main.go without any import of room from this package.
type StaleRoomRepository interface {
	FindByStatus(status string) ([]StaleRoom, error)
	FindPlayersByRoomID(roomID uint) ([]StaleRoomPlayer, error)
	FindBotSeatsByRoomID(roomID uint) ([]int, error)
	UpdateStatus(roomID uint, status string) error
}

// ReconcileStaleRooms closes out rooms left "playing" by a previous process,
// recording an abandoned match where possible and marking the room completed.
func (m *Manager) ReconcileStaleRooms(roomRepo StaleRoomRepository) error {
	rooms, err := roomRepo.FindByStatus("playing")
	if err != nil {
		return fmt.Errorf("listing playing rooms: %w", err)
	}
	if len(rooms) == 0 {
		return nil
	}
	slog.Info("session: reconciling stale playing rooms", "count", len(rooms))
	for i := range rooms {
		if err := m.reconcileStaleRoom(roomRepo, rooms[i]); err != nil {
			slog.Error("session: reconcile failed", "roomID", rooms[i].ID, "error", err)
		}
	}
	return nil
}

func (m *Manager) reconcileStaleRoom(roomRepo StaleRoomRepository, r StaleRoom) error {
	players, err := roomRepo.FindPlayersByRoomID(r.ID)
	if err != nil {
		return fmt.Errorf("loading players: %w", err)
	}

	botSeatList, err := roomRepo.FindBotSeatsByRoomID(r.ID)
	if err != nil {
		return fmt.Errorf("loading bot seats: %w", err)
	}

	var seated [4]uint
	var botSeats [4]bool
	seatedCount := 0
	for _, p := range players {
		if p.Seat != nil && *p.Seat >= 0 && *p.Seat < 4 {
			seated[*p.Seat] = p.UserID
			seatedCount++
		}
	}
	for _, s := range botSeatList {
		if s >= 0 && s < 4 && !botSeats[s] && seated[s] == 0 {
			botSeats[s] = true
			seatedCount++
		}
	}
	if m.matchRepo != nil && seatedCount == 4 {
		ids, botFlags, hasBots := matchSeatColumns(seated, botSeats)
		record := &Match{
			RoomID:               r.ID,
			Player1ID:            ids[0],
			Player2ID:            ids[1],
			Player3ID:            ids[2],
			Player4ID:            ids[3],
			Player1IsBot:         botFlags[0],
			Player2IsBot:         botFlags[1],
			Player3IsBot:         botFlags[2],
			Player4IsBot:         botFlags[3],
			HasBots:              hasBots,
			TeamAScore:           0,
			TeamBScore:           0,
			WinnerTeam:           0,
			Variant:              r.Variant,
			MatchMode:            r.MatchMode,
			DeclarationsEnabled:  r.DeclarationsEnabled,
			StopAtTarget:         r.StopAtTarget,
			DeclarationsTransfer: r.DeclarationsTransfer,
			Clockwise:            r.Clockwise,
			StartedAt:            r.UpdatedAt,
			CompletedAt:          time.Now(),
			Status:               "abandoned",
		}
		if err := m.matchRepo.CreateWithHands(record, nil); err != nil {
			slog.Error("session: failed to persist reconciled abandoned match",
				"roomID", r.ID, "error", err)
		}
	}

	if err := roomRepo.UpdateStatus(r.ID, "completed"); err != nil {
		return fmt.Errorf("updating room status: %w", err)
	}
	slog.Info("session: stale room reconciled",
		"roomID", r.ID, "seatedPlayers", seatedCount)
	return nil
}
