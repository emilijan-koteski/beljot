// WebSocket event contract — keep in sync with server/internal/ws/events.go

import type { RoomPlayer } from "@/shared/types/apiTypes";

// Event type prefixes
// action: — client -> server
// event:  — server -> client (game state)
// error:  — server -> client (errors)
// system: — server -> client (platform events)

export interface WsMessage<T = unknown> {
  type: string;
  payload: T;
  /**
   * Server wall clock at send time (RFC3339). Stamped on match events whose
   * payloads carry absolute deadlines (turnExpiresAt, reconnectExpiresAt) so
   * the client can estimate its clock offset — see `shared/lib/clockSync.ts`.
   * Absent on client→server messages and on senders without deadlines.
   */
  serverNow?: string;
}

// --- Authentication events ---
export const ACTION_AUTHENTICATE = "action:authenticate" as const;
export const SYSTEM_AUTHENTICATED = "system:authenticated" as const;
export const ERROR_AUTH_FAILED = "error:auth_failed" as const;

export interface AuthenticatePayload {
  token: string;
}

export interface AuthenticatedPayload {
  userId: number;
}

export interface AuthFailedPayload {
  message: string;
}

// --- Game action events (client -> server) ---
export const ACTION_PLAY_CARD = "action:play_card" as const;
export const ACTION_PICK_TRUMP = "action:pick_trump" as const;
export const ACTION_PASS_TRUMP = "action:pass_trump" as const;
export const ACTION_DECLARE = "action:declare" as const;
export const ACTION_SKIP_DECLARE = "action:skip_declare" as const;
export const ACTION_ANNOUNCE_BELOT = "action:announce_belot" as const;
export const ACTION_DECLINE_BELOT = "action:decline_belot" as const;
// Acknowledges the hand-complete pause; the server deals the next hand once
// every connected player has continued (or the auto-continue timeout fires).
export const ACTION_CONTINUE = "action:continue" as const;
export const ACTION_PAUSE = "action:pause" as const;
export const ACTION_UNPAUSE = "action:unpause" as const;
export const ACTION_OWNER_UNPAUSE = "action:owner_unpause" as const;

// --- Surrender actions (Story 8.2) ---
export const ACTION_SURRENDER_REQUEST = "action:surrender_request" as const;
export const ACTION_SURRENDER_ACCEPT = "action:surrender_accept" as const;
export const ACTION_SURRENDER_DECLINE = "action:surrender_decline" as const;

export type SurrenderRequestPayload = Record<string, never>;
export type SurrenderAcceptPayload = Record<string, never>;
export type SurrenderDeclinePayload = Record<string, never>;

export interface PlayCardPayload {
  cardId: string;
}

export interface PickTrumpPayload {
  suit?: "S" | "H" | "D" | "C"; // Required in round 2 (free suit selection); omit in round 1
}

export type PassTrumpPayload = Record<string, never>;

export type DeclarePayload = Record<string, never>;

export type SkipDeclarePayload = Record<string, never>;

export type AnnounceBelotPayload = Record<string, never>;

export type DeclineBelotPayload = Record<string, never>;

// --- Game state events (server -> client) ---
export const EVENT_MATCH_STATE = "event:match_state" as const;
export const EVENT_CARD_PLAYED = "event:card_played" as const;
export const EVENT_TRICK_RESOLVED = "event:trick_resolved" as const;
export const EVENT_HAND_SCORED = "event:hand_scored" as const;
export const EVENT_MATCH_END = "event:match_end" as const;
export const EVENT_TRUMP_SELECTED = "event:trump_selected" as const;
export const EVENT_DECLARATIONS_RESOLVED = "event:declarations_resolved" as const;
export const EVENT_PLAYER_DECLARED = "event:player_declared" as const;
export const EVENT_BELOT_ANNOUNCED = "event:belot_announced" as const;
export const EVENT_MATCH_PAUSED = "event:match_paused" as const;
export const EVENT_MATCH_RESUMED = "event:match_resumed" as const;
export const EVENT_AUTO_ACTION = "event:auto_action" as const;
// Per-seat only — the server sends this to ONE user, never broadcast. See
// FaceDownRevealedPayload.
export const EVENT_FACE_DOWN_REVEALED = "event:face_down_revealed" as const;

// Match state payload types will be expanded in Story 4.2 when the session manager
// defines the exact shape of match state broadcasts. For now, typed as unknown.
export interface MatchStatePayload {
  [key: string]: unknown;
}

