package emote

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/ws"
)

const rateLimitWindow = 3 * time.Second

// Broadcaster is the subset of *ws.Hub that the emote handler depends on.
// Mirrors the chat-handler abstraction so the unit tests can swap a spy in
// place of the real hub goroutine.
type Broadcaster interface {
	BroadcastToUsers(userIDs []uint, msg []byte)
}

// GameMembership resolves a sender's four match participants and seat by userID,
// since the emote payload carries no matchID.
type GameMembership interface {
	MatchParticipantsByUser(userID uint) (participants [4]uint, seat int, ok bool)
}

// Handler processes action:emote events: validates the emote whitelist,
// enforces the per-user rate limit, and broadcasts system:emote to all four
// match participants (sender included for the own-echo).
type Handler struct {
	hub  Broadcaster
	game GameMembership

	mu          sync.Mutex
	lastEmoteAt map[uint]time.Time
}

// NewHandler creates an emote handler wired to the WebSocket broadcaster and
// the session manager (for in-game membership + participant lookup).
func NewHandler(hub Broadcaster, game GameMembership) *Handler {
	return &Handler{
		hub:         hub,
		game:        game,
		lastEmoteAt: make(map[uint]time.Time),
	}
}

// HandleAction handles action:emote and ignores every other message type,
// so a composite router can pass it every action.
func (h *Handler) HandleAction(client *ws.Client, msg ws.WSMessage) {
	if msg.Type != ws.ActionEmote {
		return
	}

	var req ws.EmoteRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		slog.Info("emote: invalid payload", "userID", client.UserID, "error", err)
		return
	}

	if _, ok := ws.ValidEmoteIDs[ws.EmoteID(req.Emote)]; !ok {
		slog.Info("emote: unknown id", "userID", client.UserID, "emote", req.Emote)
		return
	}

	participants, seat, ok := h.game.MatchParticipantsByUser(client.UserID)
	if !ok {
		slog.Info("emote: sender not in active match", "userID", client.UserID)
		return
	}

	if !h.acceptRateLimited(client.UserID, time.Now()) {
		slog.Info("emote: rate-limited", "userID", client.UserID)
		return
	}

	payload := ws.EmotePayload{
		PlayerSeat: seat,
		Emote:      ws.EmoteID(req.Emote),
	}
	msgBytes := buildMessage(ws.SystemEmote, payload)
	if msgBytes == nil {
		return
	}
	h.hub.BroadcastToUsers(participants[:], msgBytes)
}

// RemoveUser deletes the per-user rate-limit entry, bounding the map to
// current-active users (not lifetime users). Called via match.Manager's
// UserRemovedHook on session teardown.
func (h *Handler) RemoveUser(userID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.lastEmoteAt, userID)
}

// acceptRateLimited atomically checks the per-user cooldown and stamps the
// new send timestamp on success. Returns false when the previous emote from
// this user was within rateLimitWindow.
func (h *Handler) acceptRateLimited(userID uint, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if last, ok := h.lastEmoteAt[userID]; ok && now.Sub(last) < rateLimitWindow {
		return false
	}
	h.lastEmoteAt[userID] = now
	return true
}

func buildMessage(eventType string, payload interface{}) []byte {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("emote: marshal payload failed", "type", eventType, "error", err)
		return nil
	}
	msg, err := json.Marshal(ws.WSMessage{Type: eventType, Payload: payloadBytes})
	if err != nil {
		slog.Error("emote: marshal message failed", "type", eventType, "error", err)
		return nil
	}
	return msg
}
