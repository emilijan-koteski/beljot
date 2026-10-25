package room

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/ws"
)

// captureBroadcaster records what broadcastRoomUpdated puts on the wire. It
// satisfies Broadcaster, which is the whole reason LobbyDisconnectHandler holds
// that interface rather than a concrete *ws.Hub.
type captureBroadcaster struct {
	all [][]byte
}

func (c *captureBroadcaster) BroadcastToUsers(_ []uint, msg []byte) {
	c.all = append(c.all, msg)
}

func (c *captureBroadcaster) BroadcastAll(msg []byte) {
	c.all = append(c.all, msg)
}

// TestBroadcastRoomUpdated_CarriesEveryRuleFlag checks that the disconnect room_updated
// payload carries every per-room rule flag. It is internal because broadcastRoomUpdated
// is unexported and a real disconnect takes 10 seconds.
func TestBroadcastRoomUpdated_CarriesEveryRuleFlag(t *testing.T) {
	tests := []struct {
		name                 string
		declarationsEnabled  bool
		stopAtTarget         bool
		declarationsTransfer bool
	}{
		{name: "all at their defaults", declarationsEnabled: true, stopAtTarget: false, declarationsTransfer: true},
		{name: "declarations off, dosta on", declarationsEnabled: false, stopAtTarget: true, declarationsTransfer: true},
		{name: "declarations on, dosta on, no transfer", declarationsEnabled: true, stopAtTarget: true, declarationsTransfer: false},
		{name: "declarations off, dosta off, no transfer", declarationsEnabled: false, stopAtTarget: false, declarationsTransfer: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := &captureBroadcaster{}
			h := NewLobbyDisconnectHandler(nil, hub, nil, nil)

			h.broadcastRoomUpdated(&Room{
				ID:                   7,
				Name:                 "Test Room",
				OwnerID:              100,
				OwnerUsername:        "Owner",
				Variant:              "bitola",
				MatchMode:            "1001",
				PlayerCount:          3,
				Status:               "waiting",
				DeclarationsEnabled:  tt.declarationsEnabled,
				StopAtTarget:         tt.stopAtTarget,
				DeclarationsTransfer: tt.declarationsTransfer,
			})

			require.Len(t, hub.all, 1, "expected exactly one broadcast")

			var msg ws.WSMessage
			require.NoError(t, json.Unmarshal(hub.all[0], &msg))
			assert.Equal(t, ws.SystemRoomUpdated, msg.Type)

			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal(msg.Payload, &payload))

			declarations, ok := payload["declarationsEnabled"]
			require.True(t, ok, "declarationsEnabled key missing from the payload")
			assert.Equal(t, tt.declarationsEnabled, declarations)

			stop, ok := payload["stopAtTarget"]
			require.True(t, ok, "stopAtTarget key missing from the payload")
			assert.Equal(t, tt.stopAtTarget, stop)

			transfer, ok := payload["declarationsTransfer"]
			require.True(t, ok, "declarationsTransfer key missing from the payload")
			assert.Equal(t, tt.declarationsTransfer, transfer)
		})
	}
}

// TestBroadcastRoomUpdated_TolerantOfANilHub checks that a typed-nil *ws.Hub handed to
// NewLobbyDisconnectHandler does not panic on broadcast. Without the constructor's
// normalization the nil guards stop firing.
func TestBroadcastRoomUpdated_TolerantOfANilHub(t *testing.T) {
	cases := []struct {
		name string
		hub  Broadcaster
	}{
		{name: "nil interface", hub: nil},
		{name: "typed-nil *ws.Hub", hub: (*ws.Hub)(nil)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewLobbyDisconnectHandler(nil, tc.hub, nil, nil)

			require.NotPanics(t, func() {
				h.broadcastRoomUpdated(&Room{
					ID: 7, Name: "Test Room", OwnerID: 100, OwnerUsername: "Owner",
					Variant: "bitola", MatchMode: "1001", Status: "waiting",
					DeclarationsEnabled: true, StopAtTarget: true,
				})
				h.broadcastToUsers([]uint{1}, ws.SystemRoomUpdated, map[string]any{"id": 7})
			}, "a handler with no hub must silently skip broadcasting, not panic")
		})
	}
}

// The host disc on a lobby card reads ownerAvatarUrl, so the disconnect-driven
// room_updated must carry it like the other two hand-built room payloads: the
// URL when the owner has one, an explicit null when not.
func TestBroadcastRoomUpdated_CarriesOwnerAvatarURL(t *testing.T) {
	url := "https://assets.test/avatars/33333333-3333-4333-8333-333333333333/128.webp"
	for name, avatar := range map[string]*string{"with avatar": &url, "without": nil} {
		t.Run(name, func(t *testing.T) {
			hub := &captureBroadcaster{}
			h := NewLobbyDisconnectHandler(nil, hub, nil, nil)
			h.broadcastRoomUpdated(&Room{
				ID: 7, Name: "Test Room", OwnerID: 100, OwnerUsername: "Owner", OwnerAvatarURL: avatar,
				Variant: "bitola", MatchMode: "1001", Status: "waiting",
			})
			require.Len(t, hub.all, 1)
			var msg ws.WSMessage
			require.NoError(t, json.Unmarshal(hub.all[0], &msg))
			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal(msg.Payload, &payload))
			got, ok := payload["ownerAvatarUrl"]
			require.True(t, ok, "ownerAvatarUrl key missing from the payload")
			if avatar == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, url, got)
			}
		})
	}
}
