package ws

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/coder/websocket"
)

// CloseCodeReplaced signals a kicked-by-newer-connection close. Sits in the
// app-defined 4000-4999 range so it can't collide with RFC 6455 reserved codes.
// The client uses this to skip its auto-reconnect path so two tabs as the same
// user don't enter a kick → reconnect loop.
const CloseCodeReplaced websocket.StatusCode = 4001

// IncomingMessage pairs a raw message with the client that sent it.
type IncomingMessage struct {
	Client *Client
	Data   []byte
}

// ActionHandler processes action: prefixed messages from clients.
type ActionHandler func(client *Client, msg WSMessage)

// SystemHandler processes system: prefixed messages from clients.
type SystemHandler func(client *Client, msg WSMessage)

// ConnectHandler is called when a client registers (connects or reconnects).
// Used by session manager to detect game reconnections.
type ConnectHandler func(userID uint)

// DisconnectHandler is called when a client truly disconnects (not replaced by a new connection).
type DisconnectHandler func(userID uint)

// Hub manages all active WebSocket connections and routes messages.
type Hub struct {
	clients    map[uint]*Client
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	incoming   chan *IncomingMessage
	done       chan struct{}

	actionHandler     ActionHandler
	systemHandler     SystemHandler
	connectHandler    ConnectHandler
	disconnectHandler DisconnectHandler
}

// NewHub creates a new Hub ready to run.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uint]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		incoming:   make(chan *IncomingMessage, 256),
		done:       make(chan struct{}),
	}
}

// SetActionHandler registers a handler for action: prefixed messages.
// This will be wired to the session manager in Story 4.2.
func (h *Hub) SetActionHandler(handler ActionHandler) {
	h.actionHandler = handler
}

// SetSystemHandler registers a handler for system: prefixed messages.
// This will be wired to lobby/chat handlers in future stories.
func (h *Hub) SetSystemHandler(handler SystemHandler) {
	h.systemHandler = handler
}

// SetConnectHandler registers a callback for client registrations.
// Called when any client connects (including reconnections).
func (h *Hub) SetConnectHandler(handler ConnectHandler) {
	h.connectHandler = handler
}

// SetDisconnectHandler registers a callback for true client disconnections.
// Called when a client is unregistered and NOT replaced by a new connection.
func (h *Hub) SetDisconnectHandler(handler DisconnectHandler) {
	h.disconnectHandler = handler
}

// Run starts the hub's main event loop. Call as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			h.shutdownClients()
			return

		case client := <-h.register:
			h.mu.Lock()
			if existing, ok := h.clients[client.UserID]; ok {
				existing.markClosed()
				// Send a clean close frame with the app-specific "replaced"
				// status so the kicked client knows not to auto-reconnect —
				// otherwise two tabs/devices as the same user enter a tight
				// reconnect/replace loop. Falls back to CloseNow on error.
				if err := existing.conn.Close(CloseCodeReplaced, "replaced by newer connection"); err != nil {
					_ = existing.conn.CloseNow()
				}
				slog.Info("ws: replaced existing connection", "userID", client.UserID)
			}
			h.clients[client.UserID] = client
			h.mu.Unlock()
			slog.Info("ws: client registered", "userID", client.UserID)
			// Fire connect handler for reconnection detection
			if h.connectHandler != nil {
				go h.connectHandler(client.UserID)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if current, ok := h.clients[client.UserID]; ok && current == client {
				delete(h.clients, client.UserID)
				client.markClosed()
				slog.Info("ws: client unregistered", "userID", client.UserID)
				// Fire disconnect handler only for true disconnects (not replacements)
				if h.disconnectHandler != nil {
					go h.disconnectHandler(client.UserID)
				}
			}
			h.mu.Unlock()

		case incoming := <-h.incoming:
			h.handleMessage(incoming)
		}
	}
}

// Shutdown signals the hub to stop and close all client connections.
func (h *Hub) Shutdown() {
	close(h.done)
}

