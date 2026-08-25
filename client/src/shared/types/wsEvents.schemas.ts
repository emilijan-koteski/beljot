// WebSocket event payload schemas — runtime parsers (Zod) for the event
// payloads enumerated in `wsEvents.ts`. Two roles:
//
// 1. Drift gate against the Go server. The contract test in
//    `wsEvents.contract.test.ts` runs every JSON golden produced by
//    `server/internal/ws/events_contract_test.go` through the matching
//    schema; any rename or shape change on either side triggers a parse
//    failure (or, locally, a Go test diff) so the WS contract files stay
//    paired.
//
// 2. Compile-time conformance check against the hand-maintained interface
//    types in `wsEvents.ts`. The bottom of this file declares
//    `_*Conformance` helper types: each asserts that the schema's inferred
//    type and the interface are mutually-extending. This catches
//    schema-vs-interface drift the moment `tsc --noEmit` runs — no need to
//    wait for a runtime parse to fire.
//
// Schemas use `z.strictObject({...})` so unknown fields cause parse
// errors. That is the whole point — without strictness, the Go side could
// add a field and the TS side would silently swallow it.
//
// Coverage scope: team-shaped payloads (HandScored, MatchEnd,
// MatchAbandoned) plus the small set the spec lists as required for the
// contract goldens. Non-team-shaped payloads not in this file are covered
// by interfaces in `wsEvents.ts` only — adding their schemas is a future
// refactor, not in this commit's scope.

import { z } from "zod";

import type { MatchState, PlayerState } from "./matchTypes";
import { SERVER_PHASES } from "./matchTypes";
import type {
  AutoActionPayload,
  BelotAnnouncedPayload,
  CardPlayedPayload,
  CoinSettlementPayload,
  DeclarationsResolvedPayload,
  HandScoredPayload,
  HonorUpdatedPayload,
  MatchAbandonedPayload,
  MatchEndPayload,
  MatchPausedPayload,
  MatchResumedPayload,
  PlayerDeclaredPayload,
  PlayerDisconnectedPayload,
  PlayerReconnectedPayload,
  SurrenderDeclinedPayload,
  SurrenderProposedPayload,
  TrickResolvedPayload,
  TrumpSelectedPayload,
  XpAwardedPayload,
} from "./wsEvents";

// --- Card / declaration sub-schemas (used by MatchState + DeclarationsResolved) ---

const CardSchema = z.strictObject({
  rank: z.string(),
  suit: z.string(),
});

const DeclarationSchema = z.strictObject({
  type: z.string(),
  cards: z.array(CardSchema),
  playerSeat: z.number(),
  value: z.number(),
});

const PlayerStateSchema = z.strictObject({
  hand: z.array(CardSchema),
  seat: z.number(),
  userId: z.number(),
  username: z.string(),
  team: z.string(),
  declarations: z.array(DeclarationSchema),
  connected: z.boolean(),
  // Story 10.3: bot seats. Strict schema — this entry must land in the same
  // commit as the Go PlayerState.IsBot field or every snapshot hard-fails.
  isBot: z.boolean(),
  // Server-authoritative lifetime level (from total_xp), captured at match
  // start and static for the match. Must match the Go PlayerState.Level field.
  level: z.number().int(),
  // Story 12.8: how many cards the seat holds face-down, outside `hand`. Must
  // land in the same commit as the Go PlayerState.FaceDownCount field or the
  // contract test fails. Note the gate is BUILD-time, not runtime: these schemas
  // are exercised by wsEvents.contract.test.ts against the Go goldens, while the
  // live dispatch path casts the payload instead of parsing it — so consumers
  // still guard against a field an older server did not send.
  faceDownCount: z.number().int(),
  // Story 12.10: the open-hand counterpart of faceDownCount. match_state is
  // projected per recipient — only the viewer's own `hand` carries cards, so
  // opponents render from this count. Real hand length on all four seats.
  // Same build-time gate as faceDownCount above.
  handCount: z.number().int(),
  // Whether this seat has answered the dedicated declaration phase (declared or
  // skipped). Public on all four seats because the phase asks every seat
  // regardless of what it holds, so it reports who has clicked and never who
  // holds a meld. Must land in the same commit as the Go
  // PlayerState.DeclarationAnswered field. Same build-time gate as the two
  // counts above.
  declarationAnswered: z.boolean(),
});

const TrickCardSchema = z.strictObject({
  card: CardSchema,
  playerSeat: z.number(),
});

