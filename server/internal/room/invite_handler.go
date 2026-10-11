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

// FriendSummary is one accepted friend of the viewer, resolved to a username and
// avatar URL. It is owned by room so that `room` never imports `friend`.
type FriendSummary struct {
	UserID    uint
	Username  string
	AvatarURL *string
}

// FriendDirectory is the subset of the friend domain the invite handler needs.
// It is satisfied by an adapter in main.go, so `friend` never imports `room`.
type FriendDirectory interface {
	AreFriends(a, b uint) (bool, error)
	ListFriends(userID uint) ([]FriendSummary, error)
}

// ConnectionTracker reports whether a user holds a live WebSocket connection.
// Implemented by *ws.Hub.
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

const (
	inviteReasonOffline     = "offline"
	inviteReasonInMatch     = "in_match"
	inviteReasonInRoom      = "in_room"
	inviteReasonAlreadyHere = "in_this_room"
	inviteReasonRoomFull    = "room_full"
)

// InviteHandler serves the friend-invite endpoints, kept apart from RoomHandler to
// avoid widening its constructor. It shares the InviteRegistry with RoomHandler:
// this handler issues grants and JoinRoom consumes them.
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

// InviteToRoomRequest is the body of POST /rooms/:id/invite. It deliberately has no
// bypass flag; the server derives that from the room owner.
type InviteToRoomRequest struct {
	FriendUserID uint `json:"friendUserId"`
}

// InvitableFriendDTO is one row of GET /rooms/:id/invitable-friends. Available
// is computed server-side from the presence trio; Reason is empty when Available.
type InvitableFriendDTO struct {
	UserID    uint    `json:"userId"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
	Available bool    `json:"available"`
	Reason    string  `json:"reason"`
}

// ListInvitableFriends handles GET /rooms/:id/invitable-friends, returning every friend
// annotated with whether they can be invited now. The result is advisory rendering
// data; availability is re-checked at invite time.
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
		reason, rerr := h.availabilityReason(f.UserID, roomID)
		if rerr != nil {
			return rerr
		}
		items = append(items, InvitableFriendDTO{
			UserID:    f.UserID,
			Username:  f.Username,
			AvatarURL: f.AvatarURL,
			Available: reason == "",
			Reason:    reason,
		})
	}

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

// InviteToRoom handles POST /rooms/:id/invite, issuing a one-time grant and pushing
// the popup. Only one invite per room and friend may be live, and only the owner's
// invite carries a password bypass.
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

	reason, err := h.availabilityReason(req.FriendUserID, roomID)
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

// DeclineInvite handles POST /rooms/:id/invite/decline, voiding the caller's own
// invite to that room. It skips requireRoomMember because the invitee is not in the
// room, and it always returns 200.
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

// requireRoomMember resolves the caller and the :id room, requiring a waiting,
// non-quick-play room the caller belongs to. Quick-play rooms are refused because
// JoinRoom has no bracket check, so an invite would bypass the bracket.
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
	if room == nil || room.Status != "waiting" {
		return 0, 0, nil, apperr.ErrRoomNotFound
	}
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

// availabilityReason returns why a friend cannot be invited, or "" when available.
// The in-room probe uses JoinRoom's FindPlayerRoom predicate, and a nil tracker
// counts as no signal.
func (h *InviteHandler) availabilityReason(friendID, roomID uint) (string, error) {
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
		if existing.RoomID == roomID {
			return inviteReasonAlreadyHere, nil
		}
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

// pushInvite delivers the best-effort system:room_invite push. Offline recipients
// and marshal failures are dropped, so a failed notification never fails the request.
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
