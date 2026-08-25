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

// An internal test (package room) on purpose: broadcastRoomUpdated is
// unexported, and driving it from room_test would mean provoking a real lobby
// disconnect and waiting out the 10-second seat-free timeout to assert on two
// map keys. bracket_test.go and variant_allowlist_test.go already use this style
// in this package.
//
// This guards the payload map with the worst track record of the three: it fires
// only on a lobby disconnect, so a missing key does not show up in any ordinary
// create/join flow. declarationsEnabled was in fact omitted here when the
// declarations toggle shipped, and a bez-zvanja room lost its lobby chip
// whenever any lobby player dropped. Both rule flags are asserted together so
// the next per-room rule has a place to land.
func TestBroadcastRoomUpdated_CarriesBothRuleFlags(t *testing.T) {
	tests := []struct {
		name                string
		declarationsEnabled bool
		stopAtTarget        bool
	}{
		{name: "both at their defaults", declarationsEnabled: true, stopAtTarget: false},
		{name: "declarations off, dosta on", declarationsEnabled: false, stopAtTarget: true},
		{name: "declarations on, dosta on", declarationsEnabled: true, stopAtTarget: true},
		{name: "declarations off, dosta off", declarationsEnabled: false, stopAtTarget: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := &captureBroadcaster{}
			h := NewLobbyDisconnectHandler(nil, hub, nil, nil)

			// OwnerUsername is set so the hydration branch does not reach the repo,
			// which lets this test run with a nil repo.
			h.broadcastRoomUpdated(&Room{
				ID:                  7,
				Name:                "Test Room",
				OwnerID:             100,
				OwnerUsername:       "Owner",
				Variant:             "bitola",
				MatchMode:           "1001",
				PlayerCount:         3,
				Status:              "waiting",
				DeclarationsEnabled: tt.declarationsEnabled,
				StopAtTarget:        tt.stopAtTarget,
			})

			require.Len(t, hub.all, 1, "expected exactly one broadcast")

			var msg ws.WSMessage
			require.NoError(t, json.Unmarshal(hub.all[0], &msg))
			assert.Equal(t, ws.SystemRoomUpdated, msg.Type)

			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal(msg.Payload, &payload))

			// Assert PRESENCE separately from value: an omitted key and a key set to
			// false both read as false through a plain equality check, so a dropped
			// key would pass a value-only assertion in the default rows above.
			declarations, ok := payload["declarationsEnabled"]
			require.True(t, ok, "declarationsEnabled key missing from the payload")
			assert.Equal(t, tt.declarationsEnabled, declarations)

			stop, ok := payload["stopAtTarget"]
			require.True(t, ok, "stopAtTarget key missing from the payload")
			assert.Equal(t, tt.stopAtTarget, stop)
		})
	}
}

// TestBroadcastRoomUpdated_TolerantOfANilHub pins the one hazard the Broadcaster
// narrowing introduced. The broadcast helpers still guard on `h.hub == nil`, a
// test written when the field was a concrete *ws.Hub and nil really was nil. Put
// a typed-nil *ws.Hub inside the interface and that guard stops firing — the
// interface value is non-nil — so the next line dereferences nothing and panics.
//
// NewLobbyDisconnectHandler normalizes it back to a nil interface instead of the
// guards being deleted, which keeps the "no hub means no broadcasting" affordance
// that presence and invites already offer. This is what proves the normalization
// is there: without it this test panics rather than failing.
func TestBroadcastRoomUpdated_TolerantOfANilHub(t *testing.T) {
	cases := []struct {
		name string
		hub  Broadcaster
	}{
		{name: "nil interface", hub: nil},
		// The typed-nil case: this is what a caller wiring up an unconfigured
		// *ws.Hub actually hands in.
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
