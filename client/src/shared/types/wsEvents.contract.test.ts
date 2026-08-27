// WS contract test (TS side). Loads each JSON golden produced by
// `server/internal/ws/events_contract_test.go` and parses it through the
// matching Zod schema in `wsEvents.schemas.ts`. Any field rename on either
// side breaks parsing here — this is the second half of the drift gate
// described in AC-006 of the team-rename spec.
//
// JSON imports go through Vite's built-in JSON loader, same as i18n.ts. The
// relative path crosses out of `client/` into the server's testdata folder
// — that's intentional: the goldens are the single source of truth, owned
// by the Go test, and the TS side reads them read-only. If the Go test
// regenerates the goldens (via UPDATE_GOLDENS=1) the next vitest run picks
// up the new shape automatically.

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

// Each row pairs a Zod schema with its golden. `it.each` over the table
// gives one Vitest test per event, so a parse failure points straight at
// the offending payload — the diff is in `result.error.issues`.
//
// The schemas use z.strictObject, so a Go-side new key fails this with
// "Unrecognized key(s) in object". A removed key fails with "Required".
// Either way the spec's AC-006 "drift gate" works.
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
  // Story 12.8: the forced dealer pick. Separate golden because the literal
  // "pick_trump" is the part that must match the Go constant, not the shape.
  ["AutoActionPayload (pick_trump)", AutoActionPayloadSchema, autoActionPickTrumpGolden],
  ["CoinSettlementPayload", CoinSettlementPayloadSchema, coinSettlementGolden],
  ["XpAwardedPayload", XpAwardedPayloadSchema, xpAwardedGolden],
  ["HonorUpdatedPayload", HonorUpdatedPayloadSchema, honorUpdatedGolden],
  ["SeasonPointsAwardedPayload", SeasonPointsAwardedPayloadSchema, seasonPointsAwardedGolden],
  ["PlayerDisconnectedPayload", PlayerDisconnectedPayloadSchema, playerDisconnectedGolden],
  ["PlayerReconnectedPayload", PlayerReconnectedPayloadSchema, playerReconnectedGolden],
  ["SurrenderProposedPayload", SurrenderProposedPayloadSchema, surrenderProposedGolden],
  ["SurrenderDeclinedPayload", SurrenderDeclinedPayloadSchema, surrenderDeclinedGolden],
] as const;

// The phase vocabulary is pinned separately from the payload schemas: it is a
// list of strings, not an event body, and it is the ONE field Story 12.6 made
// load-bearing while leaving it typed as a bare z.string().
//
// SERVER_PHASES is the client's single source for both the `Phase` union and
// the Zod enum, so comparing it to the Go-owned golden closes the loop: adding
// a phase on either side without the other now fails here. On the Go side
// game.AllPhases() is itself guarded against the constants by
// TestAllPhasesCoversEveryConstant, so the chain has no unguarded link.
describe("phase vocabulary contract", () => {
  it("SERVER_PHASES matches the Go-owned phases golden exactly, in order", () => {
    expect(SERVER_PHASES).toEqual(phasesGolden);
  });

  it("does not include the client-local empty phase", () => {
    // "" means "no game loaded" and is never sent by the server; it belongs on
    // the Phase union but must stay out of the wire vocabulary and the Zod enum.
    expect(SERVER_PHASES as readonly string[]).not.toContain("");
  });

  it("accepts every golden phase through the match-state schema and rejects an unknown one", () => {
    for (const phase of phasesGolden) {
      const result = EventMatchStateSchema.safeParse({ ...eventMatchStateGolden, phase });
      expect(result.success, `schema rejected known phase "${phase}"`).toBe(true);
    }
    // The whole point of the enum: an unknown phase must not slip through as a
    // plain string and leave the client rendering a blank table.
    expect(
      EventMatchStateSchema.safeParse({ ...eventMatchStateGolden, phase: "bogus" }).success,
    ).toBe(false);
  });
});

describe("WS event JSON contract (Zod parse against Go-produced goldens)", () => {
  it.each(cases)("%s parses cleanly through its Zod schema", (name, schema, golden) => {
    const result = schema.safeParse(golden);
    if (!result.success) {
      // Surface the schema diff in the failure message so the cause is
      // obvious in CI logs. Without this, the user sees a generic boolean
      // mismatch and has to re-run locally to find which field drifted.
      console.error(`[${name}] schema mismatch issues:`, result.error.issues);
    }
    expect(
      result.success,
      `Schema '${name}' rejected the Go-produced golden — see console.error above for details.`,
    ).toBe(true);
  });
});
