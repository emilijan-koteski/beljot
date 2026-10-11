package room

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/ws"
)

const (
	defaultQuickFillFastInterval    = 3 * time.Second
	defaultQuickFillPatientInterval = 20 * time.Second
)

// quickFillTimer is one room's auto-fill timer. generation is bumped on every (re)arm or cancel
// so a superseded callback is inert, and interval is the cadence chosen once at arm time.
type quickFillTimer struct {
	timer      *time.Timer
	generation uint64
	interval   time.Duration
}

// quickFillScheduler is the roomID-keyed registry of auto-fill timers; the tick body lives on
// *RoomHandler so it can reuse the repo, broadcasts and autoStartIfFull.
type quickFillScheduler struct {
	mu              sync.Mutex
	pending         map[uint]*quickFillTimer
	fastInterval    time.Duration
	patientInterval time.Duration
}

func newQuickFillScheduler(fast, patient time.Duration) *quickFillScheduler {
	return &quickFillScheduler{
		pending:         make(map[uint]*quickFillTimer),
		fastInterval:    fast,
		patientInterval: patient,
	}
}

// connectedUsersProvider is the optional hub capability the idle-lobby count type-asserts for.
type connectedUsersProvider interface {
	ConnectedUserIDs() []uint
}

type inMatchUsersProvider interface {
	InMatchUserIDs() []uint
}

// idleLobbyCount returns how many connected users are in neither a waiting room nor a match,
// excluding exclude. It returns 0, selecting the fast cadence, when a capability or read is missing.
func (h *RoomHandler) idleLobbyCount(exclude uint) int {
	cp, ok := h.hub.(connectedUsersProvider)
	if !ok {
		return 0
	}
	mp, ok := h.matchStarter.(inMatchUsersProvider)
	if !ok {
		return 0
	}

	inMatch := make(map[uint]struct{})
	for _, uid := range mp.InMatchUserIDs() {
		inMatch[uid] = struct{}{}
	}

	waiting, err := h.repo.FindUserIDsByRoomStatus("waiting")
	if err != nil {
		slog.Error("quick fill: reading waiting-room users for idle count", "error", err)
		return 0
	}
	inRoom := make(map[uint]struct{}, len(waiting))
	for _, uid := range waiting {
		inRoom[uid] = struct{}{}
	}

	idle := 0
	for _, uid := range cp.ConnectedUserIDs() {
		if uid == exclude {
			continue
		}
		if _, busy := inMatch[uid]; busy {
			continue
		}
		if _, busy := inRoom[uid]; busy {
			continue
		}
		idle++
	}
	return idle
}

