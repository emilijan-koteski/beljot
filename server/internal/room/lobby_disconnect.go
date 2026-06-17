package room

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/ws"
)

const lobbyDisconnectTimeout = 10 * time.Second

// LobbyDisconnectHandler manages pre-game lobby disconnections.
// When a player disconnects while in a room lobby (status "waiting"),
// their seat is freed after a short timeout (10 seconds). If they
// reconnect within the window, the timer is cancelled.
type LobbyDisconnectHandler struct {
	repo     RoomRepository
	hub      *ws.Hub
	presence *PresenceRegistry
	mu       sync.Mutex
	pending  map[uint]*lobbyDisconnect // userID → pending disconnect
}

type lobbyDisconnect struct {
	timer  *time.Timer
	roomID uint
}

// NewLobbyDisconnectHandler creates a new handler for lobby disconnect events.
func NewLobbyDisconnectHandler(repo RoomRepository, hub *ws.Hub, presence *PresenceRegistry) *LobbyDisconnectHandler {
	if presence == nil {
		presence = NewPresenceRegistry()
	}
	return &LobbyDisconnectHandler{
		repo:     repo,
		hub:      hub,
		presence: presence,
		pending:  make(map[uint]*lobbyDisconnect),
	}
}

// HandleDisconnect is called when a player disconnects. If the player is in a
// room lobby (status "waiting"), a 10-second timer starts. On expiry the player
// is removed from the room and remaining players are notified.
func (h *LobbyDisconnectHandler) HandleDisconnect(userID uint) {
	rp, err := h.repo.FindPlayerRoom(userID)
	if err != nil || rp == nil {
		return // Not in any room
	}

	room, err := h.repo.FindByID(rp.RoomID)
	if err != nil || room == nil {
		return
	}

	// Only handle lobby disconnects (room not yet started)
	if room.Status != "waiting" {
		return
	}

	h.mu.Lock()
	// Cancel any existing timer for this user (shouldn't happen, but be safe)
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

	// Look up player info before removing
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
	// Double-check room is still in waiting status
	if room.Status != "waiting" {
		return
	}

	// Remove the player from the room
	if err := h.repo.RunInTransaction(func(tx RoomRepository) error {
		if err := tx.RemovePlayer(roomID, userID); err != nil {
			return err
		}
		if err := tx.DecrementPlayerCount(roomID); err != nil {
			return err
		}
		// Handle owner transfer or room closure
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
			return tx.Update(freshRoom)
		}
		return nil
	}); err != nil {
		slog.Error("room: failed to remove disconnected lobby player", "userID", userID, "roomID", roomID, "error", err)
		return
	}

	slog.Info("room: lobby player removed after disconnect timeout", "userID", userID, "roomID", roomID)

	// Drop their presence (auto-clears the room entry when the last present
	// member times out).
	h.presence.Remove(roomID, userID)

	// Broadcast player_left to remaining room participants
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

	// Broadcast room_updated to lobby
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

func (h *LobbyDisconnectHandler) broadcastRoomUpdated(r *Room) {
	if h.hub == nil {
		return
	}
	if r.OwnerUsername == "" {
		// Best-effort hydration so disconnect-driven updates carry the host
		// avatar the lobby card needs. Silently swallow errors — the rest of
		// the payload is still valid without the username field.
		_ = h.repo.LoadOwnerUsernames([]*Room{r})
	}
	payload := map[string]interface{}{
		"id":                   r.ID,
		"name":                 r.Name,
		"code":                 r.Code,
		"ownerId":              r.OwnerID,
		"ownerUsername":        r.OwnerUsername,
		"variant":              r.Variant,
		"matchMode":            r.MatchMode,
		"timerStyle":           r.TimerStyle,
		"timerDurationSeconds": r.TimerDurationSeconds,
		"playerCount":          r.PlayerCount,
		"status":               r.Status,
		"createdAt":            r.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":            r.UpdatedAt.UTC().Format(time.RFC3339),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}
	msg, err := json.Marshal(ws.WSMessage{Type: ws.SystemRoomUpdated, Payload: payloadBytes})
	if err != nil {
		return
	}
	h.hub.BroadcastAll(msg)
}
