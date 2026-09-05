package room

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/ws"
)

// defaultQuickFillFastInterval / defaultQuickFillPatientInterval are the two
// cadences the Quick Play auto-fill scheduler picks between at arm time. Fast is
// used when NOBODY else is idle in the lobby — waiting for a human is pointless,
// so a bot lands every 3s until the room fills. Patient is used when at least
// one idle player is online — a bot is added only after 20s pass with no new
// human, and any human join resets that inactivity window (see startQuickFill /
// resetQuickFill). Both are package-level defaults; the intervals on a live
// scheduler are field-level and overridable in tests.
const (
	defaultQuickFillFastInterval    = 3 * time.Second
	defaultQuickFillPatientInterval = 20 * time.Second
)

// quickFillTimer is one room's live auto-fill timer. generation is bumped every
// time the timer is (re)armed or cancelled so a callback that fires after its
// arming was superseded — a stale duplicate, a reset, a cancel — is inert
// (mirrors the bot/turn timer generation guard in internal/match). interval is
// the cadence chosen once at arm time and reused for every subsequent tick.
type quickFillTimer struct {
	timer      *time.Timer
	generation uint64
	interval   time.Duration
}

// quickFillScheduler is the roomID-keyed registry of auto-fill timers, modelled
// on lobby_disconnect.go's mutex-guarded pending map. It owns only the timing
// machinery; the tick body lives on *RoomHandler so it can reuse the repo,
// broadcasts and autoStartIfFull.
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

// connectedUsersProvider / inMatchUsersProvider are the small optional
// capabilities the idle-lobby count needs. *ws.Hub (held as Broadcaster) and
// *match.Manager (held as MatchStarter) already implement them for lobby.GetStats;
// type-asserting keeps the idle read wire-free (no new constructor params). Test
// stubs implement neither, so idleLobbyCount degrades to 0 → the fast path.
type connectedUsersProvider interface {
	ConnectedUserIDs() []uint
}

type inMatchUsersProvider interface {
	InMatchUserIDs() []uint
}

// idleLobbyCount returns how many connected users are idle in the lobby —
// neither in a waiting room nor in a live match — excluding the given user (the
// scheduler's initiator, who is by construction already seated in the room they
// just created). It mirrors lobby.GetStats' inLobby bucketing: connected −
// in-match − in-waiting-room.
//
// When the hub / match-manager capabilities are absent (test stubs) or the
// waiting-room read fails, it returns 0 — a safe, sleep-free default that
// selects the fast cadence rather than making the caller wait on humans who may
// not be there.
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

// startQuickFill arms (or re-arms) the auto-fill scheduler for a room. The
// cadence is decided once, here, from the idle-lobby count: ≥1 idle player ⇒ the
// patient 20s inactivity cadence, 0 idle ⇒ the fast 3s cadence. initiatorID is
// excluded from that count since they are already seated in the room.
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

// resetQuickFill restarts a room's inactivity countdown from zero, keeping the
// cadence chosen at arm time (the patient/fast decision is made ONCE). Called
// when a human joins a still-waiting Quick Play room so a fresh human never has
// a bot dropped on them within the patient window. A no-op when no timer is
// armed (the room already started, was cancelled, or was never a Quick Play arm).
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

// cancelQuickFill stops and forgets a room's auto-fill timer. The generation
// bump prevents a not-yet-fired (or about-to-re-arm) callback from acting via
// the generation guard. It does NOT abort a tick already past that guard — such
// a tick still runs its whole transaction; it is stopped from seating a stray
// bot only by re-reading the room status under the row lock and self-terminating
// once the room is no longer `waiting`. Idempotent.
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

// quickFillTick is one scheduler firing: under the room row-lock it seats a
// single bot into the lowest empty seat of a still-fillable Quick Play room,
// then runs the shared auto-start path. It is generation-guarded so a
// stale/cancelled/reset timer seats nothing, and it self-terminates (seating no
// bot) when the room is missing, not waiting, not Quick Play, or already full.
func (h *RoomHandler) quickFillTick(roomID uint, generation uint64) {
	s := h.quickFill
	if s == nil {
		return
	}

	// Guard: the entry must still exist with the generation this callback was
	// armed under. A reset/cancel/re-start since then makes this firing stale.
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
		// Row-lock so the seat/coverage picture serializes against concurrent
		// human joins and the auto-start transition (mirrors the AddBot handler).
		r, err := tx.FindByIDForUpdate(roomID)
		if err != nil {
			return fmt.Errorf("finding room: %w", err)
		}
		// Self-terminate: room gone, not Quick Play, or no longer waiting.
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
		// Already full (humans + bots cover all four seats): nothing to do,
		// self-terminate. Never remove a seated bot to prefer a human.
		if seatedHumans+len(bots) >= 4 {
			terminate = true
			return nil
		}

		// Lowest seat free of BOTH a human and a bot.
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
			// Coverage says a seat is free but every index is taken (drift):
			// self-terminate rather than loop.
			terminate = true
			return nil
		}

		if err := tx.AddBot(roomID, seat); err != nil {
			return fmt.Errorf("adding bot: %w", err)
		}
		seatedBot = seat
		return nil
	}); err != nil {
		// A hard repository error: log and self-terminate rather than re-arm into
		// a tight failure loop.
		slog.Error("quick fill: tick transaction failed", "roomID", roomID, "error", err)
		h.cancelQuickFill(roomID)
		return
	}

	if terminate {
		h.cancelQuickFill(roomID)
		return
	}

	if seatedBot >= 0 {
		// Post-commit broadcasts mirror the AddBot handler: bot_added to room
		// participants, then a fresh lobby seat snapshot, as separate ordered
		// messages. Then void invites if this bot closed the room.
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

	// Attempt the shared auto-start (counts humans + bots). On a successful start
	// it cancels this scheduler itself; otherwise re-arm the next tick.
	matchStarted, err := h.autoStartIfFull(roomID)
	if err != nil {
		slog.Error("quick fill: auto-start check failed", "roomID", roomID, "error", err)
	}
	if matchStarted {
		return
	}

	// Re-arm the next tick with the same cadence — unless a reset/cancel/re-start
	// changed the generation while this tick ran (in which case a fresh timer is
	// already scheduled, or the room is done).
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