// startQuickFill arms the auto-fill scheduler for a room, choosing the patient interval when anyone
// else is idle in the lobby and the fast interval otherwise.
func (h *RoomHandler) startQuickFill(roomID, initiatorID uint) {
	if h.quickFill == nil {
		return
	}
	s := h.quickFill

	s.mu.Lock()
	interval := s.fastInterval
	patient := s.patientInterval
	s.mu.Unlock()

	if h.idleLobbyCount(initiatorID) >= 1 {
		interval = patient
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.pending[roomID]
	if !ok {
		entry = &quickFillTimer{}
		s.pending[roomID] = entry
	} else if entry.timer != nil {
		entry.timer.Stop()
	}
	entry.interval = interval
	entry.generation++
	gen := entry.generation
	entry.timer = time.AfterFunc(interval, func() {
		h.quickFillTick(roomID, gen)
	})
}

// resetQuickFill restarts a room's inactivity countdown, keeping the cadence chosen at arm time.
// It is a no-op when no timer is armed.
func (h *RoomHandler) resetQuickFill(roomID uint) {
	if h.quickFill == nil {
		return
	}
	s := h.quickFill
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.pending[roomID]
	if !ok {
		return
	}
	if entry.timer != nil {
		entry.timer.Stop()
	}
	entry.generation++
	gen := entry.generation
	entry.timer = time.AfterFunc(entry.interval, func() {
		h.quickFillTick(roomID, gen)
	})
}

// cancelQuickFill stops and forgets a room's auto-fill timer and is idempotent. A tick already past
// the generation guard still runs, and stops itself by re-reading the room status under the row lock.
func (h *RoomHandler) cancelQuickFill(roomID uint) {
	if h.quickFill == nil {
		return
	}
	s := h.quickFill
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.pending[roomID]
	if !ok {
		return
	}
	if entry.timer != nil {
		entry.timer.Stop()
	}
	entry.generation++
	delete(s.pending, roomID)
}

// quickFillTick seats one bot in the lowest free seat of a still-waiting Quick Play room under the
// row lock, then tries auto-start and re-arms; stale generations and unfillable rooms seat nothing.
func (h *RoomHandler) quickFillTick(roomID uint, generation uint64) {
	s := h.quickFill
	if s == nil {
		return
	}

	s.mu.Lock()
	entry, ok := s.pending[roomID]
	if !ok || entry.generation != generation {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	seatedBot := -1
	terminate := false
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		r, err := tx.FindByIDForUpdate(roomID)
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		if r == nil || !r.IsQuickPlay || r.Status != "waiting" {
			terminate = true
			return nil
		}

		players, err := tx.FindPlayersByRoomID(roomID)
		if err != nil {
			return fmt.Errorf("finding players: %w", err)
		}
		bots, err := tx.FindBotsByRoomID(roomID)
		if err != nil {
			return fmt.Errorf("finding bots: %w", err)
		}

		seatedHumans := 0
		for _, p := range players {
			if p.Seat != nil {
				seatedHumans++
			}
		}
		if seatedHumans+len(bots) >= 4 {
			terminate = true
			return nil
		}

		seat := -1
		for cand := 0; cand < 4; cand++ {
			occupied := false
			for _, p := range players {
				if p.Seat != nil && *p.Seat == cand {
					occupied = true
					break
				}
			}
			if !occupied {
				for _, b := range bots {
					if b.Seat == cand {
						occupied = true
						break
					}
				}
			}
			if !occupied {
				seat = cand
				break
			}
		}
		if seat == -1 {
			terminate = true
			return nil
		}

		if err := tx.AddBot(roomID, seat); err != nil {
			return fmt.Errorf("adding bot: %w", err)
		}
		seatedBot = seat
		return nil
	}); err != nil {
		slog.Error("quick fill: tick transaction failed", "roomID", roomID, "error", err)
		h.cancelQuickFill(roomID)
		return
	}

	if terminate {
		h.cancelQuickFill(roomID)
		return
	}

	if seatedBot >= 0 {
		h.broadcastToRoom(roomID, ws.SystemBotAdded, map[string]interface{}{
			"roomId": roomID,
			"seat":   seatedBot,
			"team":   teamForSeat(seatedBot),
		})
		if humansOnly, herr := h.repo.FindPlayersByRoomID(roomID); herr == nil {
			h.broadcastRoomSeatSnapshot(roomID, humansOnly)
		} else {
			slog.Error("quick fill: loading players for seat snapshot", "roomID", roomID, "error", herr)
		}
		h.voidInvitesIfFull(roomID)
	}

	matchStarted, err := h.autoStartIfFull(roomID)
	if err != nil {
		slog.Error("quick fill: auto-start check failed", "roomID", roomID, "error", err)
	}
	if matchStarted {
		return
	}

	s.mu.Lock()
	entry, ok = s.pending[roomID]
	if !ok || entry.generation != generation {
		s.mu.Unlock()
		return
	}
	entry.generation++
	nextGen := entry.generation
	entry.timer = time.AfterFunc(entry.interval, func() {
		h.quickFillTick(roomID, nextGen)
	})
	s.mu.Unlock()
}
