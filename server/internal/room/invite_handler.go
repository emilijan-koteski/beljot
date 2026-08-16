package room

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/ws"
)

// FriendSummary is one accepted friend of the viewer, already resolved to a
// username. It is a room-owned DTO on purpose: the adapter that satisfies
// FriendDirectory does the friend-rows → usernames join, so `room` never imports
// `friend` and the two packages stay decoupled (same discipline as WalletService
// / HonorService).
type FriendSummary struct {
	UserID   uint
	Username string
}

// FriendDirectory is the subset of the friend domain the invite handler needs
// (Story 11.5, D5): who are my friends, and is this specific player one of them.
// Satisfied by an adapter in main.go over friend.Repository + user.UserRepository.
//
// Import direction: room → (adapter) → friend. `friend` must never import `room`.
type FriendDirectory interface {
	AreFriends(a, b uint) (bool, error)
	ListFriends(userID uint) ([]FriendSummary, error)
}

// ConnectionTracker reports whether a user holds a live WebSocket connection.
// Implemented by *ws.Hub. Mirrors lobby.ConnectionTracker — declared here so the
// invite handler takes a one-method dependency instead of the whole hub.
type ConnectionTracker interface {
	IsConnected(userID uint) bool
}

// SessionTracker reports whether a user is mapped to an active game session.
// Implemented by *match.Manager.
type SessionTracker interface {
	IsUserInMatch(userID uint) bool
}

// InviteNotifier is the per-user push the invite handler needs. *ws.Hub
// satisfies it; handler tests inject a spy.
type InviteNotifier interface {
	SendToUser(userID uint, msg []byte)
}

// Availability reasons returned by GET /rooms/:id/invitable-friends. An
// available friend carries the empty string; the client maps the rest to a
// localized "why not" line under a disabled Invite button.
const (
	inviteReasonOffline  = "offline"
	inviteReasonInMatch  = "in_match"
	inviteReasonInRoom   = "in_room"
	inviteReasonRoomFull = "room_full"
)

// InviteHandler serves the two friend-invite endpoints (Story 11.5). It is a
// separate handler from RoomHandler on purpose: the invite feature needs four
// dependencies (friends, connections, sessions, notifier) that nothing else in
// RoomHandler uses, and threading them through its already six-argument
// constructor would touch every room handler test for no benefit.
//
// The InviteRegistry, however, IS shared with RoomHandler — that is the whole
// point: this handler issues grants, JoinRoom consumes them.
type InviteHandler struct {
	repo        RoomRepository
	invites     *InviteRegistry
	friends     FriendDirectory
	connections ConnectionTracker
	sessions    SessionTracker
	notifier    InviteNotifier
}

// NewInviteHandler wires the invite handler. invites must be the SAME registry
// injected into RoomHandler, or issued grants will never be consumable.
func NewInviteHandler(
	repo RoomRepository,
	invites *InviteRegistry,
	friends FriendDirectory,
	connections ConnectionTracker,
	sessions SessionTracker,
	notifier InviteNotifier,
) *InviteHandler {
	if invites == nil {
		invites = NewInviteRegistry()
	}
	return &InviteHandler{
		repo:        repo,
		invites:     invites,
		friends:     friends,
		connections: connections,
		sessions:    sessions,
		notifier:    notifier,
	}
}

// InviteToRoomRequest is the body of POST /rooms/:id/invite.
//
// It carries the target and NOTHING else. In particular there is no bypass flag
// of any kind (Story 11.5 D3) — whether the invite grants a password bypass is
// decided server-side from room.OwnerID == callerID, never from the wire.
type InviteToRoomRequest struct {
	FriendUserID uint `json:"friendUserId"`
}

