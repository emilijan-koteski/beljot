package room

import (
	"sort"
	"sync"
)

// PresenceRegistry tracks which users are actually "present" in a reopened
// (waiting) room after a match — i.e. who has clicked "Return to room" or
// freshly joined, as opposed to lingering on the match result dialog.
//
// A room_players row survives a match (so returners reclaim their original
// seats), so DB membership cannot distinguish "returned" from "still away".
// Presence is therefore tracked separately, in process memory.
//
// The registry is best-effort and NOT durable across restarts: after a restart
// it is empty and rebuilds as players return/join. It is application-level
// (return/join), not tied to WS connect/disconnect.
type PresenceRegistry struct {
	mu      sync.Mutex
	present map[uint]map[uint]struct{} // roomID → set of userID
}

// NewPresenceRegistry creates an empty presence registry.
func NewPresenceRegistry() *PresenceRegistry {
	return &PresenceRegistry{
		present: make(map[uint]map[uint]struct{}),
	}
}

// Add marks userID as present in roomID.
func (r *PresenceRegistry) Add(roomID, userID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.present[roomID]
	if !ok {
		set = make(map[uint]struct{})
		r.present[roomID] = set
	}
	set[userID] = struct{}{}
}

// Remove drops userID from roomID's presence set (no-op if absent).
func (r *PresenceRegistry) Remove(roomID, userID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.present[roomID]
	if !ok {
		return
	}
	delete(set, userID)
	if len(set) == 0 {
		delete(r.present, roomID)
	}
}

// Clear drops all presence for roomID (e.g. match start or room close).
func (r *PresenceRegistry) Clear(roomID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.present, roomID)
}

// Present returns the userIDs marked present in roomID, sorted ascending for
// deterministic payloads. Returns an empty (non-nil) slice when none.
func (r *PresenceRegistry) Present(roomID uint) []uint {
	r.mu.Lock()
	defer r.mu.Unlock()
	set := r.present[roomID]
	ids := make([]uint, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