export interface CardPlayedPayload {
  playerSeat: number;
  cardId: string;
  autoPlayed: boolean;
}

export interface TrickResolvedPayload {
  winnerSeat: number;
  winnerTeam: number;
  cards: string[];
}

export interface HandScoredPayload {
  teamACardPoints: number;
  teamBCardPoints: number;
  teamADeclPoints: number;
  teamBDeclPoints: number;
  lastTrickTeam: number;
  lastTrickBonus: number;
  capot: boolean;
  capotTeam: number | null;
  capotBonus: number;
  failedContract: boolean;
  contractingTeam: number;
  teamAHandTotal: number;
  teamBHandTotal: number;
  teamAMatchScore: number;
  teamBMatchScore: number;
}

export interface MatchEndPayload {
  winnerTeam: number;
  teamAFinalScore: number;
  teamBFinalScore: number;
  matchDurationSec: number;
  // Optional fields added by Story 8.2 — natural-end matches omit both via
  // server-side omitempty so existing readers continue to work.
  // surrenderedBySeat is a seat index (0..3); the persistence column
  // match.SurrenderedBy holds a userID — distinct fields, distinct names.
  outcomeReason?: "surrender" | "timeout" | "abandonment" | "natural";
  surrenderedBySeat?: number;
}

export interface TrumpSelectedPayload {
  playerSeat: number;
  trumpSuit: string;
  // Originally face-up trump candidate the picker absorbed. The post-pick
  // MatchState clears trumpCandidate, so this event is the only carrier.
  //
  // EMPTY STRING when the variant has no trump candidate: trump was a bare
  // named suit and the taker drew no card. The reveal renders candidate-less
  // in that case rather than being suppressed.
  cardId: string;
}

/**
 * A seat's own two face-down cards, turned up for that seat alone.
 *
 * Sent per-user via SendToUser and never broadcast: under the
 * all-before-bidding deal each seat physically holds eight cards but only six
 * are open, and when bidding round 1 is passed out the other two turn up for
 * their owner only. They deliberately never ride `event:match_state` — that
 * payload is serialized once and the identical bytes go to all four seats, so a
 * card visible to exactly one player cannot travel on it.
 *
 * `playerSeat` is always the recipient's OWN seat; a different seat here means a
 * server bug, not another player's cards. Re-sent on reconnect (the server
 * replays it from `SyncStateOnConnect`) because it is a one-shot event.
 */
export interface FaceDownRevealedPayload {
  playerSeat: number;
  cardIds: string[];
}

export interface DeclarationsResolvedPayload {
  winnerTeam: number | null;
  // True when BOTH teams put a meld on the table, i.e. a comparison actually
  // decided the winner. Only the winning team's melds are on the wire, so the
  // client cannot derive this: "we out-declared them" and "we were the only
  // team to declare" look identical, and under a declaration-overlap config one
  // seat holding two melds is ordinary rather than evidence of a contest.
  contested: boolean;
  declarations: Array<{
    playerSeat: number;
    type: string;
    value: number;
    cards: string[];
  }>;
}

// Fired the moment a player commits a declare — drives the seat-anchored
// "has a declaration" banner. Seat only: meld type/value/cards stay secret
// until event:declarations_resolved. When declarations are collected depends on
// the server's rules (inside trick 1, or in a dedicated phase before it); this
// event's meaning does not.
export interface PlayerDeclaredPayload {
  playerSeat: number;
}

export interface BelotAnnouncedPayload {
  playerSeat: number;
  team: number;
  cardId: string;
}

export interface MatchPausedPayload {
  pausedBy: number;
  pausedPlayers: [boolean, boolean, boolean, boolean];
}

export interface MatchResumedPayload {
  resumedBy: number;
  ownerOverride: boolean;
}

// Non-card auto-action emitted on per-move timer expiry. Card auto-play uses
// the autoPlayed flag on event:card_played and is not represented here.
//
// "pick_trump" (Story 12.8) is the odd one out: the other three DECLINE
// something, while this one names trump for a seat that had no legal pass — the
// dealer bidding last in round 2 of a variant where the hand must find a taker.
export type AutoActionType = "pass_trump" | "skip_declare" | "skip_belot" | "pick_trump";

export interface AutoActionPayload {
  playerSeat: number;
  type: AutoActionType;
}

// --- Economy events (Story 9.2) ---
// Sent per-human at match end (after event:match_end) with that player's own
// net coin delta and resulting wallet balance. Per-user, not broadcast, because
// newBalance differs per player; pot is shared.
export const EVENT_COIN_SETTLEMENT = "event:coin_settlement" as const;

