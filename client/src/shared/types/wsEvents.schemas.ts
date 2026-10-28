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
  SeasonPointsAwardedPayload,
  SurrenderDeclinedPayload,
  SurrenderProposedPayload,
  TrickResolvedPayload,
  TrumpSelectedPayload,
  XpAwardedPayload,
} from "./wsEvents";
import { SEASON_RANK_CHANGES, SEASON_SP_REASONS } from "./wsEvents";

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
  isBot: z.boolean(),
  level: z.number().int(),
  avatarUrl: z.string().nullable(),
  faceDownCount: z.number().int(),
  handCount: z.number().int(),
  declarationAnswered: z.boolean(),
});

const TrickCardSchema = z.strictObject({
  card: CardSchema,
  playerSeat: z.number(),
});

export const EventMatchStateSchema = z.strictObject({
  id: z.number(),
  roomId: z.number(),
  variant: z.string(),
  matchMode: z.string(),
  phase: z.enum(SERVER_PHASES),
  ownerSeat: z.number(),
  handNumber: z.number(),
  dealerSeat: z.number(),
  trumpSuit: z.string().nullable(),
  trumpCallerSeat: z.number().nullable(),
  trumpCandidate: CardSchema.nullable(),
  biddingRound: z.number(),
  biddingPassCount: z.number(),
  mustPickTrump: z.boolean(),
  declarationsEnabled: z.boolean(),
  stopAtTarget: z.boolean(),
  declarationsTransfer: z.boolean(),
  clockwise: z.boolean(),
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

export const CardPlayedPayloadSchema = z.strictObject({
  playerSeat: z.number(),
  cardId: z.string(),
  autoPlayed: z.boolean(),
});

export const TrickResolvedPayloadSchema = z.strictObject({
  winnerSeat: z.number(),
  winnerTeam: z.number(),
  cards: z.array(z.string()),
});

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

export const MatchEndPayloadSchema = z.strictObject({
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

export const TrumpSelectedPayloadSchema = z.strictObject({
  playerSeat: z.number(),
  trumpSuit: z.string(),
  cardId: z.union([z.literal(""), z.string().length(2)]),
});

export const DeclarationsResolvedPayloadSchema = z.strictObject({
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
    z.literal("pick_trump"),
    z.literal("timeout"),
  ]),
});

export const CoinSettlementPayloadSchema = z.strictObject({
  coinDelta: z.number().int(),
  newBalance: z.number().int(),
  pot: z.number().int(),
});

export const XpAwardedPayloadSchema = z.strictObject({
  xpEarned: z.number().int(),
  newTotalXp: z.number().int(),
  newLevel: z.number().int(),
  leveledUp: z.boolean(),
});

export const HonorUpdatedPayloadSchema = z.strictObject({
  honorScore: z.number().int(),
  honorTier: z.string(),
  honorCompletedTotal: z.number().int(),
  honorAbandonedTotal: z.number().int(),
  isNewPlayer: z.boolean(),
});

export const SeasonPointsAwardedPayloadSchema = z.strictObject({
  spChange: z.number().int(),
  newSeasonSp: z.number().int(),
  rankTier: z.string(),
  rankDivision: z.number().int().nullable(),
  rankChange: z.enum(SEASON_RANK_CHANGES),
  reason: z.enum(SEASON_SP_REASONS),
  seasonName: z.string(),
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

/** True when A and B are mutually assignable, so each witness below fails tsc on schema-vs-interface drift. */
type MutualExtends<A, B> = A extends B ? (B extends A ? true : false) : false;

/** True when A and B have exactly the same keys, for interfaces that narrow the wire's plain strings. */
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

/** Checks every MatchState value type against the schema, minus `phase`, whose union adds the client-local "". */
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

type _SeasonPointsAwardedConformance = MutualExtends<
  z.infer<typeof SeasonPointsAwardedPayloadSchema>,
  SeasonPointsAwardedPayload
>;
const _seasonPointsAwardedConforms: _SeasonPointsAwardedConformance = true;

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
  _seasonPointsAwardedConforms,
  _playerDisconnectedConforms,
  _playerReconnectedConforms,
  _surrenderProposedConforms,
  _surrenderDeclinedConforms,
};
