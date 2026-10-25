import type { CardDeck, MatchMode, Variant } from "./matchTypes";

/** The signed-in user's own account; numeric and boolean fields are real Go zero values, so compare explicitly, never by truthiness. */
export interface User {
  id: number;
  username: string;
  email: string;
  languagePreference: string;
  cardDeckPreference: CardDeck;
  soundEnabled: boolean;
  musicEnabled: boolean;
  soundVolume: number;
  musicVolume: number;
  walletBalance: number;
  loginStreakDays: number;
  totalXp: number;
  level: number;
  honorScore: number;
  honorTier: string;
  isNewPlayer: boolean;
  avatarUrl: string | null;
  createdAt: string;
}

/** Public player-search projection, deliberately narrower than the private `User`. */
export interface PlayerSearchResult {
  id: number;
  username: string;
  avatarUrl: string | null;
}

/** An accepted friend; `online` is a real Go bool, so compare with `=== true`. */
export interface Friend {
  id: number;
  username: string;
  avatarUrl: string | null;
  online: boolean;
}

/** One incoming pending friend request. */
export interface FriendRequest {
  id: number;
  fromUserId: number;
  fromUsername: string;
  avatarUrl: string | null;
  createdAt: string;
}

/** A friend row for the invite panel; unavailable friends are included with a `reason` slug ("" when available). */
export interface InvitableFriend {
  userId: number;
  username: string;
  avatarUrl: string | null;
  available: boolean;
  reason: "" | "offline" | "in_match" | "in_room" | "in_this_room" | "room_full";
}

/** The four friendship states between a viewer and a subject. */
export type FriendshipState = "none" | "pending_outgoing" | "pending_incoming" | "friends";

/** Friendship state for a subject; `requestId` is the relationship row id, or null when none exists. */
export interface FriendshipStatus {
  status: FriendshipState;
  requestId: number | null;
}

/** A game room; optional `declarationsEnabled` and `declarationsTransfer` mean on when absent (`=== false`), `stopAtTarget` and `clockwise` off (`=== true`). */
export interface Room {
  id: number;
  name: string;
  code: string;
  ownerId: number;
  ownerUsername: string;
  ownerAvatarUrl: string | null;
  players?: RoomPlayer[];
  variant: Variant;
  matchMode: MatchMode;
  timerStyle: string;
  timerDurationSeconds: number | null;
  coinBuyIn: number;
  isPrivate: boolean;
  minHonor: number;
  allowNewPlayers: boolean;
  declarationsEnabled?: boolean;
  stopAtTarget?: boolean;
  declarationsTransfer?: boolean;
  clockwise?: boolean;
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
  coinBuyIn: number;
  isPrivate: boolean;
  password?: string;
  minHonor: number;
  allowNewPlayers: boolean;
  declarationsEnabled: boolean;
  stopAtTarget: boolean;
  declarationsTransfer: boolean;
  clockwise: boolean;
}

export interface RoomPlayer {
  id: number;
  roomId: number;
  userId: number;
  username: string;
  avatarUrl: string | null;
  seat: number | null;
  team: string | null;
  isBot: boolean;
  honorScore?: number;
  honorTier?: string;
  level?: number;
  createdAt: string;
}

export interface RoomDetail {
  room: Room;
  players: RoomPlayer[];
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

/** The active season plus the caller's record; a player with no games gets the zero state, never a 404. */
export interface CurrentSeasonResponse {
  seasonName: string;
  endsAt: string;
  sp: number;
  rankTier: string;
  rankDivision: number | null;
  spIntoDivision: number;
  spForNextDivision: number;
  gamesPlayed: number;
  gamesCompleted: number;
}

/** One leaderboard row; `position` is its 1-based slot in the whole season order, not the page index. */
export interface LeaderboardRow {
  position: number;
  userId: number;
  username: string;
  avatarUrl: string | null;
  sp: number;
  tier: string;
  division: number | null;
  gamesPlayed: number;
}

/** The viewer's own standing, without a username since the client already has it. */
export interface LeaderboardViewer {
  position: number;
  userId: number;
  sp: number;
  tier: string;
  division: number | null;
  gamesPlayed: number;
}

/** One offset page of a season's standings; `viewer` is null only when the caller has not played that season. */
export interface LeaderboardResponse {
  items: LeaderboardRow[];
  total: number;
  limit: number;
  offset: number;
  viewer: LeaderboardViewer | null;
}

/** One season window for the leaderboard picker; `name` is a "YYYY QN" token rendered verbatim. */
export interface SeasonSummary {
  id: number;
  name: string;
  startedAt: string;
  endsAt: string;
}

/** Every season window, newest first. */
export interface SeasonsListResponse {
  items: SeasonSummary[];
}

/** One ended, played season in a player's archive, with the stored rank it finished on. */
export interface SeasonArchiveEntry {
  seasonId: number;
  seasonName: string;
  sp: number;
  tier: string;
  division: number | null;
  gamesPlayed: number;
  startedAt: string;
  endsAt: string;
}

/** A player's season archive, newest first; an unknown user id yields an empty list. */
export interface SeasonArchiveResponse {
  items: SeasonArchiveEntry[];
}

/** The subject's standing in the active season; a played season at 0 SP is a real rank. */
export interface SeasonRank {
  seasonName: string;
  tier: string;
  division: number | null;
  sp: number;
}
