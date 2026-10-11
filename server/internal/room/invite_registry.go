package room

import (
	"sync"
	"time"
)

const inviteTTL = 60 * time.Second

// inviteGrant is one outstanding invite for one (room, invitee) pair. Only isHost
// authorizes a password bypass; a non-host invite never satisfies ConsumeHostGrant.
type inviteGrant struct {
	id        uint64
	inviterID uint
	isHost    bool
	expiresAt time.Time
	timer     *time.Timer
}

// InviteRegistry holds the server-issued one-time grants that let a host-invited
// friend past a private room's password. It is in-process and deliberately not
// durable, since invites are ephemeral.
type InviteRegistry struct {
	mu     sync.Mutex
	ttl    time.Duration
	nextID uint64
	grants map[uint]map[uint]*inviteGrant
}

// NewInviteRegistry creates an empty registry using the default invite TTL.
func NewInviteRegistry() *InviteRegistry {
	return NewInviteRegistryWithTTL(inviteTTL)
}

// NewInviteRegistryWithTTL creates a registry with a custom TTL, mainly for tests.
// A non-positive ttl falls back to the default.
func NewInviteRegistryWithTTL(ttl time.Duration) *InviteRegistry {
	if ttl <= 0 {
		ttl = inviteTTL
	}
	return &InviteRegistry{
		ttl:    ttl,
		grants: make(map[uint]map[uint]*inviteGrant),
	}
}

// Issue records an invite for the pair, replacing any outstanding one, and returns
// its id and expiry. Only the room owner's invite should pass isHost, and a
// replacement never downgrades a live host grant.
func (r *InviteRegistry) Issue(roomID, inviteeID, inviterID uint, isHost bool) (uint64, time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	byInvitee, ok := r.grants[roomID]
	if !ok {
		byInvitee = make(map[uint]*inviteGrant)
		r.grants[roomID] = byInvitee
	}
	if existing, ok := byInvitee[inviteeID]; ok {
		if existing.timer != nil {
			existing.timer.Stop()
		}
		if existing.isHost && time.Now().Before(existing.expiresAt) {
			isHost = true
			inviterID = existing.inviterID
		}
	}

	r.nextID++
	g := &inviteGrant{
		id:        r.nextID,
		inviterID: inviterID,
		isHost:    isHost,
		expiresAt: time.Now().Add(r.ttl),
	}
	grantID := g.id
	g.timer = time.AfterFunc(r.ttl, func() { r.expire(roomID, inviteeID, grantID) })
	byInvitee[inviteeID] = g

	return g.id, g.expiresAt
}

// HasHostGrant reports whether a valid host grant exists for the pair without
// consuming it. JoinRoom checks this at the password gate so a rejection by a later
// gate does not burn the grant.
func (r *InviteRegistry) HasHostGrant(roomID, inviteeID uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	g, ok := r.grants[roomID][inviteeID]
	if !ok {
		return false
	}
	if time.Now().After(g.expiresAt) {
		r.deleteLocked(roomID, inviteeID)
		return false
	}
	return g.isHost
}

// ConsumeHostGrant reports whether a valid host grant exists for the pair and
// deletes it if so. A non-host grant is left in place, since it is not a bypass token.
func (r *InviteRegistry) ConsumeHostGrant(roomID, inviteeID uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	g, ok := r.grants[roomID][inviteeID]
	if !ok {
		return false
	}
	if time.Now().After(g.expiresAt) {
		r.deleteLocked(roomID, inviteeID)
		return false
	}
	if !g.isHost {
		return false
	}
	r.deleteLocked(roomID, inviteeID)
	return true
}

// Pending reports whether a non-expired invite (of either kind) is outstanding
// for the pair. Read-only — it never consumes.
func (r *InviteRegistry) Pending(roomID, inviteeID uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	g, ok := r.grants[roomID][inviteeID]
	if !ok {
		return false
	}
	if time.Now().After(g.expiresAt) {
		r.deleteLocked(roomID, inviteeID)
		return false
	}
	return true
}

// Void drops one outstanding invite (no-op when absent).
func (r *InviteRegistry) Void(roomID, inviteeID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleteLocked(roomID, inviteeID)
}

// VoidRoom drops every outstanding invite for a room. Called when the room stops
// accepting invitees — it filled, the match started, or it closed (AC2).
func (r *InviteRegistry) VoidRoom(roomID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for inviteeID, g := range r.grants[roomID] {
		if g.timer != nil {
			g.timer.Stop()
		}
		delete(r.grants[roomID], inviteeID)
	}
	delete(r.grants, roomID)
}

// VoidUser drops every outstanding invite addressed to one invitee, across all
// rooms. Called when the friend leaves the lobby (disconnect) — their popup is
// gone with them, so the grant must not outlive it (AC2).
func (r *InviteRegistry) VoidUser(inviteeID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for roomID := range r.grants {
		r.deleteLocked(roomID, inviteeID)
	}
}

// VoidInviter drops every invite a player issued into one room once they can no
// longer vouch for invitees. This stops a departed ex-owner's grant from bypassing
// the current owner's password.
func (r *InviteRegistry) VoidInviter(roomID, inviterID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for inviteeID, g := range r.grants[roomID] {
		if g.inviterID == inviterID {
			r.deleteLocked(roomID, inviteeID)
		}
	}
}

// expire is the TTL timer callback. It deletes the grant ONLY when the stored
// grant is still the one the timer was created for — a re-issue between the
// scheduling and the firing must survive.
func (r *InviteRegistry) expire(roomID, inviteeID uint, grantID uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if g, ok := r.grants[roomID][inviteeID]; ok && g.id == grantID {
		r.deleteLocked(roomID, inviteeID)
	}
}

// deleteLocked removes one grant and prunes the room bucket when it empties.
// Callers must hold r.mu.
func (r *InviteRegistry) deleteLocked(roomID, inviteeID uint) {
	byInvitee, ok := r.grants[roomID]
	if !ok {
		return
	}
	if g, ok := byInvitee[inviteeID]; ok && g.timer != nil {
		g.timer.Stop()
	}
	delete(byInvitee, inviteeID)
	if len(byInvitee) == 0 {
		delete(r.grants, roomID)
	}
}
