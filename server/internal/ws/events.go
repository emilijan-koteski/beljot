package ws

const ActionAuthenticate = "action:authenticate"
const SystemAuthenticated = "system:authenticated"
const ErrorAuthFailed = "error:auth_failed"

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

const ActionSurrenderRequest = "action:surrender_request"
const ActionSurrenderAccept = "action:surrender_accept"
const ActionSurrenderDecline = "action:surrender_decline"

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

const EventCoinSettlement = "event:coin_settlement"

// CoinSettlementPayload is sent per human at match end with their net coin delta and new balance.
// It is sent per user because NewBalance differs per player.
type CoinSettlementPayload struct {
	CoinDelta  int `json:"coinDelta"`
	NewBalance int `json:"newBalance"`
	Pot        int `json:"pot"`
}

const EventXPAwarded = "event:xp_awarded"

// XPAwardedPayload is sent per human at match end with their XP earned, new total and derived level.
// The level is server-authoritative; LeveledUp drives the client's level-up flourish.
type XPAwardedPayload struct {
	XPEarned   int  `json:"xpEarned"`
	NewTotalXP int  `json:"newTotalXp"`
	NewLevel   int  `json:"newLevel"`
	LeveledUp  bool `json:"leveledUp"`
}

const EventHonorUpdated = "event:honor_updated"

// HonorUpdatedPayload is sent per human at match end with their authoritative honor score and raw lifetime totals.
// HonorTier is a machine token the client maps to a label; IsNewPlayer is a presentation hint only.
type HonorUpdatedPayload struct {
	HonorScore          int    `json:"honorScore"`
	HonorTier           string `json:"honorTier"`
	HonorCompletedTotal int64  `json:"honorCompletedTotal"`
	HonorAbandonedTotal int64  `json:"honorAbandonedTotal"`
	IsNewPlayer         bool   `json:"isNewPlayer"`
}

const EventSeasonPointsAwarded = "event:season_points_awarded"

// SeasonPointsAwardedPayload is sent per human at match end with the applied, signed SP change and the new rank.
// Match-end events go match_end, coin_settlement, xp_awarded, honor_updated, season_points_awarded, match_state.
type SeasonPointsAwardedPayload struct {
	SPChange     int    `json:"spChange"`
	NewSeasonSP  int    `json:"newSeasonSp"`
	RankTier     string `json:"rankTier"`
	RankDivision *int   `json:"rankDivision"`
	RankChange   string `json:"rankChange"`
	Reason       string `json:"reason"`
	SeasonName   string `json:"seasonName"`
}

const (
	RankChangePromoted = "promoted"
	RankChangeDemoted  = "demoted"
	RankChangeNone     = "none"
)

const (
	SPReasonNormal           = "normal"
	SPReasonAbandoned        = "abandoned"
	SPReasonPartnerAbandoned = "partner_abandoned"
)

// CardPlayedPayload is the typed payload for EventCardPlayed events.
type CardPlayedPayload struct {
	PlayerSeat int    `json:"playerSeat"`
	CardID     string `json:"cardId"`
	AutoPlayed bool   `json:"autoPlayed"`
}

// TrumpSelectedPayload is the typed payload for EventTrumpSelected events.
// CardID is the face-up trump candidate the picker absorbed, which the post-pick state no longer carries.
type TrumpSelectedPayload struct {
	PlayerSeat int    `json:"playerSeat"`
	TrumpSuit  string `json:"trumpSuit"`
	CardID     string `json:"cardId"`
}

// PlayerDeclaredPayload announces that a seat declared during trick 1.
// It carries only the seat; the melds stay secret until EventDeclarationsResolved.
type PlayerDeclaredPayload struct {
	PlayerSeat int `json:"playerSeat"`
}

// DeclarationPayload is one revealed meld inside a DeclarationsResolvedPayload, tagged with its own seat.
type DeclarationPayload struct {
	PlayerSeat int      `json:"playerSeat"`
	Type       string   `json:"type"`
	Value      int      `json:"value"`
	Cards      []string `json:"cards"`
}