func (h *Hub) shutdownClients() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, client := range h.clients {
		client.markClosed()
		_ = client.conn.CloseNow()
	}
	h.clients = make(map[uint]*Client)
}

func (h *Hub) handleMessage(incoming *IncomingMessage) {
	var msg WSMessage
	if err := json.Unmarshal(incoming.Data, &msg); err != nil {
		slog.Warn("ws: invalid message format", "userID", incoming.Client.UserID, "error", err)
		h.SendToUser(incoming.Client.UserID, buildErrorMessage(ErrorUnknownEvent, "invalid message format"))
		return
	}

	prefix := extractPrefix(msg.Type)
	switch prefix {
	case "action":
		if h.actionHandler != nil {
			go h.actionHandler(incoming.Client, msg)
		} else {
			slog.Warn("ws: unhandled action (no handler registered)", "type", msg.Type, "userID", incoming.Client.UserID)
		}
	case "system":
		if h.systemHandler != nil {
			go h.systemHandler(incoming.Client, msg)
		} else {
			slog.Warn("ws: unhandled system event (no handler registered)", "type", msg.Type, "userID", incoming.Client.UserID)
		}
	default:
		slog.Warn("ws: unknown event prefix", "type", msg.Type, "userID", incoming.Client.UserID)
		h.SendToUser(incoming.Client.UserID, buildErrorMessage(ErrorUnknownEvent, "unknown event type: "+msg.Type))
	}
}

// SendToUser sends a message to a specific user by ID.
func (h *Hub) SendToUser(userID uint, msg []byte) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()
	if ok {
		client.Send(msg)
	}
}

// UserFrame pairs one recipient with the exact bytes that recipient must
// receive. The unit of SendFrames: one logical event whose payload differs per
// recipient (the per-seat projected match_state, Story 12.10).
type UserFrame struct {
	UserID uint
	Msg    []byte
}

// SendFrames delivers each frame to its own recipient, in slice order, under a
// single lock pass. The per-recipient counterpart of BroadcastToUsers: where
// that fans identical bytes out to many users, this carries a DISTINCT frame
// per user while remaining ONE logical send — so a per-seat state event stays
// one call on the wire log rather than decomposing into N unicasts. Unknown
// user IDs are tolerated, mirroring BroadcastToUsers. An empty batch is a
// legal no-op — a match whose only human seat just disconnected has nobody
// left to frame.
func (h *Hub) SendFrames(frames []UserFrame) {
	if len(frames) == 0 {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, f := range frames {
		if client, ok := h.clients[f.UserID]; ok {
			client.Send(f.Msg)
		}
	}
}

// BroadcastToUsers sends a message to multiple users by their IDs.
func (h *Hub) BroadcastToUsers(userIDs []uint, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, uid := range userIDs {
		if client, ok := h.clients[uid]; ok {
			client.Send(msg)
		}
	}
}

// BroadcastAll sends a message to all connected clients.
func (h *Hub) BroadcastAll(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.clients {
		client.Send(msg)
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ConnectedUserIDs returns a snapshot of all currently-connected user IDs.
// Order is unspecified. Caller may mutate the returned slice freely.
func (h *Hub) ConnectedUserIDs() []uint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]uint, 0, len(h.clients))
	for uid := range h.clients {
		ids = append(ids, uid)
	}
	return ids
}

// IsConnected checks if a user has an active WebSocket connection.
func (h *Hub) IsConnected(userID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

func extractPrefix(eventType string) string {
	for i := 0; i < len(eventType); i++ {
		if eventType[i] == ':' {
			return eventType[:i]
		}
	}
	return ""
}

func buildErrorMessage(errorType string, message string) []byte {
	payload, _ := json.Marshal(map[string]string{"message": message})
	msg, _ := json.Marshal(WSMessage{
		Type:    errorType,
		Payload: payload,
	})
	return msg
}
