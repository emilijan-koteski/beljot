package ws

// WebSocket event contract — keep in sync with client/src/shared/types/wsEvents.ts

// Event type prefixes:
// action: — client -> server
// event:  — server -> client (game state)
// error:  — server -> client (errors)
// system: — server -> client (platform events)

// --- Authentication events ---
const ActionAuthenticate = "action:authenticate"
const SystemAuthenticated = "system:authenticated"
const ErrorAuthFailed = "error:auth_failed"

// --- Game action events (client -> server) ---
const ActionPlayCard = "action:play_card"
const ActionPickTrump = "action:pick_trump"
const ActionPassTrump = "action:pass_trump"
const ActionDeclare = "action:declare"
const ActionSkipDeclare = "action:skip_declare"
const ActionAnnounceBelot = "action:announce_belot"
const ActionDeclineBelot = "action:decline_belot"
const ActionPause = "action:pause"
const ActionUnpause = "action:unpause"
const ActionOwnerUnpause = "action:owner_unpause"

// --- Surrender actions (Story 8.2) ---
const ActionSurrenderRequest = "action:surrender_request"
const ActionSurrenderAccept = "action:surrender_accept"
const ActionSurrenderDecline = "action:surrender_decline"

// --- Game state events (server -> client) ---
const EventMatchState = "event:match_state"
const EventCardPlayed = "event:card_played"
const EventTrickResolved = "event:trick_resolved"
const EventHandScored = "event:hand_scored"
const EventMatchEnd = "event:match_end"
const EventTrumpSelected = "event:trump_selected"
const EventDeclarationsResolved = "event:declarations_resolved"
const EventPlayerDeclared = "event:player_declared"
const EventBelotAnnounced = "event:belot_announced"
const EventMatchPaused = "event:match_paused"
const EventMatchResumed = "event:match_resumed"
const EventAutoAction = "event:auto_action"

// --- Game event payload structs ---

// CardPlayedPayload is the typed payload for EventCardPlayed events.
type CardPlayedPayload struct {
	PlayerSeat int    `json:"playerSeat"`
	CardID     string `json:"cardId"`
	AutoPlayed bool   `json:"autoPlayed"`
}

// TrumpSelectedPayload is the typed payload for EventTrumpSelected events.
// CardID carries the originally face-up trumpCandidate the picker absorbed —
// the post-pick GameState clears trumpCandidate to nil, so this event is the
// only place clients can read it.
type TrumpSelectedPayload struct {
	PlayerSeat int    `json:"playerSeat"`
	TrumpSuit  string `json:"trumpSuit"`
	CardID     string `json:"cardId"`
}

// PlayerDeclaredPayload is the typed payload for EventPlayerDeclared events.
// Emitted the moment a player commits a declare action during trick 1, so the
// table sees WHO holds declarations right away. Deliberately carries only the
// seat — type, value, and cards stay secret until EventDeclarationsResolved
// reveals the winning team's melds at the end of trick 1.
type PlayerDeclaredPayload struct {
	PlayerSeat int `json:"playerSeat"`
}

// BelotAnnouncedPayload is the typed payload for EventBelotAnnounced events.
// CardID is the K/Q of trump that was just played and triggered the announcement.
type BelotAnnouncedPayload struct {
	PlayerSeat int    `json:"playerSeat"`
	Team       int    `json:"team"`
	CardID     string `json:"cardId"`
}

// MatchPausedPayload is the typed payload for EventMatchPaused events.
type MatchPausedPayload struct {
	PausedBy      int     `json:"pausedBy"`
	PausedPlayers [4]bool `json:"pausedPlayers"`
}

// MatchResumedPayload is the typed payload for EventMatchResumed events.
type MatchResumedPayload struct {
	ResumedBy     int  `json:"resumedBy"`
	OwnerOverride bool `json:"ownerOverride"`
}

// AutoActionType enumerates the non-card auto-actions emitted on per-move
// timer expiry. Card auto-play does NOT produce an EventAutoAction — it
// continues to use the AutoPlayed flag on EventCardPlayed.
type AutoActionType string

const (
	AutoActionPassTrump   AutoActionType = "pass_trump"
	AutoActionSkipDeclare AutoActionType = "skip_declare"
	AutoActionSkipBelot   AutoActionType = "skip_belot"
)

