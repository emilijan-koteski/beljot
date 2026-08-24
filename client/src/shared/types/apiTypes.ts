// API response types — keep in sync with server models

import type { CardDeck, MatchMode, Variant } from "./matchTypes";

export interface ApiResponse<T> {
  data: T;
}

export interface ApiErrorResponse {
  error: {
    code: string;
    message: string;
  };
}

export interface User {
  id: number;
  username: string;
  email: string;
  languagePreference: string;
  // Card-face artwork the match renders with (Story 12.4): "french" |
  // "croatian". PURELY VISUAL — it feeds `cardFaceUrl` and nothing else, so no
  // gameplay, engine, WS payload or bot behaviour reads it. Typed as `CardDeck`
  // rather than `string` so a typo cannot silently resolve to a missing asset
  // path. NOT the game variant: the deck is "croatian", the variant is
  // "croatia" (see `Variant` in matchTypes).
  cardDeckPreference: CardDeck;
  // Wallet fields (Story 9.1). Go zero values (0) serialize as real numbers, not
  // null — never use JS truthiness on these; compare explicitly (e.g. > 1).
  walletBalance: number;
  loginStreakDays: number;
  // XP & level (Story 9.5). Both are server-authoritative; level is derived from
  // totalXp server-side and is never recomputed on the client for any decision.
  // Go zero values serialize as real 0s — compare explicitly, never truthiness.
  totalXp: number;
  level: number;
  // Honor (Story 9.7). Server-authoritative: honorScore is recomputed from the
  // stored decayed weights on every response, and honorTier is a stable machine
  // token the client maps to an i18n label — never a display string on the wire.
  // isNewPlayer suppresses the score/tier in favour of a "New Player" chip, but
  // the real values are still present (Story 9.8's join gate reads them).
  //
  // Go zero values serialize as real 0s / false — compare explicitly, never
  // truthiness. In particular `honorScore || 80` is WRONG: a legitimate score of
  // 0 ("Problematic") would silently render as 80 ("Fair").
  honorScore: number;
  honorTier: string;
  isNewPlayer: boolean;
  createdAt: string;
}

/**
 * Minimal public shape returned by the player-search endpoint (Story 11.1,
 * GET /users?search=). Deliberately NOT the self `User` type — that carries
 * `email` and other private fields and is not a public/search projection. Only
 * `id` (for navigation to /players/:id) and `username` are exposed.
 */
export interface PlayerSearchResult {
  id: number;
  username: string;
}

/**
 * An accepted friend (Story 11.2, GET /friends). `online` is derived
 * server-side from the live WS hub. It is a Go bool on the wire, so a real
 * `false` is a legitimate value — compare with `=== true`, never JS truthiness.
 */
export interface Friend {
  id: number;
  username: string;
  online: boolean;
}

/**
 * One incoming pending friend request (Story 11.2, GET /friends/requests).
 * `fromUserId` links to the sender's public profile; `fromUsername` is rendered
 * directly in the requests list.
 */
export interface FriendRequest {
  id: number;
  fromUserId: number;
  fromUsername: string;
  createdAt: string;
}

/**
 * One row of GET /rooms/:id/invitable-friends (Story 11.5, AC1). `available` is
 * computed SERVER-side from the presence trio (online + not-in-match +
 * not-in-room); unavailable friends are still returned so the panel can show
 * them disabled with a reason instead of silently omitting them.
 *
 * `reason` is "" when available, otherwise one of the reason slugs which the
 * panel maps to a localized line.
 */
export interface InvitableFriend {
  userId: number;
  username: string;
  available: boolean;
  // "in_room" is some OTHER room; "in_this_room" is already seated here. Both
  // block the invite, but only one of them means "look for them elsewhere".
  reason: "" | "offline" | "in_match" | "in_room" | "in_this_room" | "room_full";
}

/** The four friendship states between a viewer and a subject (Story 11.2). */
export type FriendshipState = "none" | "pending_outgoing" | "pending_incoming" | "friends";

/**
 * Friendship state for a subject (Story 11.2, GET /friends/status/:id) — drives
 * the public-profile Add-Friend button. `requestId` is the row id when a
 * relationship exists (for accept/decline), and null otherwise.
 */
export interface FriendshipStatus {
  status: FriendshipState;
  requestId: number | null;
}