export interface CoinSettlementPayload {
  coinDelta: number;
  newBalance: number;
  pot: number;
}

// --- XP & progression events (Story 9.5) ---
// Sent per-human at match end, slotted after event:coin_settlement and before
// the trailing event:match_state. Carries that player's own XP earned this
// match, new lifetime total, derived level, and whether they leveled up. The
// level is server-authoritative (derived from totalXp) — never recomputed on the
// client for any decision. Keep in sync with server events.go (EventXPAwarded).
export const EVENT_XP_AWARDED = "event:xp_awarded" as const;

export interface XpAwardedPayload {
  xpEarned: number;
  newTotalXp: number;
  newLevel: number;
  leveledUp: boolean;
}

// --- Honor events (Story 9.7) ---
// Sent per-human at match end, slotted after event:xp_awarded and before the
// trailing event:match_state. Carries that player's own recomputed honor score
// and its tier bucket. The score and tier are server-authoritative — the client
// mirror in shared/lib/honor.ts is presentation only and never decides access.
//
// honorTier is a STABLE MACHINE TOKEN ("exemplary" | "trusted" | "fair" |
// "unreliable" | "problematic") that the client maps to an i18n label and
// colour; a display string never crosses the wire.
//
// isNewPlayer is a presentation hint: below the matches-played floor
// (completed + abandoned < 5 — it counts experience, not successes) the UI
// hides the score and tier behind a "New Player" chip, but honorScore and
// honorTier are still populated. Keep in sync with server events.go
// (EventHonorUpdated).
export const EVENT_HONOR_UPDATED = "event:honor_updated" as const;

export interface HonorUpdatedPayload {
  honorScore: number;
  honorTier: string;
  honorCompletedTotal: number;
  honorAbandonedTotal: number;
  isNewPlayer: boolean;
}

// --- Disconnect/reconnect events (server -> client) ---
export const EVENT_PLAYER_DISCONNECTED = "event:player_disconnected" as const;
export const EVENT_PLAYER_RECONNECTED = "event:player_reconnected" as const;
export const EVENT_MATCH_ABANDONED = "event:match_abandoned" as const;

export interface PlayerDisconnectedPayload {
  playerSeat: number;
  username: string;
  reconnectExpiresAt: string;
}

export interface PlayerReconnectedPayload {
  playerSeat: number;
}

export interface MatchAbandonedPayload {
  abandonedByPlayer: number;
  teamAFinalScore: number;
  teamBFinalScore: number;
  matchDurationSec: number;
}

// --- Surrender events (server -> client, Story 8.2) ---
export const EVENT_SURRENDER_PROPOSED = "event:surrender_proposed" as const;
export const EVENT_SURRENDER_DECLINED = "event:surrender_declined" as const;

export interface SurrenderProposedPayload {
  proposerSeat: number;
  proposerTeam: number;
  proposerUsername: string;
  partnerSeat: number;
}

export interface SurrenderDeclinedPayload {
  proposerSeat: number;
  decliningSeat: number;
}

// --- Game error events (server -> client) ---
export const ERROR_INVALID_ACTION = "error:invalid_action" as const;
export const ERROR_NOT_YOUR_TURN = "error:not_your_turn" as const;
export const ERROR_WRONG_PHASE = "error:wrong_phase" as const;
export const ERROR_ILLEGAL_PLAY = "error:illegal_play" as const;
export const ERROR_PAUSE_EXHAUSTED = "error:pause_exhausted" as const;
export const ERROR_NO_ACTIVE_PAUSE = "error:no_active_pause" as const;
export const ERROR_NOT_ROOM_OWNER = "error:not_room_owner" as const;
export const ERROR_PLAYER_DISCONNECTED = "error:player_disconnected" as const;
export const ERROR_SURRENDER_EXHAUSTED = "error:surrender_exhausted" as const;
// Sent when a seat passes but the rules give it no pass to make — the dealer
// bidding last under a "the hand must find a taker" config. Distinct from
// error:invalid_action so the toast can say "you must name a suit" rather
// than "invalid action"; the UI hides Pass in this state, so this only
// reaches a stale client or a hand-crafted frame.
export const ERROR_MUST_PICK_TRUMP = "error:must_pick_trump" as const;

