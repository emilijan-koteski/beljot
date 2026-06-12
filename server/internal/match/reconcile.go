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
	ID        uint
	Variant   string
	MatchMode string
	UpdatedAt time.Time
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

// ReconcileStaleRooms cleans up rooms left in status="playing" by a previous
// process. Sessions live only in process memory, so any "playing" row at boot
// has no live session — its players are stranded by FindPlayerRoom (which
// counts "playing" as active) and cannot quick-play or create a new room
// until that row is closed out.
//
// For each stranded room we (a) best-effort persist a match row with
// status="abandoned" so the player history reflects what happened, then
// (b) flip room.status to "completed" so the lobby treats those players as
// free again. Idempotent: a no-op when nothing is stale, safe to call on
// every boot.
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

	// Persist history when all four seats were held by occupants the
	// repository can still see — humans (room_players) or bots (room_bots).
	// Bot seats persist exactly like a live match end: nil player IDs +
	// per-seat IsBot flags + HasBots.
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
		// No precise game-start timestamp survives the restart — the room's
		// UpdatedAt is the best proxy (StartMatch is the last writer for a
		// "playing" row that hasn't moved on).
		ids, botFlags, hasBots := matchSeatColumns(seated, botSeats)
		record := &Match{
			RoomID:       r.ID,
			Player1ID:    ids[0],
			Player2ID:    ids[1],
			Player3ID:    ids[2],
			Player4ID:    ids[3],
			Player1IsBot: botFlags[0],
			Player2IsBot: botFlags[1],
			Player3IsBot: botFlags[2],
			Player4IsBot: botFlags[3],
			HasBots:      hasBots,
			TeamAScore:   0,
			TeamBScore:   0,
			WinnerTeam:   0,
			Variant:      r.Variant,
			MatchMode:    r.MatchMode,
			StartedAt:    r.UpdatedAt,
			CompletedAt:  time.Now(),
			Status:       "abandoned",
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
