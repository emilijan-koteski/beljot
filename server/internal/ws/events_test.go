package ws_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/emilijan/beljot/server/internal/ws"
)

// TestTrumpSelectedPayload_JSONContract locks the wire field names shared with the client's TrumpSelectedPayload.
func TestTrumpSelectedPayload_JSONContract(t *testing.T) {
	p := ws.TrumpSelectedPayload{
		PlayerSeat: 2,
		TrumpSuit:  "S",
		CardID:     "7S",
	}
	data, err := json.Marshal(p)
	assert.NoError(t, err)

	var decoded map[string]any
	assert.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, float64(2), decoded["playerSeat"])
	assert.Equal(t, "S", decoded["trumpSuit"])
	assert.Equal(t, "7S", decoded["cardId"])
	assert.Len(t, decoded, 3, "payload should have exactly playerSeat, trumpSuit, cardId")
}