export interface Room {
  id: number;
  name: string;
  code: string;
  ownerId: number;
  /**
   * Display username of the room's owner, hydrated by the server via a JOIN
   * to the `users` table at response time. Lets the lobby card render a host
   * avatar without an extra round-trip per row.
   */
  ownerUsername: string;
  /**
   * Embedded players, populated only by the GET /rooms list endpoint so the
   * lobby grid can render seat chips inline. The detail endpoint
   * (GET /rooms/:id) keeps its own `{room, players}` envelope and leaves
   * this field undefined on the inner room.
   */
  players?: RoomPlayer[];
  variant: Variant;
  matchMode: MatchMode;
  timerStyle: string;
  timerDurationSeconds: number | null;
  /** Per-human coin stake paid at match start (Story 9.2). 0 = free room. */
  coinBuyIn: number;
  /**
   * Derived privacy flag (Story 9.6). Server-computed from password_hash != nil;
   * the password hash itself is never sent to the client. Drives the lobby lock
   * indicator and the join-time password prompt.
   */
  isPrivate: boolean;
  /**
   * Honor gate (Story 9.8, FR57). `minHonor` is the score an EXPERIENCED player
   * must clear, 0 = no bar; `allowNewPlayers` is whether a player with no track
   * record may enter at all. The two are INDEPENDENT — a room can be
   * `minHonor: 0, allowNewPlayers: false` ("anyone experienced"), and a New Player
   * is never score-checked.
   *
   * Both are Go zero values on the wire, so compare EXPLICITLY: `minHonor > 0` and
   * `allowNewPlayers === false`. A truthiness check on either is a bug (a real 0
   * and a real false are legitimate values, not "absent").
   */
  minHonor: number;
  allowNewPlayers: boolean;
  /**
   * Whether the room plays with melds AND the Belote/Rebelote bonus. False is
   * "bez zvanja" — no melds, no K+Q-of-trump announcement, no +20 — in either
   * variant.
   *
   * OPTIONAL, deliberately. The server sends it on every room payload it builds
   * from the struct and on `roomLifecyclePayload`, but a server that predates the
   * column does not, and the correct reading of absent is ON. So every consumer
   * compares `=== false` — the same explicit-comparison rule the honor gate above
   * documents, for the same reason.
   */
  declarationsEnabled?: boolean;
  status: string;
  playerCount: number;
  isQuickPlay: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CreateRoomRequest {
  name: string;
  variant: Variant;
  matchMode: MatchMode;
  timerStyle: string;
  timerDurationSeconds: number | null;
  /** Per-human coin stake (Story 9.2). min 0, no max; omitted → server default 500. */
  coinBuyIn: number;
  /** Private-room toggle (Story 9.6). Requires `password` when true. */
  isPrivate: boolean;
  /** Plaintext room password — sent only when `isPrivate` is true; never stored client-side. */
  password?: string;
  /**
   * Honor gate at create time (Story 9.8, FR57). Server-side both are optional
   * pointers: omitted `minHonor` defaults to 0 and omitted `allowNewPlayers`
   * defaults to true, so an explicit `false` is distinguishable from "not sent".
   * `minHonor` must be within [0,100] — the same interval, in the same unit, that
   * the server validates and the DB CHECK enforces.
   */
  minHonor: number;
  allowNewPlayers: boolean;
  /**
   * Whether the room plays with melds AND the Belote/Rebelote bonus. Required
   * here, unlike on `Room`: the modal always has a value to send, and the
   * server's own default only exists for clients that predate the toggle.
   */
  declarationsEnabled: boolean;
}

export interface RoomPlayer {
  id: number;
  roomId: number;
  userId: number;
  username: string;
  seat: number | null;
  team: string | null;
  // Synthetic bot entries arrive as {id:0, userId:0, username:"", isBot:true}.
  // Always check with `isBot === true` — never infer from a falsy userId.
  isBot: boolean;
  /**
   * Per-seat honour for the waiting-room roster (honour redesign R6). OPTIONAL and
   * nullable on purpose: the server omits both when the seat is a bot, when no
   * honour service is wired, or when the read failed — so `undefined` means "no
   * shield", never "score 0". A real 0 is a legitimate value and arrives as 0.
   *
   * Test with `typeof honorScore === "number"`, never truthiness.
   */
  honorScore?: number;
  honorTier?: string;
  /**
   * Lifetime level, hydrated alongside honour from the same roster read and
   * rendered before the shield on each seat tile. Same semantics as honorScore:
   * `undefined` means "not read" (a bot, or the hydration failed) and renders
   * nothing, while a real level 0 — a brand-new account — arrives as 0.
   */
  level?: number;
  createdAt: string;
}

export interface RoomDetail {
  room: Room;
  players: RoomPlayer[];
  // User IDs currently "present" in a reopened room (returned via "Return to
  // room" or freshly joined) vs ex-players still on the match result dialog.
  // Drives the "waiting to return" seat state and the owner Start gate.
  returnedUserIds: number[];
}

export interface SelectSeatResponse {
  players: RoomPlayer[];
  matchStarted: boolean;
}

export interface QuickPlayResponse {
  room: Room;
  seat: number;
  matchStarted: boolean;
}
