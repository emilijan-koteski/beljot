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
import belotAnnouncedGolden from "../../../../server/internal/ws/testdata/events/belot_announced.json";
import cardPlayedGolden from "../../../../server/internal/ws/testdata/events/card_played.json";
import coinSettlementGolden from "../../../../server/internal/ws/testdata/events/coin_settlement.json";
import declarationsResolvedGolden from "../../../../server/internal/ws/testdata/events/declarations_resolved.json";
import eventHandScoredGolden from "../../../../server/internal/ws/testdata/events/event_hand_scored.json";
import eventMatchStateGolden from "../../../../server/internal/ws/testdata/events/event_match_state.json";
import matchAbandonedGolden from "../../../../server/internal/ws/testdata/events/match_abandoned.json";
import matchEndGolden from "../../../../server/internal/ws/testdata/events/match_end.json";
import matchPausedGolden from "../../../../server/internal/ws/testdata/events/match_paused.json";
import matchResumedGolden from "../../../../server/internal/ws/testdata/events/match_resumed.json";
import playerDeclaredGolden from "../../../../server/internal/ws/testdata/events/player_declared.json";
import playerDisconnectedGolden from "../../../../server/internal/ws/testdata/events/player_disconnected.json";
import playerReconnectedGolden from "../../../../server/internal/ws/testdata/events/player_reconnected.json";
import surrenderDeclinedGolden from "../../../../server/internal/ws/testdata/events/surrender_declined.json";
import surrenderProposedGolden from "../../../../server/internal/ws/testdata/events/surrender_proposed.json";
import trickResolvedGolden from "../../../../server/internal/ws/testdata/events/trick_resolved.json";
import trumpSelectedGolden from "../../../../server/internal/ws/testdata/events/trump_selected.json";
import {
  AutoActionPayloadSchema,
  BelotAnnouncedPayloadSchema,
  CardPlayedPayloadSchema,
  CoinSettlementPayloadSchema,
  DeclarationsResolvedPayloadSchema,
  EventMatchStateSchema,
  HandScoredPayloadSchema,
  MatchAbandonedPayloadSchema,
  MatchEndPayloadSchema,
  MatchPausedPayloadSchema,
  MatchResumedPayloadSchema,
  PlayerDeclaredPayloadSchema,
  PlayerDisconnectedPayloadSchema,
  PlayerReconnectedPayloadSchema,
  SurrenderDeclinedPayloadSchema,
  SurrenderProposedPayloadSchema,
  TrickResolvedPayloadSchema,
  TrumpSelectedPayloadSchema,
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
  ["CoinSettlementPayload", CoinSettlementPayloadSchema, coinSettlementGolden],
  ["PlayerDisconnectedPayload", PlayerDisconnectedPayloadSchema, playerDisconnectedGolden],
  ["PlayerReconnectedPayload", PlayerReconnectedPayloadSchema, playerReconnectedGolden],
  ["SurrenderProposedPayload", SurrenderProposedPayloadSchema, surrenderProposedGolden],
  ["SurrenderDeclinedPayload", SurrenderDeclinedPayloadSchema, surrenderDeclinedGolden],
] as const;

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
