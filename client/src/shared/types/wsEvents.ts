import type { RoomPlayer } from "@/shared/types/apiTypes";

/**
 * Envelope for every WebSocket frame; keep in sync with server/internal/ws/events.go. serverNow is
 * the server clock (RFC3339) stamped on events with deadlines, for clock-offset estimation.
 */
export interface WsMessage<T = unknown> {
  type: string;
  payload: T;
  serverNow?: string;
}

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

export const ACTION_PLAY_CARD = "action:play_card" as const;
export const ACTION_PICK_TRUMP = "action:pick_trump" as const;
export const ACTION_PASS_TRUMP = "action:pass_trump" as const;
export const ACTION_DECLARE = "action:declare" as const;
export const ACTION_SKIP_DECLARE = "action:skip_declare" as const;
export const ACTION_ANNOUNCE_BELOT = "action:announce_belot" as const;
export const ACTION_DECLINE_BELOT = "action:decline_belot" as const;
export const ACTION_CONTINUE = "action:continue" as const;
export const ACTION_PAUSE = "action:pause" as const;
export const ACTION_UNPAUSE = "action:unpause" as const;
export const ACTION_OWNER_UNPAUSE = "action:owner_unpause" as const;

export const ACTION_SURRENDER_REQUEST = "action:surrender_request" as const;
export const ACTION_SURRENDER_ACCEPT = "action:surrender_accept" as const;
export const ACTION_SURRENDER_DECLINE = "action:surrender_decline" as const;

export type SurrenderRequestPayload = Record<string, never>;
export type SurrenderAcceptPayload = Record<string, never>;
export type SurrenderDeclinePayload = Record<string, never>;

export interface PlayCardPayload {
  cardId: string;
}

/**
 * suit is omitted only for Bitola's round-1 candidate take, where trump binds to the candidate's
 * suit.
 */
export interface PickTrumpPayload {
  suit?: "S" | "H" | "D" | "C";
}

export type PassTrumpPayload = Record<string, never>;

export type DeclarePayload = Record<string, never>;

export type SkipDeclarePayload = Record<string, never>;

export type AnnounceBelotPayload = Record<string, never>;

export type DeclineBelotPayload = Record<string, never>;

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

/**
 * surrenderedBySeat is a seat index (0..3), not a userID. On "target_reached" the cut-short hand's
 * points are already inside the final scores.
 */
export interface MatchEndPayload {
  winnerTeam: number;
  teamAFinalScore: number;
  teamBFinalScore: number;
  matchDurationSec: number;
  outcomeReason?: "surrender" | "timeout" | "abandonment" | "natural" | "target_reached";
  surrenderedBySeat?: number;
}

/**
 * cardId is the face-up candidate the picker absorbed, or empty when the variant has no candidate.
 */
export interface TrumpSelectedPayload {
  playerSeat: number;
  trumpSuit: string;
  cardId: string;
}

/**
 * contested is true when both teams declared; the client cannot derive it because only the winner's
 * melds are sent.
 */
export interface DeclarationsResolvedPayload {
  winnerTeam: number | null;
  contested: boolean;
  declarations: Array<{
    playerSeat: number;
    type: string;
    value: number;
    cards: string[];
  }>;
}

/**
 * Announces that a seat committed a declaration; the meld stays secret until
 * event:declarations_resolved.
 */
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

/**
 * Non-card auto-action taken on move-timer expiry; card auto-play uses
 * CardPlayedPayload.autoPlayed. "pick_trump" names trump for a dealer who has no legal pass.
 */
export type AutoActionType = "pass_trump" | "skip_declare" | "skip_belot" | "pick_trump";

export interface AutoActionPayload {
  playerSeat: number;
  type: AutoActionType;
}

export const EVENT_COIN_SETTLEMENT = "event:coin_settlement" as const;

/** Per-player coin result sent at match end after event:match_end; pot is shared. */
export interface CoinSettlementPayload {
  coinDelta: number;
  newBalance: number;
  pot: number;
}

export const EVENT_XP_AWARDED = "event:xp_awarded" as const;

/** Per-player XP result sent at match end; the level is server-authoritative. */
export interface XpAwardedPayload {
  xpEarned: number;
  newTotalXp: number;
  newLevel: number;
  leveledUp: boolean;
}

export const EVENT_HONOR_UPDATED = "event:honor_updated" as const;

/**
 * Per-player honor result sent at match end; honorTier is a machine token mapped to an i18n label.
 * isNewPlayer only hides the score in the UI; honorScore and honorTier are still populated.
 */
