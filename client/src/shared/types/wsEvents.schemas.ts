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

import type {
  AutoActionPayload,
  BelotAnnouncedPayload,
  CardPlayedPayload,
  CoinSettlementPayload,
  DeclarationsResolvedPayload,
  HandScoredPayload,
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
  phase: z.string(),
  ownerSeat: z.number(),
  handNumber: z.number(),
  dealerSeat: z.number(),
  trumpSuit: z.string().nullable(),
  trumpCallerSeat: z.number().nullable(),
  trumpCandidate: CardSchema.nullable(),
  biddingRound: z.number(),
  biddingPassCount: z.number(),
  deck: z.array(CardSchema),
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

export const TrumpSelectedPayloadSchema = z.strictObject({
  playerSeat: z.number(),
  trumpSuit: z.string(),
  cardId: z.string(),
});

export const DeclarationsResolvedPayloadSchema = z.strictObject({
  // See note on TrickResolvedPayloadSchema — schema kept as `number | null`
  // to match the hand-maintained interface; values from Go are 0|1|null.
  winnerTeam: z.number().nullable(),
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
  type: z.union([z.literal("pass_trump"), z.literal("skip_declare"), z.literal("skip_belot")]),
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
  _playerDisconnectedConforms,
  _playerReconnectedConforms,
  _surrenderProposedConforms,
  _surrenderDeclinedConforms,
};