// EventMatchStateSchema mirrors `game.MatchState` in server/internal/game/state.go.
// The server emits this on every match-state-affecting event so the client can sync.
// Strict-object: any new field on the Go side fails this parse until the
// schema lands too.
export const EventMatchStateSchema = z.strictObject({
  id: z.number(),
  roomId: z.number(),
  variant: z.string(),
  matchMode: z.string(),
  // Enum, not z.string(): the phase string is the ONLY field carrying the
  // dedicated-declaration state, so it is load-bearing. SERVER_PHASES is pinned
  // against the Go-owned golden by wsEvents.contract.test.ts, which makes this
  // the far end of a chain that fails on drift instead of silently accepting an
  // unknown phase and rendering a blank table.
  phase: z.enum(SERVER_PHASES),
  ownerSeat: z.number(),
  handNumber: z.number(),
  dealerSeat: z.number(),
  trumpSuit: z.string().nullable(),
  trumpCallerSeat: z.number().nullable(),
  trumpCandidate: CardSchema.nullable(),
  biddingRound: z.number(),
  biddingPassCount: z.number(),
  // Story 12.8: the active seat has no legal pass. Must land in the same commit
  // as the Go GameState.MustPickTrump field — enforced by the contract test, not
  // at runtime (see faceDownCount above).
  mustPickTrump: z.boolean(),
  // The room's melds-and-Belote setting — the ONE rule-config field the server
  // puts on the wire, because it is a setting the owner chose and the UI labels
  // the table with it. Must land in the same commit as the Go
  // GameState.DeclarationsEnabled field; the contract test is what enforces
  // that, not runtime.
  declarationsEnabled: z.boolean(),
  // "Dosta": whether the match ends the instant a team's running total reaches
  // the target, hand unfinished. The SECOND rule-config field the server puts on
  // the wire, and for the same reason as declarationsEnabled above — the owner
  // chose it and the UI labels the table with it. Must land in the same commit as
  // the Go GameState.StopAtTarget field; the contract test is what enforces that,
  // not runtime.
  stopAtTarget: z.boolean(),
  // No `deck` field: the 11 held-back Bitola cards are hidden information and
  // Story 12.10 removed them from the wire outright (GameState.Deck is
  // json:"-" on the Go side). strictObject means the field cannot quietly
  // return without failing the contract test.
  trickNumber: z.number(),
  currentTrick: z.array(TrickCardSchema),
  leadSuit: z.string().nullable(),
  trickWinnerSeat: z.number().nullable(),
  awaitingDeclaration: z.boolean(),
  declarationsResolved: z.boolean(),
  players: z.tuple([PlayerStateSchema, PlayerStateSchema, PlayerStateSchema, PlayerStateSchema]),
  teamScores: z.tuple([z.number(), z.number()]),
  handPoints: z.tuple([z.number(), z.number()]),
  declarationPoints: z.tuple([z.number(), z.number()]),
  belotPoints: z.tuple([z.number(), z.number()]),
  tricksWon: z.tuple([z.number(), z.number()]),
  pendingBelotSeat: z.number().nullable(),
  belotAnnounced: z.boolean(),
  winnerTeam: z.number().nullable(),
  // LastHandResult uses the same shape as the typed payload, but on MatchState
  // it can be nil between hands. Inline-declared rather than reusing
  // HandScoredPayloadSchema because MatchState's lastHandResult does NOT
  // include match-score keys (those live on TeamScores).
  lastHandResult: z
    .strictObject({
      teamACardPoints: z.number(),
      teamBCardPoints: z.number(),
      teamADeclPoints: z.number(),
      teamBDeclPoints: z.number(),
      lastTrickTeam: z.number(),
      lastTrickBonus: z.number(),
      capot: z.boolean(),
      capotTeam: z.number().nullable(),
      capotBonus: z.number(),
      failedContract: z.boolean(),
      contractingTeam: z.number(),
      teamAHandTotal: z.number(),
      teamBHandTotal: z.number(),
    })
    .nullable(),
  activePlayerSeat: z.number(),
  turnExpiresAt: z.string().nullable(),
  timerDurationSec: z.number(),
  previousPhase: z.string(),
  pausedPlayers: z.tuple([z.boolean(), z.boolean(), z.boolean(), z.boolean()]),
  pauseUsed: z.tuple([z.boolean(), z.boolean(), z.boolean(), z.boolean()]),
  turnTimeRemaining: z.number(),
  surrenderProposerSeat: z.number().nullable(),
  surrenderUsed: z.tuple([z.boolean(), z.boolean(), z.boolean(), z.boolean()]),
  disconnectedSeat: z.number(),
  reconnectExpiresAt: z.string().nullable(),
  playerReconnectExpiresAt: z.tuple([
    z.string().nullable(),
    z.string().nullable(),
    z.string().nullable(),
    z.string().nullable(),
  ]),
});

