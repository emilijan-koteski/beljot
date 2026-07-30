// API response types — keep in sync with server models

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
  variant: string;
  matchMode: string;
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
  status: string;
  playerCount: number;
  isQuickPlay: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CreateRoomRequest {
  name: string;
  variant: string;
  matchMode: string;
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
