// Card ID format: two-character strings (rank + suit)
// Rank: 7, 8, 9, T (ten), J, Q, K, A
// Suit: S (spades), H (hearts), D (diamonds), C (clubs)
// Examples: KS = King of Spades, TH = Ten of Hearts

export type Suit = "S" | "H" | "D" | "C";
export type Rank = "7" | "8" | "9" | "T" | "J" | "Q" | "K" | "A";
export type CardId = `${Rank}${Suit}`;

export type Variant = "bitola";

export type Phase =
  | ""
  | "dealing"
  | "bidding"
  | "playing"
  | "trick_resolving"
  | "hand_scoring"
  | "hand_complete"
  | "match_end"
  | "paused"
  | "disconnected";

export type ActionType =
  | "play_card"
  | "pick_trump"
  | "pass_trump"
  | "declare"
  | "skip_declare"
  | "announce_belot"
  | "decline_belot"
  | "pause"
  | "unpause"
  | "owner_unpause"
  | "surrender_request"
  | "surrender_accept"
  | "surrender_decline";

export type DeclarationType = "sequence" | "four_of_a_kind";

// Team string literal — full word, never bare "a"/"b" (Winston's grep-ability rule).
// Conversion between the integer team index (0/1) used in payload fields and this
// string literal lives in exactly one helper, `teamStringForIndex`.
export type TeamString = "teamA" | "teamB";

export function teamStringForIndex(i: 0 | 1): TeamString;
export function teamStringForIndex(i: number): TeamString | null;
export function teamStringForIndex(i: number): TeamString | null {
  if (i === 0) return "teamA";
  if (i === 1) return "teamB";
  return null;
}

export interface Card {
  rank: Rank;
  suit: Suit;
}

export interface Declaration {
  type: DeclarationType;
  cards: Card[];
  playerSeat: number;
  value: number;
}

export interface TrickCard {
  card: Card;
  playerSeat: number;
}

export interface PlayerState {
  hand: Card[];
  seat: number;
  userId: number;
  username: string;
  team: TeamString;
  declarations: Declaration[];
  connected: boolean;
}

export interface HandResult {
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
}

export interface MatchState {
  id: number;
  roomId: number;
  variant: Variant;
  matchMode: string;
  phase: Phase;
  ownerSeat: number;
  handNumber: number;
  dealerSeat: number;
  trumpSuit: Suit | null;
  trumpCallerSeat: number | null;
  trumpCandidate: Card | null;
  biddingRound: number;
  biddingPassCount: number;
  deck: Card[];
  activePlayerSeat: number;
  trickNumber: number;
  currentTrick: TrickCard[];
  leadSuit: Suit | null;
  trickWinnerSeat: number | null;
  awaitingDeclaration: boolean;
  declarationsResolved: boolean;
  players: [PlayerState, PlayerState, PlayerState, PlayerState];
  teamScores: [number, number];
  handPoints: [number, number];
  declarationPoints: [number, number];
  tricksWon: [number, number];
  pendingBelotSeat: number | null;
  belotAnnounced: boolean;
  winnerTeam: number | null;
  lastHandResult: HandResult | null;
  turnExpiresAt: string | null;
  timerDurationSec: number;
  previousPhase: Phase;
  pausedPlayers: [boolean, boolean, boolean, boolean];
  pauseUsed: [boolean, boolean, boolean, boolean];
  turnTimeRemaining: number;
  surrenderProposerSeat: number | null;
  surrenderUsed: [boolean, boolean, boolean, boolean];
  disconnectedSeat: number;
  reconnectExpiresAt: string | null;
  /** Per-seat reconnect window expiry (RFC3339, nullable per seat). The
   *  server tracks one window per disconnected player so concurrent drops
   *  don't share a clock — `disconnectedSeat` / `reconnectExpiresAt` above
   *  remain the legacy view of whichever seat closes soonest. */
  playerReconnectExpiresAt: [string | null, string | null, string | null, string | null];
}
