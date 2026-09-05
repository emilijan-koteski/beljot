package room

import "time"

// This file is compiled only under `go test` (the _test.go suffix) and exposes
// the unexported Quick Play auto-fill scheduler to the external room_test
// package so its behaviour can be driven deterministically.

// SetQuickFillIntervals overrides the scheduler cadence. Tests set a large
// interval and drive ticks manually via TriggerQuickFillTick so real timers
// never fire mid-test.
func (h *RoomHandler) SetQuickFillIntervals(fast, patient time.Duration) {
	if h.quickFill == nil {
		return
	}
	h.quickFill.mu.Lock()
	defer h.quickFill.mu.Unlock()
	h.quickFill.fastInterval = fast
	h.quickFill.patientInterval = patient
}

// StartQuickFill / ResetQuickFill / CancelQuickFill expose the arm / reset /
// cancel entry points.
func (h *RoomHandler) StartQuickFill(roomID, initiatorID uint) { h.startQuickFill(roomID, initiatorID) }
func (h *RoomHandler) ResetQuickFill(roomID uint)              { h.resetQuickFill(roomID) }
func (h *RoomHandler) CancelQuickFill(roomID uint)             { h.cancelQuickFill(roomID) }

// IdleLobbyCount exposes the idle-lobby bucketing for direct assertion.
func (h *RoomHandler) IdleLobbyCount(exclude uint) int { return h.idleLobbyCount(exclude) }

// QuickFillState reports a room's timer state: its current generation, the
// cadence chosen at arm time, and whether a timer is armed at all.
func (h *RoomHandler) QuickFillState(roomID uint) (generation uint64, interval time.Duration, armed bool) {
	if h.quickFill == nil {
		return 0, 0, false
	}
	h.quickFill.mu.Lock()
	defer h.quickFill.mu.Unlock()
	entry, ok := h.quickFill.pending[roomID]
	if !ok {
		return 0, 0, false
	}
	return entry.generation, entry.interval, true
}

// InvokeQuickFillTick fires the tick body directly with an explicit generation,
// without touching the pending timer — used to exercise the staleness guard
// (a superseded generation must be inert).
func (h *RoomHandler) InvokeQuickFillTick(roomID uint, generation uint64) {
	h.quickFillTick(roomID, generation)
}

// TriggerQuickFillTick fires exactly one scheduler tick synchronously, using the
// entry's current generation so it passes the staleness guard. It stops the
// pending real timer first so the AfterFunc doesn't also fire. A no-op when no
// timer is armed. The tick may re-arm a new (large-interval) timer; call
// CancelQuickFill at test end to clean it up.
func (h *RoomHandler) TriggerQuickFillTick(roomID uint) {
	if h.quickFill == nil {
		return
	}
	h.quickFill.mu.Lock()
	entry, ok := h.quickFill.pending[roomID]
	if !ok {
		h.quickFill.mu.Unlock()
		return
	}
	if entry.timer != nil {
		entry.timer.Stop()
	}
	gen := entry.generation
	h.quickFill.mu.Unlock()
	h.quickFillTick(roomID, gen)
}