// --- Action / state event payloads ---

export const CardPlayedPayloadSchema = z.strictObject({
  playerSeat: z.number(),
  cardId: z.string(),
  autoPlayed: z.boolean(),
});

export const TrickResolvedPayloadSchema = z.strictObject({
  winnerSeat: z.number(),
  // Schema mirrors the interface (`number`) for type-conformance; runtime
  // values from Go are still 0|1 so consumers can treat it as such.
  winnerTeam: z.number(),
  cards: z.array(z.string()),
});

// HandScoredPayloadSchema is the team-shaped payload that drove the
// teamA/teamB rename. Field-name drift here breaks ScorePanel's
// per-hand reveal — the contract test catches it before the dispatcher
// silently drops the event.
export const HandScoredPayloadSchema = z.strictObject({
  teamACardPoints: z.number(),
  teamBCardPoints: z.number(),
  teamADeclPoints: z.number(),
  teamBDeclPoints: z.number(),
  lastTrickTeam: z.number(),
  lastTrickBonus: z.number(),
  capot: z.boolean(),
  capotTeam: z.number().nullable(),
  capotBonus: z.number(),
  failedContract: z.boolean(),
  contractingTeam: z.number(),
  teamAHandTotal: z.number(),
  teamBHandTotal: z.number(),
  teamAMatchScore: z.number(),
  teamBMatchScore: z.number(),
});

// MatchEndPayloadSchema — outcomeReason / surrenderedBySeat are optional
// (Go uses omitempty). Strict-object still rejects unknown keys.
export const MatchEndPayloadSchema = z.strictObject({
  // See note on TrickResolvedPayloadSchema — schema kept as `number` to match
  // the hand-maintained interface; values from Go are still 0|1 in practice.
  winnerTeam: z.number(),
  teamAFinalScore: z.number(),
  teamBFinalScore: z.number(),
  matchDurationSec: z.number(),
  outcomeReason: z
    .union([
      z.literal("surrender"),
      z.literal("timeout"),
      z.literal("abandonment"),
      z.literal("natural"),
      z.literal("target_reached"),
    ])
    .optional(),
  surrenderedBySeat: z.number().optional(),
});

export const MatchAbandonedPayloadSchema = z.strictObject({
  abandonedByPlayer: z.number(),
  teamAFinalScore: z.number(),
  teamBFinalScore: z.number(),
  matchDurationSec: z.number(),
});

// cardId is the absorbed candidate, or EMPTY when the variant has no candidate.
// Nothing in between: a 1- or 3-character id is malformed and must not parse.
export const TrumpSelectedPayloadSchema = z.strictObject({
  playerSeat: z.number(),
  trumpSuit: z.string(),
  cardId: z.union([z.literal(""), z.string().length(2)]),
});

export const DeclarationsResolvedPayloadSchema = z.strictObject({
  // See note on TrickResolvedPayloadSchema — schema kept as `number | null`
  // to match the hand-maintained interface; values from Go are 0|1|null.
  winnerTeam: z.number().nullable(),
  contested: z.boolean(),
  declarations: z.array(
    z.strictObject({
      playerSeat: z.number(),
      type: z.string(),
      value: z.number(),
      cards: z.array(z.string()),
    }),
  ),
});

export const PlayerDeclaredPayloadSchema = z.strictObject({
  playerSeat: z.number(),
});

export const BelotAnnouncedPayloadSchema = z.strictObject({
  playerSeat: z.number(),
  team: z.number(),
  cardId: z.string(),
});

export const MatchPausedPayloadSchema = z.strictObject({
  pausedBy: z.number(),
  pausedPlayers: z.tuple([z.boolean(), z.boolean(), z.boolean(), z.boolean()]),
});

export const MatchResumedPayloadSchema = z.strictObject({
  resumedBy: z.number(),
  ownerOverride: z.boolean(),
});

export const AutoActionPayloadSchema = z.strictObject({
  playerSeat: z.number().int().min(0).max(3),
  type: z.union([
    z.literal("pass_trump"),
    z.literal("skip_declare"),
    z.literal("skip_belot"),
    // Story 12.8: the forced dealer pick. Must stay in step with the Go
    // ws.AutoActionType constants.
    z.literal("pick_trump"),
  ]),
});

// Story 9.2: per-human match-end coin settlement.
export const CoinSettlementPayloadSchema = z.strictObject({
  coinDelta: z.number().int(),
  newBalance: z.number().int(),
  pot: z.number().int(),
});

