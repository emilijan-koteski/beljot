package room

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
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

// PlayerSummary is one player resolved for the invite panel's Lobby tab, with
// their opt-in to invites from players who are not their friends.
type PlayerSummary struct {
	UserID                 uint
	Username               string
	AvatarURL              *string
	AllowsNonFriendInvites bool
}

// PlayerDirectory resolves user IDs to invite-panel rows, omitting unknown or deleted users.
// It is satisfied by the same main.go adapter as FriendDirectory.
type PlayerDirectory interface {
	FindPlayers(userIDs []uint) ([]PlayerSummary, error)
}

// ConnectionTracker reports which users hold a live WebSocket connection.
// Implemented by *ws.Hub.
type ConnectionTracker interface {
	IsConnected(userID uint) bool
	ConnectedUserIDs() []uint
}

// SessionTracker reports which users are mapped to an active game session.
// Implemented by *match.Manager.
type SessionTracker interface {
	IsUserInMatch(userID uint) bool
	InMatchUserIDs() []uint
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

const lobbyInviteListLimit = 50

// InviteHandler serves the room-invite endpoints, kept apart from RoomHandler to
// avoid widening its constructor. It shares the InviteRegistry with RoomHandler:
// this handler issues grants and JoinRoom consumes them.
type InviteHandler struct {
	repo        RoomRepository
	invites     *InviteRegistry
	friends     FriendDirectory
	players     PlayerDirectory
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
	players PlayerDirectory,
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
		players:     players,
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

// InvitableFriendDTO is one row of GET /rooms/:id/invitable-friends or /invitable-players.
// Available is computed server-side from the presence trio; Reason is empty when Available.
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

	if err := h.markUnavailableWhenFull(roomID, items); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// ListInvitablePlayers handles GET /rooms/:id/invitable-players?search=, returning idle
// lobby players who are not the caller's friends and accept non-friend invites. Opted-out
// players are simply absent; the result is sorted by username and capped.
func (h *InviteHandler) ListInvitablePlayers(c echo.Context) error {
	callerID, roomID, _, err := h.requireRoomMember(c)
	if err != nil {
		return err
	}

	candidates, err := h.idleNonFriendIDs(callerID)
	if err != nil {
		return err
	}

	players, err := h.players.FindPlayers(candidates)
	if err != nil {
		return fmt.Errorf("resolving lobby players for invite panel: %w", err)
	}

	search := strings.ToLower(strings.TrimSpace(c.QueryParam("search")))
	items := make([]InvitableFriendDTO, 0, len(players))
	for _, p := range players {
		if !p.AllowsNonFriendInvites {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(p.Username), search) {
			continue
		}
		items = append(items, InvitableFriendDTO{
			UserID:    p.UserID,
			Username:  p.Username,
			AvatarURL: p.AvatarURL,
			Available: true,
		})
	}

	slices.SortFunc(items, func(a, b InvitableFriendDTO) int {
		if byName := strings.Compare(strings.ToLower(a.Username), strings.ToLower(b.Username)); byName != 0 {
			return byName
		}
		return cmp.Compare(a.UserID, b.UserID)
	})
	if len(items) > lobbyInviteListLimit {
		items = items[:lobbyInviteListLimit]
	}

	if err := h.markUnavailableWhenFull(roomID, items); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"data": items})
}

// InviteToRoom handles POST /rooms/:id/invite, issuing a one-time grant and pushing the popup.
// A non-friend is invitable only while they accept non-friend invites; one invite per room and player may be live.
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

	invitable, err := h.acceptsInviteFrom(callerID, req.FriendUserID)
	if err != nil {
		return err
	}
	if !invitable {
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

// acceptsInviteFrom reports whether inviteeID takes invites from inviterID: always for
// a friend, and for anyone else only while the invitee's opt-in is on.
func (h *InviteHandler) acceptsInviteFrom(inviterID, inviteeID uint) (bool, error) {
	areFriends, err := h.friends.AreFriends(inviterID, inviteeID)
	if err != nil {
		return false, fmt.Errorf("checking friendship for invite: %w", err)
	}
	if areFriends {
		return true, nil
	}
	found, err := h.players.FindPlayers([]uint{inviteeID})
	if err != nil {
		return false, fmt.Errorf("reading invitee invite preference: %w", err)
	}
	return len(found) == 1 && found[0].AllowsNonFriendInvites, nil
}

// idleNonFriendIDs returns the connected users who are in neither a match nor a waiting or playing room,
// excluding callerID and callerID's friends. Without a connection tracker nobody is known to be online.
func (h *InviteHandler) idleNonFriendIDs(callerID uint) ([]uint, error) {
	if h.connections == nil {
		return []uint{}, nil
	}
	excluded := map[uint]struct{}{callerID: {}}
	if h.sessions != nil {
		for _, uid := range h.sessions.InMatchUserIDs() {
			excluded[uid] = struct{}{}
		}
	}
	for _, status := range []string{"waiting", "playing"} {
		ids, err := h.repo.FindUserIDsByRoomStatus(status)
		if err != nil {
			return nil, fmt.Errorf("listing %s-room players for invite panel: %w", status, err)
		}
		for _, uid := range ids {
			excluded[uid] = struct{}{}
		}
	}
	friends, err := h.friends.ListFriends(callerID)
	if err != nil {
		return nil, fmt.Errorf("listing friends for invite panel: %w", err)
	}
	for _, f := range friends {
		excluded[f.UserID] = struct{}{}
	}

	idle := make([]uint, 0)
	for _, uid := range h.connections.ConnectedUserIDs() {
		if _, skip := excluded[uid]; skip || uid == 0 {
			continue
		}
		idle = append(idle, uid)
	}
	return idle, nil
}

// markUnavailableWhenFull disables every row with reason room_full once the room
// cannot seat another human, keeping any more specific reason a row already has.
func (h *InviteHandler) markUnavailableWhenFull(roomID uint, items []InvitableFriendDTO) error {
	full, err := h.roomIsFull(roomID)
	if err != nil {
		return err
	}
	if !full {
		return nil
	}
	for i := range items {
		items[i].Available = false
		if items[i].Reason == "" {
			items[i].Reason = inviteReasonRoomFull
		}
	}
	return nil
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
