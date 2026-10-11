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

// FriendChecker reports whether two users have an accepted friendship.
// It is declared locally so the chat package never imports friend.
type FriendChecker interface {
	AreFriends(a, b uint) (bool, error)
}

// PresenceLocator resolves the waiting-or-playing room a user is in, if any.
// A live match always has a "playing" room, so one roomID covers both cases.
type PresenceLocator interface {
	ActiveRoomID(userID uint) (roomID uint, inRoom bool, err error)
}

// WhisperNotifier is the subset of *ws.Hub the whisper handler needs.
// SendToUser silently drops for offline users, so callers check IsConnected first.
type WhisperNotifier interface {
	IsConnected(userID uint) bool
	SendToUser(userID uint, msg []byte)
}

// WhisperHandler processes action:whisper events, private messages between friends.
// Whispers are ephemeral: they are routed live and never stored.
type WhisperHandler struct {
	hub      WhisperNotifier
	userRepo user.UserRepository
	friends  FriendChecker
	presence PresenceLocator
}

// NewWhisperHandler creates a whisper handler from its hub, user repository,
// friend checker and presence locator.
func NewWhisperHandler(
	hub WhisperNotifier,
	userRepo user.UserRepository,
	friends FriendChecker,
	presence PresenceLocator,
) *WhisperHandler {
	return &WhisperHandler{hub: hub, userRepo: userRepo, friends: friends, presence: presence}
}

// HandleAction delivers a whisper to an online friend outside the sender's room or
// match, ignoring other message types. Unknown usernames get the not-friends
// error so the user registry cannot be probed.
func (h *WhisperHandler) HandleAction(client *ws.Client, msg ws.WSMessage) {
	if msg.Type != ws.ActionWhisper {
		return
	}

	var req ws.WhisperRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		slog.Info("whisper: invalid payload", "userID", client.UserID, "error", err)
		return
	}

	text := strings.TrimSpace(req.Text)
	runeCount := utf8.RuneCountInString(text)
	if text == "" || runeCount > maxMessageLength {
		slog.Info("whisper: message rejected (empty or too long)",
			"userID", client.UserID, "runes", runeCount)
		return
	}

	toUsername := strings.TrimSpace(req.ToUsername)
	if toUsername == "" {
		slog.Info("whisper: empty target ignored", "userID", client.UserID)
		return
	}

	senderID := client.UserID

	sender, err := h.userRepo.FindByID(senderID)
	if err != nil || sender == nil {
		slog.Warn("whisper: sender not found", "userID", senderID, "error", err)
		return
	}

	target, err := h.userRepo.FindByUsername(toUsername)
	if err != nil {
		slog.Warn("whisper: target lookup failed", "toUsername", toUsername, "error", err)
		return
	}
	if target == nil {
		h.sendError(senderID, ws.ErrorNotFriends)
		return
	}

	if target.ID == senderID {
		slog.Info("whisper: self-target ignored", "userID", senderID)
		return
	}

	friends, err := h.friends.AreFriends(senderID, target.ID)
	if err != nil {
		slog.Error("whisper: friend check failed", "sender", senderID, "target", target.ID, "error", err)
		return
	}
	if !friends {
		h.sendError(senderID, ws.ErrorNotFriends)
		return
	}

	senderRoom, senderInRoom, err := h.presence.ActiveRoomID(senderID)
	if err != nil {
		slog.Error("whisper: sender presence lookup failed", "userID", senderID, "error", err)
		return
	}
	targetRoom, targetInRoom, err := h.presence.ActiveRoomID(target.ID)
	if err != nil {
		slog.Error("whisper: target presence lookup failed", "userID", target.ID, "error", err)
		return
	}
	if senderInRoom && targetInRoom && senderRoom == targetRoom {
		h.sendError(senderID, ws.ErrorWhisperBlockedInGame)
		return
	}

	if !h.hub.IsConnected(target.ID) {
		h.sendError(senderID, ws.ErrorWhisperRecipientOffline)
		return
	}

	payload := ws.WhisperPayload{
		FromUserID:   sender.ID,
		FromUsername: sender.Username,
		ToUserID:     target.ID,
		ToUsername:   target.Username,
		Message:      text,
		Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
	}
	msgBytes := buildMessage(ws.SystemWhisper, payload)
	if msgBytes == nil {
		return
	}

	h.hub.SendToUser(target.ID, msgBytes)
	h.hub.SendToUser(senderID, msgBytes)
}

// sendError emits an error:* event to the sender only (never the target),
// mirroring the per-user error pattern. error:* is outside the drift gate.
func (h *WhisperHandler) sendError(userID uint, errorType string) {
	h.hub.SendToUser(userID, buildMessage(errorType, map[string]string{"type": errorType}))
}
