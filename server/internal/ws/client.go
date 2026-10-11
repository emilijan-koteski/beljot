package ws

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	pingInterval = 30 * time.Second
	pingTimeout  = 10 * time.Second
	sendBufSize  = 256
)

// Client represents a single authenticated WebSocket connection.
type Client struct {
	conn   *websocket.Conn
	hub    *Hub
	UserID uint
	send   chan []byte

	closedMu sync.Mutex
	closed   bool
}

// NewClient creates a Client bound to a hub and connection.
func NewClient(conn *websocket.Conn, hub *Hub, userID uint) *Client {
	return &Client{
		conn:   conn,
		hub:    hub,
		UserID: userID,
		send:   make(chan []byte, sendBufSize),
	}
}

// Send enqueues a message for delivery and is safe to call concurrently.
// It returns false if the client is closed or the buffer is full.
func (c *Client) Send(msg []byte) bool {
	c.closedMu.Lock()
	if c.closed {
		c.closedMu.Unlock()
		return false
	}
	select {
	case c.send <- msg:
		c.closedMu.Unlock()
		return true
	default:
		c.closedMu.Unlock()
		slog.Warn("ws: send buffer full, dropping message", "userID", c.UserID)
		return false
	}
}

// markClosed marks the client as closed and closes the send channel.
// Safe to call multiple times — only the first call has effect.
func (c *Client) markClosed() {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.send)
	}
}

// readPump forwards incoming messages to the hub until the connection closes.
// It runs pingLoop alongside Read because pong frames are only processed by an active reader.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.CloseNow()
	}()

	ctx, cancelPing := context.WithCancel(context.Background())
	defer cancelPing()
	go c.pingLoop(ctx)

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			status := websocket.CloseStatus(err)
			if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
				slog.Info("ws: client closed", "userID", c.UserID, "status", status)
			} else {
				slog.Info("ws: read error", "userID", c.UserID, "error", err)
			}
			return
		}

		select {
		case c.hub.incoming <- &IncomingMessage{Client: c, Data: data}:
		default:
			slog.Warn("ws: incoming buffer full, closing client", "userID", c.UserID)
			return
		}
	}
}

func (c *Client) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			if err := c.conn.Ping(pingCtx); err != nil {
				cancel()
				slog.Info("ws: ping failed", "userID", c.UserID, "error", err)
				_ = c.conn.CloseNow()
				return
			}
			cancel()
		}
	}
}

// writePump writes messages from the send channel to the WebSocket connection.
// Only one goroutine should call writePump.
func (c *Client) writePump() {
	defer func() { _ = c.conn.CloseNow() }()

	for msg := range c.send {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.conn.Write(ctx, websocket.MessageText, msg)
		cancel()
		if err != nil {
			slog.Info("ws: write error", "userID", c.UserID, "error", err)
			return
		}
	}
}
