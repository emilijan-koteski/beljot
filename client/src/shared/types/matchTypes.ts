// Card ID format: two-character strings (rank + suit)
// Rank: 7, 8, 9, T (ten), J, Q, K, A
// Suit: S (spades), H (hearts), D (diamonds), C (clubs)
// Examples: KS = King of Spades, TH = Ten of Hearts

export type Suit = "S" | "H" | "D" | "C";
export type Rank = "7" | "8" | "9" | "T" | "J" | "Q" | "K" | "A";
export type CardId = `${Rank}${Suit}`;

// VARIANTS is every game variant, as a runtime VALUE so that a surface which
// has to enumerate them (the rules page's tab bar, the in-match rules overlay)
// derives its list from the union instead of hand-copying it. `Variant` is
// derived FROM this array, so the two can never disagree.
//
// Order is presentation order: `bitola` first, the variant Quick Play offers and
// the default everywhere a variant is unknown.
export const VARIANTS = ["bitola", "croatia"] as const;

export type Variant = (typeof VARIANTS)[number];

/**
 * Narrow an untrusted variant string to the union, falling back to `bitola`.
 *
 * The fallback mirrors the engine's own (`game.RulesFor` resolves anything
 * unrecognised to the Bitola preset) so client and server agree on what an
 * unknown variant means. It exists as ONE function because the hand-rolled
 * `v === "croatia" ? "croatia" : "bitola"` it replaces silently resolved every
 * future variant to Bitola with no type error — which, on the rules surfaces, is
 * a page describing the wrong rules over a live table.
 */
export function normalizeVariant(v: unknown): Variant {
  return VARIANTS.includes(v as Variant) ? (v as Variant) : "bitola";
}

/**
 * The card-face artwork a player has chosen (Story 12.4). `french` is the
 * original French-suited deck; `croatian` is the German/Hungarian-suited deck
 * (leaves / hearts / bells / acorns).
 *
 * PURELY VISUAL — it selects assets and nothing else. No gameplay, engine, WS
 * payload or bot behaviour reads it.
 *
 * NOT the game variant. This is `croatian`; the variant is `croatia` (see
 * `Variant` above). Two unrelated enums that happen to share a country — a
 * Croatian-variant room does not imply the Croatian deck.
 *
 * Lives here beside `Suit` and `Variant` because it is a wire value carried on
 * the auth envelope and the profile DTO: `shared/` must not reach into
 * `features/` for the type of one of its own fields. `features/match/lib/cardFace`
 * re-exports it for the render code that lives next to it.
 */
export type CardDeck = "french" | "croatian";

// The race target for a match. Stringly-typed end to end until now (D139): the
// server sends "1001"/"501" as a string and every consumer re-tested it with a
// bare === against a string literal, so a typo produced a silent fallback
// rather than a compile error.
export type MatchMode = "1001" | "501";

// SERVER_PHASES is every phase the server can send, in state-machine order.
// It is a runtime VALUE, not just a type, for two reasons: the Zod schema needs
// it to validate `phase` as an enum rather than a bare string, and the contract
// test can compare it byte-for-byte against the Go-owned golden
// (server/internal/ws/testdata/events/phases.json). Before this existed the
// phase string was matched by nothing but hand-copying, so a server constant
// and its client union member could drift with no test failing anywhere.
//
// Keep in the SAME ORDER as game.AllPhases() — wsEvents.contract.test.ts
// compares the two as ordered lists.
//
// "declaring" is the dedicated declaration phase between bidding and trick 1.
// The server decides when it happens — the client only reacts to the phase it
// is told it is in, and derives nothing about it from the variant.
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

// "" is client-local ("no game loaded") and is never sent by the server, which
// is why it lives here rather than in SERVER_PHASES.
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
  // Bot seats carry userId 0 + empty username; display surfaces render the
  // localized seat-derived bot name. Check with `isBot === true`, never
  // truthiness on userId/username.
  isBot: boolean;
  // Server-authoritative lifetime level (derived from total_xp), captured once
  // at match start and static for the whole match. Bot seats are 0.
  level: number;
  // How many cards this seat holds face-down, outside `hand`. Non-zero only
  // while a Croatian hand is still bidding; 0 everywhere else.
  //
  // A count, never the cards: the identities are server-only, and the viewer's
  // own two arrive on a per-seat event. A seat's rendered stack is
  // `handCount + faceDownCount`, which is server-authoritative and needs no
  // client-side variant branch.
  faceDownCount: number;
  // How many cards this seat holds in `hand` — the open-hand counterpart of
  // faceDownCount. The server projects every match_state per recipient
  // (Story 12.10): only the viewer's own `hand` carries cards, every other
  // seat's arrives empty, and this count is what renders their card backs.
  // Real hand length on all four seats, the viewer's own included.
  handCount: number;
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
  /** Server-authoritative: the seat on the clock has NO legal pass, so
   *  pick_trump is its only bid (the dealer, speaking fourth in the single
   *  bidding round of a variant where the hand must find a taker). Drives
   *  whether the trump prompt offers a Pass control — the client never
   *  re-derives the rule. */
  mustPickTrump: boolean;
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
  /** Belote/rebelote bonus (K+Q of trump). Classified as a declaration, kept
   *  separate from handPoints (card points). See server GameState.BelotPoints. */
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
  /** Per-seat reconnect window expiry (RFC3339, nullable per seat). The
   *  server tracks one window per disconnected player so concurrent drops
   *  don't share a clock — `disconnectedSeat` / `reconnectExpiresAt` above
   *  remain the legacy view of whichever seat closes soonest. */
  playerReconnectExpiresAt: [string | null, string | null, string | null, string | null];
}
