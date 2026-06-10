package ws

import (
	"encoding/json"
	"time"
)

// WSMessage is the wire format for all WebSocket communication.
// Type uses prefix:event_name format (action:, event:, error:, system:).
// Payload is lazily parsed — the router only needs the type for dispatch.
//
// ServerNow is the server's wall clock at send time, stamped by senders whose
// payloads carry absolute deadlines (match events: turnExpiresAt,
// reconnectExpiresAt). Clients sample `serverNow - Date.now()` to estimate the
// clock offset and render countdowns against corrected time, so a skewed
// client clock can't make a deadline appear longer or shorter than it is.
// Optional: senders without deadline-bearing payloads omit it, and clients
// fall back to the uncorrected local clock until the first sample arrives.
type WSMessage struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	ServerNow *time.Time      `json:"serverNow,omitempty"`
}