// AutoActionPayload is the typed payload for EventAutoAction events.
// Informational one-shot — clients use it to surface a toast naming the
// timed-out player. Authoritative state still rides EventMatchState.
type AutoActionPayload struct {
	PlayerSeat int            `json:"playerSeat"`
	Type       AutoActionType `json:"type"`
}

// --- Disconnect/reconnect events (server -> client) ---
const EventPlayerDisconnected = "event:player_disconnected"
const EventPlayerReconnected = "event:player_reconnected"
const EventMatchAbandoned = "event:match_abandoned"

// PlayerDisconnectedPayload is the typed payload for EventPlayerDisconnected events.
type PlayerDisconnectedPayload struct {
	PlayerSeat         int    `json:"playerSeat"`
	Username           string `json:"username"`
	ReconnectExpiresAt string `json:"reconnectExpiresAt"` // ISO 8601 timestamp
}

// PlayerReconnectedPayload is the typed payload for EventPlayerReconnected events.
type PlayerReconnectedPayload struct {
	PlayerSeat int `json:"playerSeat"`
}

// MatchAbandonedPayload is the typed payload for EventMatchAbandoned events.
type MatchAbandonedPayload struct {
	AbandonedByPlayer int `json:"abandonedByPlayer"`
	TeamAFinalScore   int `json:"teamAFinalScore"`
	TeamBFinalScore   int `json:"teamBFinalScore"`
	MatchDurationSec  int `json:"matchDurationSec"`
}

// OutcomeReason is the typed outcome discriminator carried in event:match_end.
// Epic 9 will add OutcomeReasonInsolvency and OutcomeReasonHonorEject.
type OutcomeReason string

const (
	OutcomeReasonNatural     OutcomeReason = "natural"
	OutcomeReasonSurrender   OutcomeReason = "surrender"
	OutcomeReasonTimeout     OutcomeReason = "timeout"
	OutcomeReasonAbandonment OutcomeReason = "abandonment"
)

// MatchEndPayload is the typed payload for EventMatchEnd events.
// OutcomeReason is always present on the wire ("natural" | "surrender" | "timeout" | "abandonment").
// SurrenderedBySeat carries a seat index (0..3) only when OutcomeReason == "surrender".
// The name disambiguates this seat-index field from the persistence column
// `match.SurrenderedBy` which holds a userID (uint).
type MatchEndPayload struct {
	WinnerTeam        int           `json:"winnerTeam"`
	TeamAFinalScore   int           `json:"teamAFinalScore"`
	TeamBFinalScore   int           `json:"teamBFinalScore"`
	MatchDurationSec  int           `json:"matchDurationSec"`
	OutcomeReason     OutcomeReason `json:"outcomeReason,omitempty"`     // "natural" | "surrender" | "timeout" | "abandonment"
	SurrenderedBySeat *int          `json:"surrenderedBySeat,omitempty"` // seat index, only when outcomeReason == "surrender"
}

// --- Surrender events (Story 8.2) ---
const EventSurrenderProposed = "event:surrender_proposed"
const EventSurrenderDeclined = "event:surrender_declined"

// SurrenderProposedPayload is the typed payload for EventSurrenderProposed events.
// Sent to all four player WS connections when a player initiates surrender.
type SurrenderProposedPayload struct {
	ProposerSeat     int    `json:"proposerSeat"`
	ProposerTeam     int    `json:"proposerTeam"`
	ProposerUsername string `json:"proposerUsername"`
	PartnerSeat      int    `json:"partnerSeat"`
}

// SurrenderDeclinedPayload is the typed payload for EventSurrenderDeclined events.
// Sent to all four player WS connections when the partner declines.
type SurrenderDeclinedPayload struct {
	ProposerSeat  int `json:"proposerSeat"`
	DecliningSeat int `json:"decliningSeat"`
}

// --- Game error events (server -> client) ---
const ErrorInvalidAction = "error:invalid_action"
const ErrorNotYourTurn = "error:not_your_turn"
const ErrorWrongPhase = "error:wrong_phase"
const ErrorIllegalPlay = "error:illegal_play"
const ErrorPauseExhausted = "error:pause_exhausted"
const ErrorNoActivePause = "error:no_active_pause"
const ErrorNotRoomOwner = "error:not_room_owner"
const ErrorPlayerDisconnected = "error:player_disconnected"
const ErrorSurrenderExhausted = "error:surrender_exhausted"

