import { describe, expect, it } from "vitest";

import autoActionGolden from "../../../../server/internal/ws/testdata/events/auto_action.json";
import autoActionPickTrumpGolden from "../../../../server/internal/ws/testdata/events/auto_action_pick_trump.json";
import belotAnnouncedGolden from "../../../../server/internal/ws/testdata/events/belot_announced.json";
import cardPlayedGolden from "../../../../server/internal/ws/testdata/events/card_played.json";
import coinSettlementGolden from "../../../../server/internal/ws/testdata/events/coin_settlement.json";
import declarationsResolvedGolden from "../../../../server/internal/ws/testdata/events/declarations_resolved.json";
import eventHandScoredGolden from "../../../../server/internal/ws/testdata/events/event_hand_scored.json";
import eventMatchStateGolden from "../../../../server/internal/ws/testdata/events/event_match_state.json";
import honorUpdatedGolden from "../../../../server/internal/ws/testdata/events/honor_updated.json";
import matchAbandonedGolden from "../../../../server/internal/ws/testdata/events/match_abandoned.json";
import matchEndGolden from "../../../../server/internal/ws/testdata/events/match_end.json";
import matchPausedGolden from "../../../../server/internal/ws/testdata/events/match_paused.json";
import matchResumedGolden from "../../../../server/internal/ws/testdata/events/match_resumed.json";
import phasesGolden from "../../../../server/internal/ws/testdata/events/phases.json";
import playerDeclaredGolden from "../../../../server/internal/ws/testdata/events/player_declared.json";
import playerDisconnectedGolden from "../../../../server/internal/ws/testdata/events/player_disconnected.json";
import playerReconnectedGolden from "../../../../server/internal/ws/testdata/events/player_reconnected.json";
import seasonPointsAwardedGolden from "../../../../server/internal/ws/testdata/events/season_points_awarded.json";
import seasonPointsAwardedPartnerAbandonedGolden from "../../../../server/internal/ws/testdata/events/season_points_awarded_partner_abandoned.json";
import surrenderDeclinedGolden from "../../../../server/internal/ws/testdata/events/surrender_declined.json";
import surrenderProposedGolden from "../../../../server/internal/ws/testdata/events/surrender_proposed.json";
import trickResolvedGolden from "../../../../server/internal/ws/testdata/events/trick_resolved.json";
import trumpSelectedGolden from "../../../../server/internal/ws/testdata/events/trump_selected.json";
import xpAwardedGolden from "../../../../server/internal/ws/testdata/events/xp_awarded.json";
import { SERVER_PHASES } from "./matchTypes";
import {
  AutoActionPayloadSchema,
  BelotAnnouncedPayloadSchema,
  CardPlayedPayloadSchema,
  CoinSettlementPayloadSchema,
  DeclarationsResolvedPayloadSchema,
  EventMatchStateSchema,
  HandScoredPayloadSchema,
  HonorUpdatedPayloadSchema,
  MatchAbandonedPayloadSchema,
  MatchEndPayloadSchema,
  MatchPausedPayloadSchema,
  MatchResumedPayloadSchema,
  PlayerDeclaredPayloadSchema,
  PlayerDisconnectedPayloadSchema,
  PlayerReconnectedPayloadSchema,
  SeasonPointsAwardedPayloadSchema,
  SurrenderDeclinedPayloadSchema,
  SurrenderProposedPayloadSchema,
  TrickResolvedPayloadSchema,
  TrumpSelectedPayloadSchema,
  XpAwardedPayloadSchema,
} from "./wsEvents.schemas";

