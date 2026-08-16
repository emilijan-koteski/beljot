package room

import (
	"sync"
	"time"
)

// inviteTTL bounds how long a friend room invite (Story 11.5) stays actionable.
// An invite is a real-time popup, not an inbox item: if the friend has not acted
// within the window the room has almost certainly moved on, so the grant is
// dropped rather than left to authorize a stale bypass minutes later.
const inviteTTL = 60 * time.Second

// inviteGrant is one outstanding invite for one (room, invitee) pair.
//
// isHost is the ONLY thing that authorizes a password bypass. A non-host
// member's invite is recorded all the same — the popup and its expiry are
// identical — it simply never satisfies ConsumeHostGrant, so the invitee lands
// on the normal password prompt (AC4).
type inviteGrant struct {
	id        uint64
	inviterID uint
	isHost    bool
	expiresAt time.Time
	timer     *time.Timer
}

// InviteRegistry holds the server-issued one-time invite grants that let a
// HOST-invited friend past a private room's password gate (Story 11.5, AC3/D3).
//
// The bypass is server-authorized by construction: a grant is looked up by the
// authenticated userID + roomID at join time, and the client never sends a
// bypass flag of any kind (JoinRoomRequest gains no field). There is no way for
// a client to self-authorize.
//
// Like PresenceRegistry and the lobby-disconnect timers this is in-process,
// mutex-guarded, and deliberately NOT durable: an invite is ephemeral, so losing
// every outstanding grant on restart is correct behaviour, not data loss. The
// friend simply re-requests an invite.
type InviteRegistry struct {
	mu     sync.Mutex
	ttl    time.Duration
	nextID uint64
	grants map[uint]map[uint]*inviteGrant // roomID → inviteeID → grant
}

// NewInviteRegistry creates an empty registry using the default invite TTL.
func NewInviteRegistry() *InviteRegistry {
	return NewInviteRegistryWithTTL(inviteTTL)
}

// NewInviteRegistryWithTTL is the tunable-TTL constructor. Tests use it to make
// expiry observable without sleeping for a minute; production uses the default.
// A non-positive ttl falls back to the default rather than issuing grants that
// are born expired.
func NewInviteRegistryWithTTL(ttl time.Duration) *InviteRegistry {
	if ttl <= 0 {
		ttl = inviteTTL
	}
	return &InviteRegistry{
		ttl:    ttl,
		grants: make(map[uint]map[uint]*inviteGrant),
	}
}

// Issue records an invite from inviterID to inviteeID for roomID and returns the
// invite id (echoed to the client in the system:room_invite push) and its expiry.
//
// isHost must be true only when the inviter is the room OWNER — that is what
// makes the grant consumable as a password bypass. Re-inviting the same friend
// replaces the outstanding grant and restarts the clock; the replaced grant's
// timer is stopped first so it cannot void its own successor.
//
// A replacement NEVER downgrades an existing non-expired host grant. Without
// that rule an ordinary member who invites the same friend right after the owner
// did would silently strip the owner's password bypass, and the invitee would be
// asked for a password the host meant to wave them past. The stronger grant wins
// and only the clock is refreshed. (The handler additionally refuses to issue at
// all while an invite is pending, so this is defence in depth for any future
// caller.)
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
	// Capture the id so a fired timer for a REPLACED grant is a no-op.
	grantID := g.id
	g.timer = time.AfterFunc(r.ttl, func() { r.expire(roomID, inviteeID, grantID) })
	byInvitee[inviteeID] = g

	return g.id, g.expiresAt
}

// HasHostGrant reports whether a valid HOST invite grant exists for the pair
// WITHOUT consuming it. This — not ConsumeHostGrant — is what JoinRoom asks at
// the password gate.
//
// The distinction matters because the password block is only the FIRST of six
// gates: capacity, bot-capacity, already-in-room, coin affordability and honor
// all still run after it. Consuming there would burn the grant on a rejection
// the invitee can recover from (a seat frees up, they top up their coins, they
// leave the other room) and their retry would then hit the bcrypt gate for a
// password they were never given. The grant is instead spent on SUCCESS, by the
// VoidUser(userID) call that already runs after a completed join — see JoinRoom.
//
// An expired grant is deleted on the way out, same as ConsumeHostGrant.
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

// ConsumeHostGrant reports whether a valid HOST invite grant exists for the pair
// and, if so, deletes it — the grant is one-time by construction.
//
// It returns false for a missing grant, an expired one, and a non-host one. A
// rejected non-host grant is deliberately LEFT IN PLACE: it is not a bypass
// token, so consuming it would only make the invite vanish for no reason.
//
// JoinRoom uses the non-consuming HasHostGrant instead (see above); this stays
// as the registry's explicit one-time primitive.
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

// VoidInviter drops every outstanding invite a given player issued into one
// room. Called when that player stops being able to vouch for an invitee — they
// left the room, were kicked, or handed ownership away (AC2/AC3).
//
// Without it a grant issued by a departed EX-owner would keep bypassing the
// CURRENT owner's password for the rest of its TTL, which is exactly the
// authority the host-only bypass is supposed to encode.
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
