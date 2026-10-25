export type Suit = "S" | "H" | "D" | "C";
export type Rank = "7" | "8" | "9" | "T" | "J" | "Q" | "K" | "A";

/** Two-character card id, rank then suit, e.g. "KS" (King of Spades) or "TH" (Ten of Hearts). */
export type CardId = `${Rank}${Suit}`;

export const VARIANTS = ["bitola", "croatia"] as const;

export type Variant = (typeof VARIANTS)[number];

/** Narrow an untrusted variant string to the union, falling back to `bitola` like the engine's `game.RulesFor`. */
export function normalizeVariant(v: unknown): Variant {
  return VARIANTS.includes(v as Variant) ? (v as Variant) : "bitola";
}

/** The player's chosen card-face artwork, purely visual and unrelated to the `croatia` variant. */
export type CardDeck = "french" | "croatian";

/** The race target for a match. */
export type MatchMode = "1001" | "701" | "501";

/** The point target for a match mode, mirroring the server's `game.MatchTarget`: unknown modes race to 1001. */
export function matchTargetFor(mode: string): number {
  switch (mode) {
    case "501":
      return 501;
    case "701":
      return 701;
    default:
      return 1001;
  }
}

export const SERVER_PHASES = [
  "dealing",
  "bidding",
  "declaring",
  "playing",
  "trick_resolving",
  "hand_scoring",
  "hand_complete",
  "match_end",
  "paused",
  "disconnected",
] as const;

/** A server phase, or "" when the client has no game loaded. */
export type Phase = "" | (typeof SERVER_PHASES)[number];

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

/** Team literal; convert from the 0/1 payload index only through `teamStringForIndex`. */
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

/** One seat of the match state; only the viewer's own `hand` carries cards, other seats render from `handCount`. */
export interface PlayerState {
  hand: Card[];
  seat: number;
  userId: number;
  username: string;
  team: TeamString;
  declarations: Declaration[];
  connected: boolean;
  isBot: boolean;
  level: number;
  avatarUrl: string | null;
  faceDownCount: number;
  handCount: number;
  declarationAnswered: boolean;
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

/** The match state as projected for one recipient; the server is authoritative on every rule. */
export interface MatchState {
  id: number;
  roomId: number;
  variant: Variant;
  matchMode: MatchMode;
  phase: Phase;
  ownerSeat: number;
  handNumber: number;
  dealerSeat: number;
  trumpSuit: Suit | null;
  trumpCallerSeat: number | null;
  trumpCandidate: Card | null;
  biddingRound: number;
  biddingPassCount: number;
  mustPickTrump: boolean;
  declarationsEnabled: boolean;
  stopAtTarget: boolean;
  declarationsTransfer: boolean;
  clockwise: boolean;
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
  belotPoints: [number, number];
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
  playerReconnectExpiresAt: [string | null, string | null, string | null, string | null];
}
