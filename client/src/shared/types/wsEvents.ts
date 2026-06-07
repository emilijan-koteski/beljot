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
export const EVENT_BELOT_ANNOUNCED = "event:belot_announced" as const;
export const EVENT_MATCH_PAUSED = "event:match_paused" as const;
export const EVENT_MATCH_RESUMED = "event:match_resumed" as const;
export const EVENT_AUTO_ACTION = "event:auto_action" as const;

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
  cardId: string;
}

export interface DeclarationsResolvedPayload {
  winnerTeam: number | null;
  declarations: Array<{
    playerSeat: number;
    type: string;
    value: number;
    cards: string[];
  }>;
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
export type AutoActionType = "pass_trump" | "skip_declare" | "skip_belot";

export interface AutoActionPayload {
  playerSeat: number;
  type: AutoActionType;
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
  createdAt: string;
  updatedAt: string;
}

// --- Room player events ---
export const SYSTEM_PLAYER_JOINED = "system:player_joined" as const;
export const SYSTEM_PLAYER_LEFT = "system:player_left" as const;
export const SYSTEM_ROOM_KICKED = "system:room_kicked" as const;

export interface RoomKickedPayload {
  roomId: number;
  reason: string;
}

export interface PlayerJoinedPayload {
  roomId: number;
  userId: number;
  username: string;
  playerCount: number;
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

// --- General error events ---
export const SYSTEM_ERROR = "system:error" as const;
export const ERROR_UNKNOWN_EVENT = "error:unknown_event" as const;

export interface ErrorPayload {
  message: string;
}