// InvitableFriendDTO is one row of GET /rooms/:id/invitable-friends. Available
// is computed server-side from the presence trio; Reason is empty when Available.
type InvitableFriendDTO struct {
	UserID    uint   `json:"userId"`
	Username  string `json:"username"`
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

// ListInvitableFriends handles GET /rooms/:id/invitable-friends (AC1). It returns
// the caller's accepted friends, each annotated with whether they can be invited
// right now. Unavailable friends are RETURNED, not filtered out — the panel shows
// them disabled with a reason, which is far less confusing than a friend silently
// missing from the list.
//
// Availability is server-computed and re-checked at invite time; this endpoint is
// advisory rendering data, never an authorization.
func (h *InviteHandler) ListInvitableFriends(c echo.Context) error {
	callerID, roomID, _, err := h.requireRoomMember(c)
	if err != nil {
		return err
	}

	friends, err := h.friends.ListFriends(callerID)
	if err != nil {
		return fmt.Errorf("listing friends for invite panel: %w", err)
	}

	items := make([]InvitableFriendDTO, 0, len(friends))
	for _, f := range friends {
		reason, rerr := h.availabilityReason(f.UserID)
		if rerr != nil {
			return rerr
		}
		items = append(items, InvitableFriendDTO{
			UserID:    f.UserID,
			Username:  f.Username,
			Available: reason == "",
			Reason:    reason,
		})
	}

	// The room itself may already be full; the panel disables every button in
	// that case rather than letting an invite be sent into a dead end.
	full, err := h.roomIsFull(roomID)
	if err != nil {
		return err
	}
	if full {
		for i := range items {
			items[i].Available = false
			if items[i].Reason == "" {
				items[i].Reason = inviteReasonRoomFull
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// InviteToRoom handles POST /rooms/:id/invite (AC2). It issues a one-time invite
// grant and pushes the popup. The gate order is: caller is in a waiting room →
// target is the caller's friend → target is available. isHostInvite is derived
// from room.OwnerID, and ONLY a host invite produces a consumable password
// bypass (AC3/AC4/D2).
func (h *InviteHandler) InviteToRoom(c echo.Context) error {
	callerID, roomID, room, err := h.requireRoomMember(c)
	if err != nil {
		return err
	}

	var req InviteToRoomRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return apperr.ErrBadRequest
	}
	if req.FriendUserID == 0 || req.FriendUserID == callerID {
		return apperr.ErrBadRequest
	}

	areFriends, err := h.friends.AreFriends(callerID, req.FriendUserID)
	if err != nil {
		return fmt.Errorf("checking friendship for invite: %w", err)
	}
	if !areFriends {
		return apperr.ErrNotFriends
	}

	// Availability is ALWAYS recomputed here — a client claim that "this friend is
	// available" is never trusted.
	reason, err := h.availabilityReason(req.FriendUserID)
	if err != nil {
		return err
	}
	if reason != "" {
		return apperr.ErrFriendNotAvailable
	}

	full, err := h.roomIsFull(roomID)
	if err != nil {
		return err
	}
	if full {
		return apperr.ErrRoomFull
	}

	// One live invite per (room, friend). This is the throttle: re-issuing on
	// every POST would let any seated member replace the friend's popup and reset
	// its clock as fast as they can send requests, and would let a member's
	// non-host invite silently overwrite the owner's stronger host grant. The
	// friend answers or the TTL lapses before anyone can invite them again.
	if h.invites.Pending(roomID, req.FriendUserID) {
		return apperr.ErrInviteAlreadyPending
	}

	isHost := room.OwnerID == callerID
	inviteID, expiresAt := h.invites.Issue(roomID, req.FriendUserID, callerID, isHost)

	h.pushInvite(req.FriendUserID, ws.RoomInvitePayload{
		InviteID:        inviteID,
		RoomID:          roomID,
		RoomName:        room.Name,
		InviterUserID:   callerID,
		InviterUsername: h.usernameInRoom(roomID, callerID),
		CoinBuyIn:       room.CoinBuyIn,
		IsPrivate:       room.PasswordHash != nil,
		IsHostInvite:    isHost,
		MinHonor:        room.MinHonor,
		ExpiresAt:       expiresAt.UTC().Format(time.RFC3339),
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"inviteId":     inviteID,
			"roomId":       roomID,
			"friendUserId": req.FriendUserID,
			"isHostInvite": isHost,
			"expiresAt":    expiresAt.UTC().Format(time.RFC3339),
		},
	})
}

// DeclineInvite handles POST /rooms/:id/invite/decline. The authenticated caller
// declines an invite addressed to THEM, which voids the grant immediately rather
// than leaving it consumable for the rest of its TTL.
//
// Note the asymmetry with the other two endpoints: this one is called by the
// INVITEE, who by definition is not in the room, so it deliberately does not use
// requireRoomMember. The grant is keyed by (roomID, authenticated userID), so a
// caller can only ever void their own invite — there is nothing to authorize
// beyond being logged in, and voiding a grant that does not exist is a harmless
// no-op. Always 200: an already-expired or already-voided invite is not an error
// from the declining player's point of view.
func (h *InviteHandler) DeclineInvite(c echo.Context) error {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	parsed, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.ErrRoomNotFound
	}

	h.invites.Void(uint(parsed), userID)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{"declined": true},
	})
}