export interface HonorUpdatedPayload {
  honorScore: number;
  honorTier: string;
  honorCompletedTotal: number;
  honorAbandonedTotal: number;
  isNewPlayer: boolean;
}

export const EVENT_SEASON_POINTS_AWARDED = "event:season_points_awarded" as const;

export const SEASON_RANK_CHANGES = ["promoted", "demoted", "none"] as const;
export type SeasonRankChange = (typeof SEASON_RANK_CHANGES)[number];

export const SEASON_SP_REASONS = ["normal", "abandoned", "partner_abandoned"] as const;
export type SeasonSpReason = (typeof SEASON_SP_REASONS)[number];

/**
 * Per-player season result sent at match end; spChange is signed and can be a real 0, so never test
 * truthiness. rankTier is a machine token, rankDivision is null for Master and Grandmaster, and
 * seasonName renders verbatim.
 */
export interface SeasonPointsAwardedPayload {
  spChange: number;
  newSeasonSp: number;
  rankTier: string;
  rankDivision: number | null;
  rankChange: SeasonRankChange;
  reason: SeasonSpReason;
  seasonName: string;
}

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

export const ERROR_INVALID_ACTION = "error:invalid_action" as const;
export const ERROR_NOT_YOUR_TURN = "error:not_your_turn" as const;
export const ERROR_WRONG_PHASE = "error:wrong_phase" as const;
export const ERROR_ILLEGAL_PLAY = "error:illegal_play" as const;
export const ERROR_PAUSE_EXHAUSTED = "error:pause_exhausted" as const;
export const ERROR_NO_ACTIVE_PAUSE = "error:no_active_pause" as const;
export const ERROR_NOT_ROOM_OWNER = "error:not_room_owner" as const;
export const ERROR_PLAYER_DISCONNECTED = "error:player_disconnected" as const;
export const ERROR_SURRENDER_EXHAUSTED = "error:surrender_exhausted" as const;
export const ERROR_MUST_PICK_TRUMP = "error:must_pick_trump" as const;

export const ERROR_MATCH_START_FAILED = "error:match_start_failed" as const;

export interface MatchErrorPayload {
  code: string;
  message: string;
}

export const SYSTEM_ROOM_CREATED = "system:room_created" as const;
export const SYSTEM_ROOM_UPDATED = "system:room_updated" as const;

/**
 * Lobby room snapshot; an absent declarationsEnabled means on and an absent stopAtTarget means off.
 */
export interface RoomCreatedPayload {
  id: number;
  name: string;
  code: string;
  ownerId: number;
  ownerUsername: string;
  ownerAvatarUrl?: string | null;
  players: RoomPlayer[];
  variant: string;
  matchMode: string;
  timerStyle: string;
  timerDurationSeconds: number | null;
  status: string;
  playerCount: number;
  isQuickPlay: boolean;
  isPrivate: boolean;
  minHonor: number;
  allowNewPlayers: boolean;
  declarationsEnabled?: boolean;
  stopAtTarget?: boolean;
  createdAt: string;
  updatedAt: string;
}

/**
 * Lobby room snapshot; an absent declarationsEnabled means on and an absent stopAtTarget means off.
 */
export interface RoomUpdatedPayload {
  id: number;
  name: string;
  code: string;
  ownerId: number;
  ownerUsername: string;
  ownerAvatarUrl?: string | null;
  players: RoomPlayer[];
  variant: string;
  matchMode: string;
  timerStyle: string;
  timerDurationSeconds: number | null;
  status: string;
  playerCount: number;
  isQuickPlay: boolean;
  isPrivate: boolean;
  minHonor: number;
  allowNewPlayers: boolean;
  declarationsEnabled?: boolean;
  stopAtTarget?: boolean;
  createdAt: string;
  updatedAt: string;
}

export const SYSTEM_PLAYER_JOINED = "system:player_joined" as const;
export const SYSTEM_PLAYER_LEFT = "system:player_left" as const;
export const SYSTEM_ROOM_KICKED = "system:room_kicked" as const;
export const SYSTEM_PLAYER_RETURNED = "system:player_returned" as const;

export interface RoomKickedPayload {
  roomId: number;
  reason: string;
}

/** Sent when a player re-enters a reopened room after a match. */
export interface PlayerReturnedPayload {
  roomId: number;
  userId: number;
}

