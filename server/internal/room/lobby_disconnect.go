package room

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/ws"
)

const lobbyDisconnectTimeout = 10 * time.Second

// LobbyDisconnectHandler frees a waiting-room seat after a short timeout when its
// player disconnects. Reconnecting within the window cancels the timer.
type LobbyDisconnectHandler struct {
	repo     RoomRepository
	hub      Broadcaster
	presence *PresenceRegistry
	invites  *InviteRegistry
	mu       sync.Mutex
	pending  map[uint]*lobbyDisconnect
}

type lobbyDisconnect struct {
	timer  *time.Timer
	roomID uint
}

// NewLobbyDisconnectHandler creates a handler for lobby disconnect events. Nil
// presence or invites get empty registries, and a typed-nil *ws.Hub becomes a nil hub.
func NewLobbyDisconnectHandler(repo RoomRepository, hub Broadcaster, presence *PresenceRegistry, invites *InviteRegistry) *LobbyDisconnectHandler {
	if presence == nil {
		presence = NewPresenceRegistry()
	}
	if invites == nil {
		invites = NewInviteRegistry()
	}
	if h, ok := hub.(*ws.Hub); ok && h == nil {
		hub = nil
	}
	return &LobbyDisconnectHandler{
		repo:     repo,
		hub:      hub,
		presence: presence,
		invites:  invites,
		pending:  make(map[uint]*lobbyDisconnect),
	}
}

// HandleDisconnect starts a 10-second timer when a player in a waiting room
// disconnects. On expiry the player is removed and the remaining players are notified.
func (h *LobbyDisconnectHandler) HandleDisconnect(userID uint) {
	rp, err := h.repo.FindPlayerRoom(userID)
	if err != nil || rp == nil {
		return
	}

	room, err := h.repo.FindByID(rp.RoomID)
	if err != nil || room == nil {
		return
	}

	if room.Status != "waiting" {
		return
	}

	h.mu.Lock()
	if existing, ok := h.pending[userID]; ok {
		existing.timer.Stop()
		delete(h.pending, userID)
	}

	roomID := rp.RoomID
	timer := time.AfterFunc(lobbyDisconnectTimeout, func() {
		h.handleTimeout(userID, roomID)
	})
	h.pending[userID] = &lobbyDisconnect{
		timer:  timer,
		roomID: roomID,
	}
	h.mu.Unlock()

	slog.Info("room: lobby disconnect timer started", "userID", userID, "roomID", roomID)
}

// HandleReconnect cancels any pending lobby disconnect timer for the user.
func (h *LobbyDisconnectHandler) HandleReconnect(userID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if pending, ok := h.pending[userID]; ok {
		pending.timer.Stop()
		delete(h.pending, userID)
		slog.Info("room: lobby disconnect timer cancelled (reconnected)", "userID", userID)
	}
}

// handleTimeout removes a player from the room after the disconnect window expires.
func (h *LobbyDisconnectHandler) handleTimeout(userID uint, roomID uint) {
	h.mu.Lock()
	pending, ok := h.pending[userID]
	if !ok || pending.roomID != roomID {
		h.mu.Unlock()
		return
	}
	delete(h.pending, userID)
	h.mu.Unlock()

	var leavingUsername string
	players, _ := h.repo.FindPlayersByRoomID(roomID)
	for _, p := range players {
		if p.UserID == userID {
			leavingUsername = p.Username
			break
		}
	}

	room, err := h.repo.FindByID(roomID)
	if err != nil || room == nil {
		return
	}
	if room.Status != "waiting" {
		return
	}

	roomClosed := false

	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		if err := tx.RemovePlayer(roomID, userID); err != nil {
			return err
		}
		if err := tx.DecrementPlayerCount(roomID); err != nil {
			return err
		}
		if room.OwnerID == userID {
			remainingPlayers, err := tx.FindPlayersByRoomID(roomID)
			if err != nil {
				return err
			}
			freshRoom, err := tx.FindByID(roomID)
			if err != nil {
				return err
			}
			if len(remainingPlayers) > 0 {
				freshRoom.OwnerID = remainingPlayers[0].UserID
				return tx.Update(freshRoom)
			}
			freshRoom.Status = "completed"
			roomClosed = true
			return tx.Update(freshRoom)
		}
		return nil
	}); err != nil {
		slog.Error("room: failed to remove disconnected lobby player", "userID", userID, "roomID", roomID, "error", err)
		return
	}

	slog.Info("room: lobby player removed after disconnect timeout", "userID", userID, "roomID", roomID)

	h.presence.Remove(roomID, userID)

	h.invites.VoidInviter(roomID, userID)
	if roomClosed {
		h.invites.VoidRoom(roomID)
	}

	remainingPlayers, err := h.repo.FindPlayersByRoomID(roomID)
	if err != nil || len(remainingPlayers) == 0 {
		return
	}

	postRoom, _ := h.repo.FindByID(roomID)
	actualPlayerCount := len(remainingPlayers)
	if postRoom != nil {
		actualPlayerCount = postRoom.PlayerCount
	}

	userIDs := make([]uint, 0, len(remainingPlayers))
	for _, p := range remainingPlayers {
		userIDs = append(userIDs, p.UserID)
	}

	payload := map[string]interface{}{
		"roomId":      roomID,
		"userId":      userID,
		"username":    leavingUsername,
		"playerCount": actualPlayerCount,
	}
	if postRoom != nil && room.OwnerID == userID && len(remainingPlayers) > 0 {
		payload["newOwnerId"] = postRoom.OwnerID
	}

	h.broadcastToUsers(userIDs, ws.SystemPlayerLeft, payload)

	if postRoom != nil {
		h.broadcastRoomUpdated(postRoom)
	}
}

func (h *LobbyDisconnectHandler) broadcastToUsers(userIDs []uint, msgType string, payload interface{}) {
	if h.hub == nil {
		return
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}
	msg, err := json.Marshal(ws.WSMessage{Type: msgType, Payload: payloadBytes})
	if err != nil {
		return
	}
	h.hub.BroadcastToUsers(userIDs, msg)
}

// broadcastRoomUpdated sends the lobby-wide room_updated snapshot built by roomLifecyclePayload.
func (h *LobbyDisconnectHandler) broadcastRoomUpdated(r *Room) {
	if h.hub == nil {
		return
	}
	payloadBytes, err := json.Marshal(roomLifecyclePayload(h.repo, r))
	if err != nil {
		return
	}
	msg, err := json.Marshal(ws.WSMessage{Type: ws.SystemRoomUpdated, Payload: payloadBytes})
	if err != nil {
		return
	}
	h.hub.BroadcastAll(msg)
}
