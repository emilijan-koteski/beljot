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

// FriendChecker reports whether two users have an accepted friendship. Satisfied
// by *friend.GormRepository (Story 11.2). Declared here as a narrow local
// interface so the chat package never imports friend — keeping it unit-testable
// and free of any import-cycle risk.
type FriendChecker interface {
	AreFriends(a, b uint) (bool, error)
}

// PresenceLocator resolves the waiting-or-playing room a user is currently in.
// Satisfied by an adapter over room.RoomRepository.FindPlayerRoom in main.go.
// Declared as a narrow local interface (returning a plain roomID + presence
// bool rather than a *room.RoomPlayer) so the chat package never imports room —
// no import cycle, and the anti-collusion comparison stays testable here.
type PresenceLocator interface {
	// ActiveRoomID returns the ID of the waiting-or-playing room the user is
	// currently in, and whether they are in one at all. A live match is always
	// tied to a "playing" room, so "same active room OR match" collapses to
	// "same active roomID" (Story 11.4 D3).
	ActiveRoomID(userID uint) (roomID uint, inRoom bool, err error)
}

// WhisperNotifier is the subset of *ws.Hub the whisper handler needs. It differs
// from the chat Broadcaster: whisper delivers per-user (SendToUser) and MUST
// pre-check connectivity (IsConnected), because SendToUser is a silent no-op for
// offline users — there is no send-failure signal to rely on (Story 11.4 D4).
type WhisperNotifier interface {
	IsConnected(userID uint) bool
	SendToUser(userID uint, msg []byte)
}

// WhisperHandler processes action:whisper events — private one-to-one messages
// between friends. It is a sibling of Handler (chat) rather than a method on it,
// because it needs per-user delivery + a friend/presence check that flat chat
// does not, and keeping it separate leaves the chat Handler and its tests
// untouched. Whispers are ephemeral: no model, no repository, no migration, no DB
// write — the server routes live system:whisper messages and forgets them
// (Story 11.4 D5).
type WhisperHandler struct {
	hub      WhisperNotifier
	userRepo user.UserRepository
	friends  FriendChecker
	presence PresenceLocator
}

// NewWhisperHandler wires the whisper handler to the hub (per-user delivery +
// connectivity check), the user repository (username → user resolution), the
// friend checker (Story 11.2), and the presence locator (anti-collusion).
func NewWhisperHandler(
	hub WhisperNotifier,
	userRepo user.UserRepository,
	friends FriendChecker,
	presence PresenceLocator,
) *WhisperHandler {
	return &WhisperHandler{hub: hub, userRepo: userRepo, friends: friends, presence: presence}
}

// HandleAction is the action handler entry point, composed into the hub's action
// router alongside chat + emote + the session manager. Returns silently for any
// msg.Type other than ws.ActionWhisper so the composite caller can safely route
// every action through it.
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
	// Rune count (not byte count), matching chat, so a 500-rune Cyrillic/emoji
	// whisper stays within the limit.
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

	// Resolve the typed username to a user. A non-existent username is treated as
	// "not a friend" (do NOT disclose "no such user" — that would leak the
	// registry to a probe).
	target, err := h.userRepo.FindByUsername(toUsername)
	if err != nil {
		slog.Warn("whisper: target lookup failed", "toUsername", toUsername, "error", err)
		return // fail closed on an internal error — no delivery, no leak
	}
	if target == nil {
		h.sendError(senderID, ws.ErrorNotFriends)
		return
	}

	// Ignore a self-whisper. Compare by ID (post-resolution) so username casing
	// or whitespace can't defeat the check.
	if target.ID == senderID {
		slog.Info("whisper: self-target ignored", "userID", senderID)
		return
	}

	// Friends-only, server-authoritative (Story 11.4 AC2). A client-only gate is
	// defeatable, so this is the authoritative check.
	friends, err := h.friends.AreFriends(senderID, target.ID)
	if err != nil {
		slog.Error("whisper: friend check failed", "sender", senderID, "target", target.ID, "error", err)
		return // fail closed
	}
	if !friends {
		h.sendError(senderID, ws.ErrorNotFriends)
		return
	}

	// Anti-collusion (Story 11.4 AC3 / D3): reject whispering someone in the
	// sender's current room or match. Both must be in a room AND it must be the
	// same room. FindPlayerRoom matches waiting OR playing, so this one comparison
	// covers a shared waiting room AND a shared active match.
	senderRoom, senderInRoom, err := h.presence.ActiveRoomID(senderID)
	if err != nil {
		slog.Error("whisper: sender presence lookup failed", "userID", senderID, "error", err)
		return // fail closed
	}
	targetRoom, targetInRoom, err := h.presence.ActiveRoomID(target.ID)
	if err != nil {
		slog.Error("whisper: target presence lookup failed", "userID", target.ID, "error", err)
		return // fail closed
	}
	if senderInRoom && targetInRoom && senderRoom == targetRoom {
		h.sendError(senderID, ws.ErrorWhisperBlockedInGame)
		return
	}

	// Offline recipient (Story 11.4 AC4 / D4): whispers are real-time only.
	// IsConnected MUST be an explicit pre-check — SendToUser silently drops for
	// offline users, so there is otherwise no delivery-failure feedback.
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

	// Deliver to BOTH participants: the recipient, and the sender (own-echo, so
	// the sender's own thread renders their message — chat does the same).
	h.hub.SendToUser(target.ID, msgBytes)
	h.hub.SendToUser(senderID, msgBytes)
}

// sendError emits an error:* event to the sender only (never the target),
// mirroring the per-user error pattern. error:* is outside the drift gate.
func (h *WhisperHandler) sendError(userID uint, errorType string) {
	h.hub.SendToUser(userID, buildMessage(errorType, map[string]string{"type": errorType}))
}