// Story 9.5: per-human match-end XP award.
export const XpAwardedPayloadSchema = z.strictObject({
  xpEarned: z.number().int(),
  newTotalXp: z.number().int(),
  newLevel: z.number().int(),
  leveledUp: z.boolean(),
});

// Story 9.7: per-human match-end honor update. honorTier is left as a plain
// string rather than a union of the five tokens on purpose — a server-side
// retune that adds a tier must not hard-fail a stale client bundle (deferred
// item D142); the display layer falls back for an unknown token instead.
export const HonorUpdatedPayloadSchema = z.strictObject({
  honorScore: z.number().int(),
  honorTier: z.string(),
  honorCompletedTotal: z.number().int(),
  honorAbandonedTotal: z.number().int(),
  isNewPlayer: z.boolean(),
});

export const PlayerDisconnectedPayloadSchema = z.strictObject({
  playerSeat: z.number(),
  username: z.string(),
  reconnectExpiresAt: z.string(),
});

export const PlayerReconnectedPayloadSchema = z.strictObject({
  playerSeat: z.number(),
});

export const SurrenderProposedPayloadSchema = z.strictObject({
  proposerSeat: z.number(),
  proposerTeam: z.number(),
  proposerUsername: z.string(),
  partnerSeat: z.number(),
});

export const SurrenderDeclinedPayloadSchema = z.strictObject({
  proposerSeat: z.number(),
  decliningSeat: z.number(),
});

// --- Compile-time conformance ---
//
// Each `_*Conformance` type asserts the schema's inferred type is mutually
// assignable to the hand-maintained interface. If a field drifts on either
// side the type evaluates to `false` and the `const _*Conforms: true =
// _*Conformance` line fails to compile. `tsc --noEmit` becomes the gate
// — no need for a runtime parse to surface a schema-vs-interface mismatch.
//
// Pattern: bidirectional `extends` so adding-only changes on either side
// also fail (a Zod-side new field that isn't on the interface is just as
// much drift as the reverse).

type MutualExtends<A, B> = A extends B ? (B extends A ? true : false) : false;

// --- MatchState / PlayerState witnesses (Story 12.10) ---
//
// The live dispatch path CASTS event:match_state to `MatchState` instead of
// parsing it (useWsDispatch), so nothing at runtime connects the interface to
// the schema the goldens gate — a field added or removed on only one side
// (exactly this story's change: `deck` out, `handCount` in) would drift
// silently. `MutualExtends` cannot witness these two directly because the
// interface deliberately narrows value types (Variant, Suit, Rank, TeamString
// are unions where the wire schema keeps plain strings), so the witness is
// split in two, each half compiling only while its guarantee holds:
//
//  1. Key parity, both directions and at both nesting levels that this story
//     touched (MatchState itself and the PlayerState tuple element) — the
//     drift class the cast leaves open is a missing/extra field, and keys
//     catch it symmetrically.
//  2. Assignability of the interface INTO the schema output (minus `phase`,
//     whose union adds the client-local "" member the server never sends) —
//     this checks every field's VALUE type in the direction the narrowing
//     allows, so e.g. `handCount: string` on either side still fails tsc.

type MutualKeys<A, B> = [keyof A] extends [keyof B]
  ? [keyof B] extends [keyof A]
    ? true
    : false
  : false;

type _MatchStateConformance = MutualKeys<z.infer<typeof EventMatchStateSchema>, MatchState>;
const _matchStateConforms: _MatchStateConformance = true;

type _PlayerStateConformance = MutualKeys<
  z.infer<typeof EventMatchStateSchema>["players"][number],
  PlayerState
>;
const _playerStateConforms: _PlayerStateConformance = true;

type _MatchStateAssignability =
  Omit<MatchState, "phase"> extends Omit<z.infer<typeof EventMatchStateSchema>, "phase">
    ? true
    : false;
const _matchStateAssignable: _MatchStateAssignability = true;

type _CardPlayedConformance = MutualExtends<
  z.infer<typeof CardPlayedPayloadSchema>,
  CardPlayedPayload
>;
const _cardPlayedConforms: _CardPlayedConformance = true;

type _TrickResolvedConformance = MutualExtends<
  z.infer<typeof TrickResolvedPayloadSchema>,
  TrickResolvedPayload
>;
const _trickResolvedConforms: _TrickResolvedConformance = true;

type _HandScoredConformance = MutualExtends<
  z.infer<typeof HandScoredPayloadSchema>,
  HandScoredPayload
>;
const _handScoredConforms: _HandScoredConformance = true;

