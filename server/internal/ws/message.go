package ws

import (
	"encoding/json"
	"time"
)

// WSMessage is the wire format for all WebSocket communication, with Type in prefix:event_name form.
// ServerNow is set by senders whose payloads carry absolute deadlines so clients can correct for clock skew.
type WSMessage struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	ServerNow *time.Time      `json:"serverNow,omitempty"`
}
