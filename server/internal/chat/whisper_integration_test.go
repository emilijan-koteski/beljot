package chat_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/chat"
	"github.com/emilijan/beljot/server/internal/ws"
)

const whisperTestSecret = "test-jwt-secret-for-whisper-tests"

// setupWhisperServer wires a REAL hub + WSHandler and routes action:whisper to a
// real chat.WhisperHandler (fakes only for the friend/presence/user deps, which
// have their own DB-backed tests). This exercises the full websocket path:
// dial → auth handshake → hub registration → per-user SendToUser delivery.
func setupWhisperServer(t *testing.T) *httptest.Server {
	t.Helper()
	hub := ws.NewHub()
	go hub.Run()
	t.Cleanup(hub.Shutdown)

	repo := newUserRepoStub()
	repo.add(1, "alice")
	repo.add(2, "bob")
	repo.add(3, "carol")
	friends := newFakeFriends([2]uint{1, 2}) // alice & bob are friends; carol is not
	presence := newFakePresence()            // nobody in a room → anti-collusion never fires

	whisperHandler := chat.NewWhisperHandler(hub, repo, friends, presence)
	hub.SetActionHandler(func(client *ws.Client, msg ws.WSMessage) {
		whisperHandler.HandleAction(client, msg)
	})

	e := echo.New()
	wsHandler := &ws.WSHandler{
		Hub:       hub,
		JWTSecret: whisperTestSecret,
		ValidateToken: func(token string) ([]string, string, error) {
			claims, err := auth.ValidateToken(token, whisperTestSecret)
			if err != nil {
				return nil, "", err
			}
			return []string(claims.Audience), claims.Subject, nil
		},
	}
	e.GET("/ws", wsHandler.HandleWS)

	server := httptest.NewServer(e)
	t.Cleanup(server.Close)
	return server
}

func dialWhisperClient(t *testing.T, server *httptest.Server, userID uint) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, url, nil)
	require.NoError(t, err)

	token, err := auth.GenerateAccessToken(userID, whisperTestSecret)
	require.NoError(t, err)

	tokenPayload, err := json.Marshal(map[string]string{"token": token})
	require.NoError(t, err)
	writeWhisper(t, conn, ws.WSMessage{Type: ws.ActionAuthenticate, Payload: tokenPayload})

	// Consume the system:authenticated ack.
	ack := readWhisper(t, conn, 5*time.Second)
	require.Equal(t, ws.SystemAuthenticated, ack.Type)
	return conn
}

func writeWhisper(t *testing.T, conn *websocket.Conn, msg ws.WSMessage) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Write(ctx, conn, msg))
}

func readWhisper(t *testing.T, conn *websocket.Conn, timeout time.Duration) ws.WSMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var msg ws.WSMessage
	require.NoError(t, wsjson.Read(ctx, conn, &msg))
	return msg
}

func TestWhisperIntegration_DeliveredToBothAndNotToOthers(t *testing.T) {
	server := setupWhisperServer(t)

	alice := dialWhisperClient(t, server, 1)
	defer func() { _ = alice.CloseNow() }()
	bob := dialWhisperClient(t, server, 2)
	defer func() { _ = bob.CloseNow() }()
	carol := dialWhisperClient(t, server, 3)
	defer func() { _ = carol.CloseNow() }()

	// Wait for all three hub registrations to settle.
	time.Sleep(80 * time.Millisecond)

	// alice whispers bob.
	writeWhisper(t, alice, whisperMessage(t, "bob", "just between us"))

	// bob (the recipient) receives exactly one system:whisper with the payload.
	got := readWhisper(t, bob, 3*time.Second)
	assert.Equal(t, ws.SystemWhisper, got.Type)
	var payload ws.WhisperPayload
	require.NoError(t, json.Unmarshal(got.Payload, &payload))
	assert.Equal(t, uint(1), payload.FromUserID)
	assert.Equal(t, "alice", payload.FromUsername)
	assert.Equal(t, uint(2), payload.ToUserID)
	assert.Equal(t, "bob", payload.ToUsername)
	assert.Equal(t, "just between us", payload.Message)

	// alice receives her own-echo.
	selfEcho := readWhisper(t, alice, 3*time.Second)
	assert.Equal(t, ws.SystemWhisper, selfEcho.Type)

	// carol (uninvolved) receives NOTHING — a short read must time out.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var stray ws.WSMessage
	err := wsjson.Read(ctx, carol, &stray)
	assert.Error(t, err, "an uninvolved third party must never receive a whisper")
}