// DeclarationsResolvedPayload carries only the winning team's melds; WinnerTeam is nil when no seat declared.
// Contested reports whether both teams declared, which the client cannot infer from the winner's melds alone.
type DeclarationsResolvedPayload struct {
	WinnerTeam   *int                 `json:"winnerTeam"`
	Contested    bool                 `json:"contested"`
	Declarations []DeclarationPayload `json:"declarations"`
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

// AutoActionType enumerates the non-card auto-actions emitted on per-move timer expiry.
// Card auto-play uses the AutoPlayed flag on EventCardPlayed instead.
type AutoActionType string

const (
	AutoActionPassTrump   AutoActionType = "pass_trump"
	AutoActionSkipDeclare AutoActionType = "skip_declare"
	AutoActionSkipBelot   AutoActionType = "skip_belot"
	AutoActionPickTrump   AutoActionType = "pick_trump"
)

// AutoActionPayload is the typed payload for EventAutoAction, used by clients to toast the timed-out player.
type AutoActionPayload struct {
	PlayerSeat int            `json:"playerSeat"`
	Type       AutoActionType `json:"type"`
}

const EventPlayerDisconnected = "event:player_disconnected"
const EventPlayerReconnected = "event:player_reconnected"
const EventMatchAbandoned = "event:match_abandoned"

// PlayerDisconnectedPayload is the typed payload for EventPlayerDisconnected events.
type PlayerDisconnectedPayload struct {
	PlayerSeat         int    `json:"playerSeat"`
	Username           string `json:"username"`
	ReconnectExpiresAt string `json:"reconnectExpiresAt"`
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
type OutcomeReason string

const (
	OutcomeReasonNatural       OutcomeReason = "natural"
	OutcomeReasonSurrender     OutcomeReason = "surrender"
	OutcomeReasonTimeout       OutcomeReason = "timeout"
	OutcomeReasonAbandonment   OutcomeReason = "abandonment"
	OutcomeReasonTargetReached OutcomeReason = "target_reached"
)

// MatchEndPayload is the typed payload for EventMatchEnd events.
// SurrenderedBySeat is a seat index (0..3), set only when OutcomeReason is "surrender".
type MatchEndPayload struct {
	WinnerTeam        int           `json:"winnerTeam"`
	TeamAFinalScore   int           `json:"teamAFinalScore"`
	TeamBFinalScore   int           `json:"teamBFinalScore"`
	MatchDurationSec  int           `json:"matchDurationSec"`
	OutcomeReason     OutcomeReason `json:"outcomeReason,omitempty"`
	SurrenderedBySeat *int          `json:"surrenderedBySeat,omitempty"`
}

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

const ErrorInvalidAction = "error:invalid_action"
const ErrorNotYourTurn = "error:not_your_turn"
const ErrorWrongPhase = "error:wrong_phase"
const ErrorIllegalPlay = "error:illegal_play"
const ErrorPauseExhausted = "error:pause_exhausted"
const ErrorNoActivePause = "error:no_active_pause"
const ErrorNotRoomOwner = "error:not_room_owner"
const ErrorPlayerDisconnected = "error:player_disconnected"
const ErrorSurrenderExhausted = "error:surrender_exhausted"

const ErrorMustPickTrump = "error:must_pick_trump"

const ErrorMatchStartFailed = "error:match_start_failed"

const SystemRoomCreated = "system:room_created"
const SystemRoomUpdated = "system:room_updated"

const SystemPlayerJoined = "system:player_joined"
const SystemPlayerLeft = "system:player_left"
const SystemRoomKicked = "system:room_kicked"
const SystemRoomOwnerChanged = "system:room_owner_changed"

const SystemRoomClosedInsolvent = "system:room_closed_insolvent"

const SystemInsolventEjected = "system:insolvent_ejected"

// RoomKickedPayload is the typed payload for SystemRoomKicked events,
// sent only to the kicked user's WebSocket connection.
type RoomKickedPayload struct {
	RoomID uint   `json:"roomId"`
	Reason string `json:"reason"`
}

// RoomClosedInsolventPayload is the typed payload for SystemRoomClosedInsolvent events.
type RoomClosedInsolventPayload struct {
	RoomID uint `json:"roomId"`
}

// InsolventEjectedPayload is the typed payload for SystemInsolventEjected events.
// Balance and BuyIn drive the client modal's exact numbers.
type InsolventEjectedPayload struct {
	RoomID  uint `json:"roomId"`
	BuyIn   int  `json:"buyIn"`
	Balance int  `json:"balance"`
}

const SystemHonorEjected = "system:honor_ejected"

// HonorEjectedPayload is the typed payload for SystemHonorEjected events.
// Honor is the player's authoritative recomputed score, shown beside the room's MinHonor.
type HonorEjectedPayload struct {
	RoomID   uint `json:"roomId"`
	MinHonor int  `json:"minHonor"`
	Honor    int  `json:"honor"`
}

const SystemPlayerReturned = "system:player_returned"

// PlayerReturnedPayload is the typed payload for SystemPlayerReturned events.
type PlayerReturnedPayload struct {
	RoomID uint `json:"roomId"`
	UserID uint `json:"userId"`
}

const SystemSeatUpdated = "system:seat_updated"
const SystemMatchStarted = "system:match_started"

const SystemBotAdded = "system:bot_added"
const SystemBotRemoved = "system:bot_removed"

const ActionChatMessage = "action:chat_message"
const SystemChatMessage = "system:chat_message"

const ActionEmote = "action:emote"
const SystemEmote = "system:emote"

// EmoteID is the canonical identifier for a preset in-game emote.
type EmoteID string

const (
	EmoteThumbsUp EmoteID = "thumbs_up"
	EmoteClap     EmoteID = "clap"
	EmoteLaugh    EmoteID = "laugh"
	EmoteThinking EmoteID = "thinking"
	EmoteFacepalm EmoteID = "facepalm"
	EmoteHeart    EmoteID = "heart"
)

var ValidEmoteIDs = map[EmoteID]struct{}{
	EmoteThumbsUp: {},
	EmoteClap:     {},
	EmoteLaugh:    {},
	EmoteThinking: {},
	EmoteFacepalm: {},
	EmoteHeart:    {},
}

// EmoteRequest is the typed payload for ActionEmote (client → server).
// Emote stays a plain string so the handler can log a rejected raw value.
type EmoteRequest struct {
	Emote string `json:"emote"`
}

// EmotePayload is the typed payload for SystemEmote (server → client).
// It omits the username because clients resolve it from the seat.
type EmotePayload struct {
	PlayerSeat int     `json:"playerSeat"`
	Emote      EmoteID `json:"emote"`
}

// ChatMessageRequest is the typed payload for ActionChatMessage (client → server).
type ChatMessageRequest struct {
	Channel string `json:"channel"`
	RoomID  *uint  `json:"roomId,omitempty"`
	Text    string `json:"text"`
}

// ChatMessagePayload is the typed payload for SystemChatMessage (server → client).
type ChatMessagePayload struct {
	UserID    uint   `json:"userId"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Scope     string `json:"scope"`
}

const SystemFriendRequest = "system:friend_request"

// FriendRequestPayload is the typed payload for SystemFriendRequest events.
// FromUsername lets the recipient's client render a toast without a lookup.
type FriendRequestPayload struct {
	RequestID    uint   `json:"requestId"`
	FromUserID   uint   `json:"fromUserId"`
	FromUsername string `json:"fromUsername"`
}

const ActionWhisper = "action:whisper"
const SystemWhisper = "system:whisper"

const ErrorNotFriends = "error:not_friends"
const ErrorWhisperBlockedInGame = "error:whisper_blocked_in_game"
const ErrorWhisperRecipientOffline = "error:whisper_recipient_offline"

// WhisperRequest is the typed payload for ActionWhisper (client → server).
type WhisperRequest struct {
	ToUsername string `json:"toUsername"`
	Text       string `json:"text"`
}

// WhisperPayload is the typed payload for SystemWhisper, delivered to both the recipient and the sender.
// The client keys the thread by whichever participant is not the local user.
type WhisperPayload struct {
	FromUserID   uint   `json:"fromUserId"`
	FromUsername string `json:"fromUsername"`
	ToUserID     uint   `json:"toUserId"`
	ToUsername   string `json:"toUsername"`
	Message      string `json:"message"`
	Timestamp    string `json:"timestamp"`
}

const SystemRoomInvite = "system:room_invite"

// RoomInvitePayload is the typed payload for SystemRoomInvite events.
// IsHostInvite only drives the UI; the real password-gate grant lives server-side in the InviteRegistry.
type RoomInvitePayload struct {
	InviteID        uint64 `json:"inviteId"`
	RoomID          uint   `json:"roomId"`
	RoomName        string `json:"roomName"`
	InviterUserID   uint   `json:"inviterUserId"`
	InviterUsername string `json:"inviterUsername"`
	CoinBuyIn       int    `json:"coinBuyIn"`
	IsPrivate       bool   `json:"isPrivate"`
	IsHostInvite    bool   `json:"isHostInvite"`
	MinHonor        int    `json:"minHonor"`
	ExpiresAt       string `json:"expiresAt"`
}

const SystemError = "system:error"
const ErrorUnknownEvent = "error:unknown_event"