const cases = [
  ["EventMatchState", EventMatchStateSchema, eventMatchStateGolden],
  ["CardPlayedPayload", CardPlayedPayloadSchema, cardPlayedGolden],
  ["TrickResolvedPayload", TrickResolvedPayloadSchema, trickResolvedGolden],
  ["HandScoredPayload", HandScoredPayloadSchema, eventHandScoredGolden],
  ["MatchEndPayload", MatchEndPayloadSchema, matchEndGolden],
  ["MatchAbandonedPayload", MatchAbandonedPayloadSchema, matchAbandonedGolden],
  ["TrumpSelectedPayload", TrumpSelectedPayloadSchema, trumpSelectedGolden],
  ["DeclarationsResolvedPayload", DeclarationsResolvedPayloadSchema, declarationsResolvedGolden],
  ["PlayerDeclaredPayload", PlayerDeclaredPayloadSchema, playerDeclaredGolden],
  ["BelotAnnouncedPayload", BelotAnnouncedPayloadSchema, belotAnnouncedGolden],
  ["MatchPausedPayload", MatchPausedPayloadSchema, matchPausedGolden],
  ["MatchResumedPayload", MatchResumedPayloadSchema, matchResumedGolden],
  ["AutoActionPayload", AutoActionPayloadSchema, autoActionGolden],
  ["AutoActionPayload (pick_trump)", AutoActionPayloadSchema, autoActionPickTrumpGolden],
  ["CoinSettlementPayload", CoinSettlementPayloadSchema, coinSettlementGolden],
  ["XpAwardedPayload", XpAwardedPayloadSchema, xpAwardedGolden],
  ["HonorUpdatedPayload", HonorUpdatedPayloadSchema, honorUpdatedGolden],
  ["SeasonPointsAwardedPayload", SeasonPointsAwardedPayloadSchema, seasonPointsAwardedGolden],
  [
    "SeasonPointsAwardedPayload (master, partner abandoned)",
    SeasonPointsAwardedPayloadSchema,
    seasonPointsAwardedPartnerAbandonedGolden,
  ],
  ["PlayerDisconnectedPayload", PlayerDisconnectedPayloadSchema, playerDisconnectedGolden],
  ["PlayerReconnectedPayload", PlayerReconnectedPayloadSchema, playerReconnectedGolden],
  ["SurrenderProposedPayload", SurrenderProposedPayloadSchema, surrenderProposedGolden],
  ["SurrenderDeclinedPayload", SurrenderDeclinedPayloadSchema, surrenderDeclinedGolden],
] as const;

describe("phase vocabulary contract", () => {
  it("SERVER_PHASES matches the Go-owned phases golden exactly, in order", () => {
    expect(SERVER_PHASES).toEqual(phasesGolden);
  });

  it("does not include the client-local empty phase", () => {
    expect(SERVER_PHASES as readonly string[]).not.toContain("");
  });

  it("accepts every golden phase through the match-state schema and rejects an unknown one", () => {
    for (const phase of phasesGolden) {
      const result = EventMatchStateSchema.safeParse({ ...eventMatchStateGolden, phase });
      expect(result.success, `schema rejected known phase "${phase}"`).toBe(true);
    }
    expect(
      EventMatchStateSchema.safeParse({ ...eventMatchStateGolden, phase: "bogus" }).success,
    ).toBe(false);
  });
});

describe("season points payload contract", () => {
  it("rejects the retired 13.1 shape", () => {
    expect(
      SeasonPointsAwardedPayloadSchema.safeParse({
        spEarned: 24,
        newSeasonSp: 424,
        rankTier: "silver",
        tieredUp: true,
        seasonName: "2026 Q3",
      }).success,
    ).toBe(false);
  });

  it("accepts every rank change and reason token, and rejects an unknown one", () => {
    for (const rankChange of ["promoted", "demoted", "none"]) {
      for (const reason of ["normal", "abandoned", "partner_abandoned"]) {
        const result = SeasonPointsAwardedPayloadSchema.safeParse({
          ...seasonPointsAwardedGolden,
          rankChange,
          reason,
        });
        expect(result.success, `${rankChange} / ${reason}`).toBe(true);
      }
    }
    expect(
      SeasonPointsAwardedPayloadSchema.safeParse({ ...seasonPointsAwardedGolden, rankChange: "up" })
        .success,
    ).toBe(false);
    expect(
      SeasonPointsAwardedPayloadSchema.safeParse({ ...seasonPointsAwardedGolden, reason: "quit" })
        .success,
    ).toBe(false);
  });

  it("requires rankDivision to be present, as a number or null", () => {
    const withoutDivision: Record<string, unknown> = { ...seasonPointsAwardedGolden };
    delete withoutDivision.rankDivision;
    expect(SeasonPointsAwardedPayloadSchema.safeParse(withoutDivision).success).toBe(false);
    expect(
      SeasonPointsAwardedPayloadSchema.safeParse({
        ...seasonPointsAwardedGolden,
        rankDivision: null,
      }).success,
    ).toBe(true);
  });
});

describe("WS event JSON contract (Zod parse against Go-produced goldens)", () => {
  it.each(cases)("%s parses cleanly through its Zod schema", (name, schema, golden) => {
    const result = schema.safeParse(golden);
    if (!result.success) {
      console.error(`[${name}] schema mismatch issues:`, result.error.issues);
    }
    expect(
      result.success,
      `Schema '${name}' rejected the Go-produced golden — see console.error above for details.`,
    ).toBe(true);
  });
});