// ErrorMatchStartFailed is broadcast to the four would-be participants of an
// auto-start that reached a "ready to start" state in the room transaction
// but whose subsequent matchStarter.StartMatch call returned an error. Story
// 8.5-1 AC2 — instead of stranding the room in a permanent "playing" state
// with no live session, the room is reverted to "waiting" and clients are
// told to stay on the room-lobby page rather than navigate to /game/{id}.
const ErrorMatchStartFailed = "error:match_start_failed"

// --- Room events ---
const SystemRoomCreated = "system:room_created"
const SystemRoomUpdated = "system:room_updated"

// --- Room player events ---
const SystemPlayerJoined = "system:player_joined"
const SystemPlayerLeft = "system:player_left"
const SystemRoomKicked = "system:room_kicked"
const SystemRoomOwnerChanged = "system:room_owner_changed"

// RoomKickedPayload is the typed payload for SystemRoomKicked events,
// sent only to the kicked user's WebSocket connection.
type RoomKickedPayload struct {
	RoomID uint   `json:"roomId"`
	Reason string `json:"reason"`
}

// --- Seat and game events ---
const SystemSeatUpdated = "system:seat_updated"
const SystemMatchStarted = "system:match_started"

// --- Bot seating events (Story 10.3) ---
// Payloads: bot_added {roomId, seat, team}; bot_removed {roomId, seat}.
// Bot identity is seat-derived and rendered client-side, so no name rides
// the wire.
const SystemBotAdded = "system:bot_added"
const SystemBotRemoved = "system:bot_removed"

// --- Chat events ---
const ActionChatMessage = "action:chat_message"
const SystemChatMessage = "system:chat_message"

// --- Emote events (Story 8.3) ---
const ActionEmote = "action:emote"
const SystemEmote = "system:emote"

// EmoteID is the canonical identifier for a preset in-game emote.
// Wire format is the snake_case string; the typed constants below are the
// single source of truth for the whitelist.
type EmoteID string

const (
	EmoteThumbsUp EmoteID = "thumbs_up"
	EmoteClap     EmoteID = "clap"
	EmoteLaugh    EmoteID = "laugh"
	EmoteThinking EmoteID = "thinking"
	EmoteFacepalm EmoteID = "facepalm"
	EmoteHeart    EmoteID = "heart"
)

// ValidEmoteIDs is the O(1) whitelist used by the emote handler to reject
// unknown IDs. Mirrors the EMOTE_IDS array on the client; both sides must
// stay in sync (project-rule: WS contract files updated in the same commit).
var ValidEmoteIDs = map[EmoteID]struct{}{
	EmoteThumbsUp: {},
	EmoteClap:     {},
	EmoteLaugh:    {},
	EmoteThinking: {},
	EmoteFacepalm: {},
	EmoteHeart:    {},
}

// EmoteRequest is the typed payload for ActionEmote (client → server).
// Emote is decoded as a plain string and validated against ValidEmoteIDs by
// the handler — keeping the request loose lets the handler log the raw value
// on rejection without needing to round-trip through EmoteID.
type EmoteRequest struct {
	Emote string `json:"emote"`
}

// EmotePayload is the typed payload for SystemEmote (server → client).
// PlayerSeat is the sender's seat (0..3) resolved by the server; the
// receiving client looks up the username from gameState.players, so this
// payload deliberately omits username (different from ChatMessagePayload).
type EmotePayload struct {
	PlayerSeat int     `json:"playerSeat"`
	Emote      EmoteID `json:"emote"`
}

// ChatMessageRequest is the typed payload for ActionChatMessage (client → server).
type ChatMessageRequest struct {
	Channel string `json:"channel"`          // "lobby" | "match" | "room"
	RoomID  *uint  `json:"roomId,omitempty"` // required when channel == "match" or "room"
	Text    string `json:"text"`
}

// ChatMessagePayload is the typed payload for SystemChatMessage (server → client).
type ChatMessagePayload struct {
	UserID    uint   `json:"userId"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"` // ISO 8601 (RFC3339) UTC
	Scope     string `json:"scope"`     // "lobby" | "match" | "room"
}

// --- General error events ---
const SystemError = "system:error"
const ErrorUnknownEvent = "error:unknown_event"