type _MatchEndConformance = MutualExtends<z.infer<typeof MatchEndPayloadSchema>, MatchEndPayload>;
const _matchEndConforms: _MatchEndConformance = true;

type _MatchAbandonedConformance = MutualExtends<
  z.infer<typeof MatchAbandonedPayloadSchema>,
  MatchAbandonedPayload
>;
const _matchAbandonedConforms: _MatchAbandonedConformance = true;

type _TrumpSelectedConformance = MutualExtends<
  z.infer<typeof TrumpSelectedPayloadSchema>,
  TrumpSelectedPayload
>;
const _trumpSelectedConforms: _TrumpSelectedConformance = true;

type _DeclarationsResolvedConformance = MutualExtends<
  z.infer<typeof DeclarationsResolvedPayloadSchema>,
  DeclarationsResolvedPayload
>;
const _declarationsResolvedConforms: _DeclarationsResolvedConformance = true;

type _PlayerDeclaredConformance = MutualExtends<
  z.infer<typeof PlayerDeclaredPayloadSchema>,
  PlayerDeclaredPayload
>;
const _playerDeclaredConforms: _PlayerDeclaredConformance = true;

type _BelotAnnouncedConformance = MutualExtends<
  z.infer<typeof BelotAnnouncedPayloadSchema>,
  BelotAnnouncedPayload
>;
const _belotAnnouncedConforms: _BelotAnnouncedConformance = true;

type _MatchPausedConformance = MutualExtends<
  z.infer<typeof MatchPausedPayloadSchema>,
  MatchPausedPayload
>;
const _matchPausedConforms: _MatchPausedConformance = true;

type _MatchResumedConformance = MutualExtends<
  z.infer<typeof MatchResumedPayloadSchema>,
  MatchResumedPayload
>;
const _matchResumedConforms: _MatchResumedConformance = true;

type _AutoActionConformance = MutualExtends<
  z.infer<typeof AutoActionPayloadSchema>,
  AutoActionPayload
>;
const _autoActionConforms: _AutoActionConformance = true;

type _CoinSettlementConformance = MutualExtends<
  z.infer<typeof CoinSettlementPayloadSchema>,
  CoinSettlementPayload
>;
const _coinSettlementConforms: _CoinSettlementConformance = true;

type _XpAwardedConformance = MutualExtends<
  z.infer<typeof XpAwardedPayloadSchema>,
  XpAwardedPayload
>;
const _xpAwardedConforms: _XpAwardedConformance = true;

type _HonorUpdatedConformance = MutualExtends<
  z.infer<typeof HonorUpdatedPayloadSchema>,
  HonorUpdatedPayload
>;
const _honorUpdatedConforms: _HonorUpdatedConformance = true;

type _PlayerDisconnectedConformance = MutualExtends<
  z.infer<typeof PlayerDisconnectedPayloadSchema>,
  PlayerDisconnectedPayload
>;
const _playerDisconnectedConforms: _PlayerDisconnectedConformance = true;

type _PlayerReconnectedConformance = MutualExtends<
  z.infer<typeof PlayerReconnectedPayloadSchema>,
  PlayerReconnectedPayload
>;
const _playerReconnectedConforms: _PlayerReconnectedConformance = true;

type _SurrenderProposedConformance = MutualExtends<
  z.infer<typeof SurrenderProposedPayloadSchema>,
  SurrenderProposedPayload
>;
const _surrenderProposedConforms: _SurrenderProposedConformance = true;

type _SurrenderDeclinedConformance = MutualExtends<
  z.infer<typeof SurrenderDeclinedPayloadSchema>,
  SurrenderDeclinedPayload
>;
const _surrenderDeclinedConforms: _SurrenderDeclinedConformance = true;

// Suppress unused-locals — these constants exist purely for the type-level
// assertion above. Re-exporting under a private namespace gives them a
// reachable use without polluting the public module surface.
export const _conformanceWitnesses = {
  _matchStateConforms,
  _playerStateConforms,
  _matchStateAssignable,
  _cardPlayedConforms,
  _trickResolvedConforms,
  _handScoredConforms,
  _matchEndConforms,
  _matchAbandonedConforms,
  _trumpSelectedConforms,
  _declarationsResolvedConforms,
  _playerDeclaredConforms,
  _belotAnnouncedConforms,
  _matchPausedConforms,
  _matchResumedConforms,
  _autoActionConforms,
  _coinSettlementConforms,
  _xpAwardedConforms,
  _honorUpdatedConforms,
  _playerDisconnectedConforms,
  _playerReconnectedConforms,
  _surrenderProposedConforms,
  _surrenderDeclinedConforms,
};