/** Optional honor and level fields are absent when not read; a real 0 arrives as 0. */
export interface PlayerJoinedPayload {
  roomId: number;
  userId: number;
  username: string;
  avatarUrl: string | null;
  playerCount: number;
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

export const SYSTEM_SEAT_UPDATED = "system:seat_updated" as const;
export const SYSTEM_MATCH_STARTED = "system:match_started" as const;

export const SYSTEM_ROOM_OWNER_CHANGED = "system:room_owner_changed" as const;

export interface RoomOwnerChangedPayload {
  roomId: number;
  newOwnerId: number;
  newOwnerUsername: string;
  newOwnerAvatarUrl: string | null;
  previousOwnerId: number;
}

export const SYSTEM_ROOM_CLOSED_INSOLVENT = "system:room_closed_insolvent" as const;

/** Sent when a room closes with no eligible owner left, after insolvency or an honor ejection. */
export interface RoomClosedInsolventPayload {
  roomId: number;
}

export const SYSTEM_INSOLVENT_EJECTED = "system:insolvent_ejected" as const;

/** Sent to a player ejected at match start for not affording the buy-in. */
export interface InsolventEjectedPayload {
  roomId: number;
  buyIn: number;
  balance: number;
}

export const SYSTEM_HONOR_EJECTED = "system:honor_ejected" as const;

/**
 * Sent to a player whose honor no longer clears the room's minHonor; like all system:* events it
 * has no Zod schema. A score of 0 is valid, so validate with typeof rather than truthiness.
 */
export interface HonorEjectedPayload {
  roomId: number;
  minHonor: number;
  honor: number;
}

/**
 * seat and team are null when the player vacates their seat; previousSeat is null on the first
 * pick.
 */
export interface SeatUpdatedPayload {
  roomId: number;
  userId: number;
  username: string;
  avatarUrl: string | null;
  seat: number | null;
  team: string | null;
  previousSeat: number | null;
}

export const SYSTEM_BOT_ADDED = "system:bot_added" as const;
export const SYSTEM_BOT_REMOVED = "system:bot_removed" as const;

/** Bot names are rendered client-side from the seat, so none is sent. */
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

export const ACTION_CHAT_MESSAGE = "action:chat_message" as const;
export const SYSTEM_CHAT_MESSAGE = "system:chat_message" as const;

export const ACTION_EMOTE = "action:emote" as const;
export const SYSTEM_EMOTE = "system:emote" as const;

/** Wire emote identifier, mirroring the Go EmoteID type. */
export type EmoteID = "thumbs_up" | "clap" | "laugh" | "thinking" | "facepalm" | "heart";

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

/** roomId is required when channel is "match" or "room". */
export interface ChatMessageRequest {
  channel: "lobby" | "match" | "room";
  roomId?: number;
  text: string;
}

export interface ChatMessagePayload {
  userId: number;
  username: string;
  message: string;
  timestamp: string;
  scope: "lobby" | "match" | "room";
}

export const SYSTEM_FRIEND_REQUEST = "system:friend_request" as const;

/** Best-effort push for an incoming friend request; GET /friends/requests is the durable source. */
export interface FriendRequestPayload {
  requestId: number;
  fromUserId: number;
  fromUsername: string;
}

export const ACTION_WHISPER = "action:whisper" as const;
export const SYSTEM_WHISPER = "system:whisper" as const;

export const ERROR_NOT_FRIENDS = "error:not_friends" as const;
export const ERROR_WHISPER_BLOCKED_IN_GAME = "error:whisper_blocked_in_game" as const;
export const ERROR_WHISPER_RECIPIENT_OFFLINE = "error:whisper_recipient_offline" as const;

export interface WhisperRequest {
  toUsername: string;
  text: string;
}

/** Delivered to both sender and recipient; validate numeric fields with typeof, not truthiness. */
export interface WhisperPayload {
  fromUserId: number;
  fromUsername: string;
  toUserId: number;
  toUsername: string;
  message: string;
  timestamp: string;
}

export const SYSTEM_ROOM_INVITE = "system:room_invite" as const;

/**
 * isHostInvite means the server granted a one-time password bypass, so no prompt is shown. minHonor
 * (0 = ungated) lets a rejected accept show the required honor.
 */
export interface RoomInvitePayload {
  inviteId: number;
  roomId: number;
  roomName: string;
  inviterUserId: number;
  inviterUsername: string;
  coinBuyIn: number;
  isPrivate: boolean;
  isHostInvite: boolean;
  minHonor: number;
  expiresAt: string;
}

export const SYSTEM_ERROR = "system:error" as const;
export const ERROR_UNKNOWN_EVENT = "error:unknown_event" as const;

export interface ErrorPayload {
  message: string;
}