// Story 8.5-1 AC2: broadcast to the four would-be participants of an auto-start
// whose matchStarter.StartMatch call returned an error. The room is reverted to
// "waiting" server-side; clients should surface a toast and stay on the
// room-lobby page rather than navigate to /match/{roomId}.
export const ERROR_MATCH_START_FAILED = "error:match_start_failed" as const;

export interface MatchErrorPayload {
  code: string;
  message: string;
}

// --- Room events ---
export const SYSTEM_ROOM_CREATED = "system:room_created" as const;
export const SYSTEM_ROOM_UPDATED = "system:room_updated" as const;

export interface RoomCreatedPayload {
  id: number;
  name: string;
  code: string;
  ownerId: number;
  ownerUsername: string;
  players: RoomPlayer[];
  variant: string;
  matchMode: string;
  timerStyle: string;
  timerDurationSeconds: number | null;
  status: string;
  playerCount: number;
  isQuickPlay: boolean;
  /** Derived privacy flag (Story 9.6) so the lobby card lock indicator stays live. */
  isPrivate: boolean;
  /**
   * Honor gate (Story 9.8), declared for the same reason as isPrivate above: the
   * server's roomLifecyclePayload sends both keys, and the lobby card's honor and
   * veterans-only chips read them, so the declared contract has to carry them or
   * the next author gets a tsc error on a field the server is actually sending.
   *
   * Caveat, recorded rather than hidden: the QuickPlay system:room_created map is
   * hand-built separately and omits these two (as it already omits isPrivate,
   * coinBuyIn and players). Harmless today — a synthesized quick-play room IS
   * ungated, so absent reads as ungated, which is correct — and tracked in
   * deferred-work.md, where the real fix is routing that map through
   * roomLifecyclePayload instead of maintaining a third key list.
   */
  minHonor: number;
  allowNewPlayers: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface RoomUpdatedPayload {
  id: number;
  name: string;
  code: string;
  ownerId: number;
  ownerUsername: string;
  players: RoomPlayer[];
  variant: string;
  matchMode: string;
  timerStyle: string;
  timerDurationSeconds: number | null;
  status: string;
  playerCount: number;
  isQuickPlay: boolean;
  /** Derived privacy flag (Story 9.6) so the lobby card lock indicator stays live. */
  isPrivate: boolean;
  /**
   * Honor gate (Story 9.8), declared for the same reason as isPrivate above: the
   * server's roomLifecyclePayload sends both keys, and the lobby card's honor and
   * veterans-only chips read them, so the declared contract has to carry them or
   * the next author gets a tsc error on a field the server is actually sending.
   *
   * Caveat, recorded rather than hidden: the QuickPlay system:room_created map is
   * hand-built separately and omits these two (as it already omits isPrivate,
   * coinBuyIn and players). Harmless today — a synthesized quick-play room IS
   * ungated, so absent reads as ungated, which is correct — and tracked in
   * deferred-work.md, where the real fix is routing that map through
   * roomLifecyclePayload instead of maintaining a third key list.
   */
  minHonor: number;
  allowNewPlayers: boolean;
  createdAt: string;
  updatedAt: string;
}

// --- Room player events ---
export const SYSTEM_PLAYER_JOINED = "system:player_joined" as const;
export const SYSTEM_PLAYER_LEFT = "system:player_left" as const;
export const SYSTEM_ROOM_KICKED = "system:room_kicked" as const;
// Broadcast to a reopened room's members when a player re-enters after a match
// (via "Return to room"). Drives the RoomPage "waiting to return" presence
// state. Keep in sync with server events.go (SystemPlayerReturned).
export const SYSTEM_PLAYER_RETURNED = "system:player_returned" as const;

export interface RoomKickedPayload {
  roomId: number;
  reason: string;
}

export interface PlayerReturnedPayload {
  roomId: number;
  userId: number;
}

export interface PlayerJoinedPayload {
  roomId: number;
  userId: number;
  username: string;
  playerCount: number;
  /**
   * The joiner's roster decoration, so already-seated viewers can draw the new
   * seat's level + shield without a refetch. Optional with the same semantics
   * as RoomPlayer: absent means "not read", a real 0 arrives as 0.
   */
  honorScore?: number;
  honorTier?: string;
  level?: number;
}

export interface PlayerLeftPayload {
  roomId: number;
  userId: number;
  username: string;
  playerCount: number;
  newOwnerId?: number;
}

// --- Seat and game events ---
export const SYSTEM_SEAT_UPDATED = "system:seat_updated" as const;
export const SYSTEM_MATCH_STARTED = "system:match_started" as const;

export const SYSTEM_ROOM_OWNER_CHANGED = "system:room_owner_changed" as const;

export interface RoomOwnerChangedPayload {
  roomId: number;
  newOwnerId: number;
  newOwnerUsername: string;
  previousOwnerId: number;
}

// Broadcast to every still-seated member when a room closes because no
// present-and-solvent player remained to own it (Story 9.3 AC4). Recipients
// route to the lobby with a "room closed" notice. Keep in sync with server
// events.go (SystemRoomClosedInsolvent).
//
// Story 9.8 REUSES this event for honor closes (an owner honor-ejected with no
// eligible heir). The payload and the reason-agnostic client copy already cover
// both, so the wire string is deliberately unchanged.
export const SYSTEM_ROOM_CLOSED_INSOLVENT = "system:room_closed_insolvent" as const;

export interface RoomClosedInsolventPayload {
  roomId: number;
}

// Per-user push to a player ejected at match start because they could not
// afford the buy-in at the authoritative charge (Story 9.3 AC5). Carries the
// exact numbers so the lobby modal can show balance vs buy-in. Keep in sync
// with server events.go (SystemInsolventEjected).
export const SYSTEM_INSOLVENT_EJECTED = "system:insolvent_ejected" as const;

export interface InsolventEjectedPayload {
  roomId: number;
  buyIn: number;
  balance: number;
}

// Per-user push to a player ejected from a room because their honor score no
// longer clears the room's gate (Story 9.8 AC6). Sibling of
// SYSTEM_INSOLVENT_EJECTED above: same shape, same delivery, same client
// pipeline (roomStore.roomEjection -> RoomEjectionModal). Keep in sync with
// server events.go (SystemHonorEjected).
//
// The system: prefix is deliberate — this is a pre-match, room-lifecycle push,
// not an in-match game-state event — and it means the event has ZERO WS
// drift-gate touchpoints: no Zod schema, no golden, no conformance witness, no
// contract-test row. Every system:* payload in this file is likewise outside the
// gate. Do not add any of those for it (Story 9.8 D4).
export const SYSTEM_HONOR_EJECTED = "system:honor_ejected" as const;

export interface HonorEjectedPayload {
  roomId: number;
  // The room's threshold, and the player's own authoritative recomputed score.
  // Both are real Go ints: a score of 0 is legitimate, so validate with
  // typeof === "number", never JS truthiness.
  minHonor: number;
  honor: number;
}

export interface SeatUpdatedPayload {
  roomId: number;
  userId: number;
  username: string;
  // seat/team are null when a player vacates their seat (LeaveSeat) but
  // remains in the room. previousSeat is null only for the initial seat
  // selection from an unseated state.
  seat: number | null;
  team: string | null;
  previousSeat: number | null;
}

// --- Bot seating events (Story 10.3) ---
// Bot identity is seat-derived and rendered client-side (localized "Bot N"),
// so no name rides the wire. Keep in sync with server events.go.
export const SYSTEM_BOT_ADDED = "system:bot_added" as const;
export const SYSTEM_BOT_REMOVED = "system:bot_removed" as const;

export interface BotAddedPayload {
  roomId: number;
  seat: number;
  team: string;
}

export interface BotRemovedPayload {
  roomId: number;
  seat: number;
}

export interface MatchStartedPayload {
  roomId: number;
}

// --- Chat events ---
export const ACTION_CHAT_MESSAGE = "action:chat_message" as const;
export const SYSTEM_CHAT_MESSAGE = "system:chat_message" as const;

// --- Emote events (Story 8.3) ---
export const ACTION_EMOTE = "action:emote" as const;
export const SYSTEM_EMOTE = "system:emote" as const;

// EmoteID — canonical wire-format identifier. String-literal union (not enum)
// per project rule against TypeScript `enum`. Mirrors the Go EmoteID type.
export type EmoteID = "thumbs_up" | "clap" | "laugh" | "thinking" | "facepalm" | "heart";

// EMOTE_IDS — single source of truth for picker iteration order and the
// dispatcher's whitelist check. Frozen so consumers cannot mutate it.
export const EMOTE_IDS: readonly EmoteID[] = Object.freeze([
  "thumbs_up",
  "clap",
  "laugh",
  "thinking",
  "facepalm",
  "heart",
] as const);

export interface EmoteRequest {
  emote: EmoteID;
}

export interface EmotePayload {
  playerSeat: number;
  emote: EmoteID;
}

export interface ChatMessageRequest {
  channel: "lobby" | "match" | "room";
  roomId?: number; // required when channel === "match" or channel === "room"
  text: string;
}

export interface ChatMessagePayload {
  userId: number;
  username: string;
  message: string;
  timestamp: string;
  scope: "lobby" | "match" | "room";
}

// --- Friend events (Story 11.2) ---
// Best-effort, online-only per-user push when someone sends the recipient a
// friend request. The durable path is GET /friends/requests on next load, so a
// missed push (offline recipient) is fine. The system: prefix keeps this OUTSIDE
// the WS drift gate — no Zod schema, no golden, no conformance witness, no
// contract-test row (like every other system:* payload here). Keep in sync with
// server events.go (SystemFriendRequest).
export const SYSTEM_FRIEND_REQUEST = "system:friend_request" as const;

export interface FriendRequestPayload {
  // All three are real Go values — validate with typeof === "number" / "string"
  // in the dispatcher, never JS truthiness (a real id/0 is legitimate).
  requestId: number;
  fromUserId: number;
  fromUsername: string;
}

// --- Whisper events (Story 11.4) ---
// ACTION_WHISPER (client→server) + SYSTEM_WHISPER (server→client) carry a private
// one-to-one message between two friends. They mirror the chat/emote pipeline
// exactly (action:* in, system:* out) — NOT event:whisper (Story 11.4 D1,
// PO-confirmed 2026-08-14). The system: prefix keeps whisper OUTSIDE the WS drift
// gate — no Zod schema, no golden, no conformance witness, no contract-test row,
// like every other system:* payload here. Keep in sync with server events.go
// (ActionWhisper / SystemWhisper / WhisperRequest / WhisperPayload).
export const ACTION_WHISPER = "action:whisper" as const;
export const SYSTEM_WHISPER = "system:whisper" as const;

// Whisper error events — sent to the SENDER only. Outside the drift gate.
export const ERROR_NOT_FRIENDS = "error:not_friends" as const;
export const ERROR_WHISPER_BLOCKED_IN_GAME = "error:whisper_blocked_in_game" as const;
export const ERROR_WHISPER_RECIPIENT_OFFLINE = "error:whisper_recipient_offline" as const;

export interface WhisperRequest {
  toUsername: string;
  text: string;
}

// The SAME payload is delivered to BOTH participants — recipient AND sender
// (own-echo). All numeric fields are real Go values (a userId of any value is
// legitimate), so validate with typeof === "number", never JS truthiness.
export interface WhisperPayload {
  fromUserId: number;
  fromUsername: string;
  toUserId: number;
  toUsername: string;
  message: string;
  timestamp: string;
}

// --- Room invite events (Story 11.5) ---
// Best-effort, online-only per-user push when a friend invites the recipient
// into a waiting room. There is no offline inbox and the invite carries a TTL,
// so a missed push is simply never actioned. The system: prefix is a deliberate
// deviation from the epic AC's literal event:room_invite (Story 11.5 D1,
// PO-confirmed 2026-08-14) and keeps the event OUTSIDE the WS drift gate — no
// Zod schema, no golden, no conformance witness, no contract-test row, like
// every other system:* payload here. Keep in sync with server events.go
// (SystemRoomInvite / RoomInvitePayload).
export const SYSTEM_ROOM_INVITE = "system:room_invite" as const;

export interface RoomInvitePayload {
  inviteId: number;
  roomId: number;
  roomName: string;
  inviterUserId: number;
  inviterUsername: string;
  // Real Go values: a coinBuyIn of 0 and isPrivate false are legitimate, so
  // validate with typeof === "number" / "boolean", never JS truthiness.
  coinBuyIn: number;
  isPrivate: boolean;
  // true when the inviter was the room OWNER — the server holds a one-time grant
  // that carries this invitee past the password gate, so the client shows NO
  // password prompt. It is a rendering hint only; the authority is server-side.
  isHostInvite: boolean;
  // The room's honor floor (0 = ungated). Carried so a rejected accept can render
  // the SPECIFIC "you need N honor" message, matching every other join path.
  minHonor: number;
  // Absolute ISO 8601 UTC timestamp, never a relative duration.
  expiresAt: string;
}

// --- General error events ---
export const SYSTEM_ERROR = "system:error" as const;
export const ERROR_UNKNOWN_EVENT = "error:unknown_event" as const;

export interface ErrorPayload {
  message: string;
}
