package chat

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/ws"
)

const (
	maxMessageLength = 500
	ChannelLobby     = "lobby"
	ChannelMatch     = "match"
	ChannelRoom      = "room"
)

// GameMembership reports whether a user is currently in an active game session
// and resolves the four participants of a given match.
// Wired to *match.Manager in main.go.
type GameMembership interface {
	IsUserInMatch(userID uint) bool
	MatchParticipants(roomID uint) ([4]uint, bool)
}

// RoomMembership resolves the member userIDs of a room for room-scoped chat.
// It reports waiting=false for unknown rooms and rooms whose match has started.
type RoomMembership interface {
	RoomMembers(roomID uint) (userIDs []uint, waiting bool)
}

// Broadcaster is the subset of *ws.Hub that the chat handler depends on.
// Defined as an interface to allow lightweight unit testing without spinning
// up a real hub goroutine and websocket server.
type Broadcaster interface {
	ConnectedUserIDs() []uint
	BroadcastToUsers(userIDs []uint, msg []byte)
}

// Handler processes action:chat_message events for all three scopes
// (global, match, room).
type Handler struct {
	hub      Broadcaster
	userRepo user.UserRepository
	game     GameMembership
	room     RoomMembership
}

// NewHandler creates a chat handler from its broadcaster, user repository,
// match membership and room membership dependencies.
func NewHandler(
	hub Broadcaster,
	userRepo user.UserRepository,
	game GameMembership,
	room RoomMembership,
) *Handler {
	return &Handler{hub: hub, userRepo: userRepo, game: game, room: room}
}

// HandleAction handles action:chat_message and ignores every other message type,
// so a composite router can pass it every action.
func (h *Handler) HandleAction(client *ws.Client, msg ws.WSMessage) {
	if msg.Type != ws.ActionChatMessage {
		return
	}

	var req ws.ChatMessageRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		slog.Info("chat: invalid payload", "userID", client.UserID, "error", err)
		return
	}

	text := strings.TrimSpace(req.Text)
	runeCount := utf8.RuneCountInString(text)
	if text == "" || runeCount > maxMessageLength {
		slog.Info("chat: message rejected (empty or too long)",
			"userID", client.UserID, "runes", runeCount)
		return
	}

	switch req.Channel {
	case ChannelLobby:
		h.handleGlobal(client.UserID, text)
	case ChannelMatch:
		h.handleMatch(client.UserID, req.RoomID, text)
	case ChannelRoom:
		h.handleRoom(client.UserID, req.RoomID, text)
	default:
		slog.Info("chat: unknown channel", "userID", client.UserID, "channel", req.Channel)
	}
}

// handleGlobal sends to every connected user not in a game and drops sends from
// in-game senders. The sender always gets their own echo, even after joining a game.
func (h *Handler) handleGlobal(senderID uint, text string) {
	if h.game.IsUserInMatch(senderID) {
		slog.Info("chat: global send dropped (sender in game)", "userID", senderID)
		return
	}

	sender, err := h.userRepo.FindByID(senderID)
	if err != nil || sender == nil {
		slog.Warn("chat: sender not found", "userID", senderID, "error", err)
		return
	}

	payload := ws.ChatMessagePayload{
		UserID:    sender.ID,
		Username:  sender.Username,
		Message:   text,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Scope:     ChannelLobby,
	}
	msgBytes := buildMessage(ws.SystemChatMessage, payload)
	if msgBytes == nil {
		return
	}

	recipients := h.hub.ConnectedUserIDs()
	eligible := make([]uint, 0, len(recipients))
	for _, uid := range recipients {
		if uid != senderID && h.game.IsUserInMatch(uid) {
			continue
		}
		eligible = append(eligible, uid)
	}
	h.hub.BroadcastToUsers(eligible, msgBytes)
}

func (h *Handler) handleMatch(senderID uint, roomID *uint, text string) {
	if roomID == nil {
		slog.Info("chat: match send dropped (missing roomId)", "userID", senderID)
		return
	}

	participants, ok := h.game.MatchParticipants(*roomID)
	if !ok {
		slog.Info("chat: match send dropped (unknown roomId)",
			"userID", senderID, "roomID", *roomID)
		return
	}

	authorized := false
	for _, uid := range participants {
		if uid == senderID {
			authorized = true
			break
		}
	}
	if !authorized {
		slog.Info("chat: match send dropped (sender not in match)",
			"userID", senderID, "roomID", *roomID)
		return
	}

	sender, err := h.userRepo.FindByID(senderID)
	if err != nil || sender == nil {
		slog.Warn("chat: match sender not found", "userID", senderID, "error", err)
		return
	}

	payload := ws.ChatMessagePayload{
		UserID:    sender.ID,
		Username:  sender.Username,
		Message:   text,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Scope:     ChannelMatch,
	}
	msgBytes := buildMessage(ws.SystemChatMessage, payload)
	if msgBytes == nil {
		return
	}

	h.hub.BroadcastToUsers(participants[:], msgBytes)
}

func (h *Handler) handleRoom(senderID uint, roomID *uint, text string) {
	if roomID == nil {
		slog.Info("chat: room send dropped (missing roomId)", "userID", senderID)
		return
	}

	members, waiting := h.room.RoomMembers(*roomID)
	if !waiting {
		slog.Info("chat: room send dropped (unknown room or not waiting)",
			"userID", senderID, "roomID", *roomID)
		return
	}

	authorized := false
	for _, uid := range members {
		if uid == senderID {
			authorized = true
			break
		}
	}
	if !authorized {
		slog.Info("chat: room send dropped (sender not in room)",
			"userID", senderID, "roomID", *roomID)
		return
	}

	sender, err := h.userRepo.FindByID(senderID)
	if err != nil || sender == nil {
		slog.Warn("chat: room sender not found", "userID", senderID, "error", err)
		return
	}

	payload := ws.ChatMessagePayload{
		UserID:    sender.ID,
		Username:  sender.Username,
		Message:   text,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Scope:     ChannelRoom,
	}
	msgBytes := buildMessage(ws.SystemChatMessage, payload)
	if msgBytes == nil {
		return
	}

	h.hub.BroadcastToUsers(members, msgBytes)
}

func buildMessage(eventType string, payload interface{}) []byte {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("chat: marshal payload failed", "type", eventType, "error", err)
		return nil
	}
	msg, err := json.Marshal(ws.WSMessage{Type: eventType, Payload: payloadBytes})
	if err != nil {
		slog.Error("chat: marshal message failed", "type", eventType, "error", err)
		return nil
	}
	return msg
}