// requireRoomMember resolves the authenticated caller, the :id room, and asserts
// the room is still `waiting` and the caller is one of its members (owner or
// seated player alike). Both invite endpoints share it so their guards cannot
// drift apart.
//
// Membership is read from FindPlayersByRoomID rather than FindPlayerRoom because
// it answers the stronger question directly — "is the caller in THIS room" — and
// the same rows carry the inviter's username for the push.
func (h *InviteHandler) requireRoomMember(c echo.Context) (uint, uint, *Room, error) {
	callerID, err := auth.GetUserID(c)
	if err != nil {
		return 0, 0, nil, apperr.ErrUnauthorized
	}

	parsed, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, 0, nil, apperr.ErrRoomNotFound
	}
	roomID := uint(parsed)

	room, err := h.repo.FindByID(roomID)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("finding room for invite: %w", err)
	}
	// A closed or in-progress room is indistinguishable from a missing one here,
	// matching JoinRoom's treatment of a non-waiting room.
	if room == nil || room.Status != "waiting" {
		return 0, 0, nil, apperr.ErrRoomNotFound
	}
	// Quick-play rooms are matchmaking brackets, not social tables: seats are
	// allocated by the bracket, and JoinRoom has no bracket check (that lives in
	// QuickJoin), so an invite would seat a friend in a bracket they never
	// qualified for. AddBot, LeaveSeat and StartMatch all refuse quick-play rooms
	// the same way.
	if room.IsQuickPlay {
		return 0, 0, nil, apperr.ErrRoomNotFound
	}

	players, err := h.repo.FindPlayersByRoomID(roomID)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("loading room members for invite: %w", err)
	}
	for _, p := range players {
		if p.UserID == callerID {
			return callerID, roomID, room, nil
		}
	}
	return 0, 0, nil, apperr.ErrNotInRoom
}

// availabilityReason applies the presence trio to one friend and returns the
// reason they cannot be invited, or "" when they are available.
//
// "Available" means online AND not in a match AND not in a room — the same
// bucketing lobby.GetStats uses, evaluated per user. The in-room probe is
// FindPlayerRoom (waiting OR playing), which is exactly the predicate JoinRoom's
// already-in-room gate uses, so the panel can never advertise a friend the join
// would then reject. A missing tracker is treated as "no signal" rather than
// "unavailable", mirroring the nil-service affordances elsewhere in this package.
func (h *InviteHandler) availabilityReason(friendID uint) (string, error) {
	if h.connections != nil && !h.connections.IsConnected(friendID) {
		return inviteReasonOffline, nil
	}
	if h.sessions != nil && h.sessions.IsUserInMatch(friendID) {
		return inviteReasonInMatch, nil
	}
	existing, err := h.repo.FindPlayerRoom(friendID)
	if err != nil {
		return "", fmt.Errorf("checking friend room presence: %w", err)
	}
	if existing != nil {
		return inviteReasonInRoom, nil
	}
	return "", nil
}

// roomIsFull reports whether the room can still seat another human. Bot-covered
// seats count toward capacity, exactly as in JoinRoom.
func (h *InviteHandler) roomIsFull(roomID uint) (bool, error) {
	room, err := h.repo.FindByID(roomID)
	if err != nil {
		return false, fmt.Errorf("re-reading room capacity: %w", err)
	}
	if room == nil {
		return true, nil
	}
	if room.PlayerCount >= 4 {
		return true, nil
	}
	bots, err := h.repo.FindBotsByRoomID(roomID)
	if err != nil {
		return false, fmt.Errorf("counting room bots: %w", err)
	}
	return room.PlayerCount+len(bots) >= 4, nil
}

// usernameInRoom resolves a room member's username for the invite payload.
// Best-effort: an empty string is preferable to failing the invite, and the
// popup still names the room.
func (h *InviteHandler) usernameInRoom(roomID, userID uint) string {
	players, err := h.repo.FindPlayersByRoomID(roomID)
	if err != nil {
		return ""
	}
	for _, p := range players {
		if p.UserID == userID {
			return p.Username
		}
	}
	return ""
}

// pushInvite marshals and delivers the best-effort system:room_invite push.
// Offline recipients are a silent no-op (SendToUser drops unknown users) — there
// is no offline inbox, and marshal failures are logged, never returned: a failed
// notification must not fail the request that issued the grant.
func (h *InviteHandler) pushInvite(recipientID uint, payload ws.RoomInvitePayload) {
	if h.notifier == nil {
		return
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("room: failed to marshal room_invite payload", "error", err)
		return
	}
	msg, err := json.Marshal(ws.WSMessage{Type: ws.SystemRoomInvite, Payload: payloadBytes})
	if err != nil {
		slog.Error("room: failed to marshal room_invite message", "error", err)
		return
	}
	h.notifier.SendToUser(recipientID, msg)
}
