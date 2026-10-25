---
title: 'Room rule: declarations transfer on a failed contract'
type: 'feature'
created: '2026-10-25'
status: 'done'
baseline_commit: '84d87a925a8ee7e604a9c9e7fdee9840a387985a'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/.claude/rules/game-rules.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When the taker's team goes down ("пад"), the opponents bank every point of the hand, including the declarations the fallen team announced. Some tables play without that transfer, and a room cannot express it today.

**Approach:** Add an advanced room rule, "declarations transfer", that defaults to on (today's behavior). When it is off and one side takes the whole hand (a fall or a Capot), that side gets the card play plus its own declarations. The other side's declarations and Belote are voided, and nobody scores them.

## Boundaries & Constraints

**Decisions:**
- **The fallen team's declarations are voided** when the flag is off. They do not go to the winners, and the fallen team does not keep them.
- **Belote/Rebelote follows the declarations rule**, matching the "no declarations" toggle.
- **Capot obeys the flag too.** The capot team banks 252 plus its own declarations, and the other team's declarations are voided.

**Always:** Default on everywhere: new rooms, Quick Play, and rooms or matches that existed before the migration. The engine reads `state.Rules.DeclarationsTransfer` and never the variant name. The failure test does not change: the taker still needs strictly more than the opponents, declarations included, and a tie still fails. The flag is fixed for the whole match, shown on every surface that shows `stopAtTarget`, and stored on the match row for history.

**Never:** Change scoring when the flag is on. Change the surrender, stop-at-target, instant-win or match-winner logic. Add ranked or season restrictions (the existing flags have none). Add knowledge of the flag to bots.

## I/O & Edge-Case Matrix

Taker = team A in every row. "Cards" includes the last-trick bonus, so the two sides sum to 162.

| Scenario | Input / State | Expected Output / Behavior |
|----------|--------------|---------------------------|
| Flag on, fall | A cards 50 + decl 20, B cards 112 | B +182, A +0 (unchanged) |
| Flag off, contract made | A cards 100 + decl 50, B cards 62 | A +150, B +62 (unchanged) |
| Flag off, fall, winners declared | A cards 60, B cards 102 + decl 50 | B +212, A +0 (same as today) |
| Flag off, fall, fallen team declared | A cards 50 + decl 20, B cards 112 | B +162, A +0; the 20 is void |
| Flag off, fall, fallen team has Belote | A cards 40 + Belote 20, B cards 122 | B +162, A +0 |
| Flag off, fall, both have Belote/decl | A cards 50 + Belote 20, B cards 112 + decl 50 | B +212, A +0 |
| Flag off, tie fails | A cards 71 + decl 20, B cards 91 | B +162, A +0 |
| Flag on, B capots A | A decl 20, B cards 252 | B +272, A +0 (unchanged) |
| Flag off, B capots A | A decl 20, B cards 252 | B +252, A +0 |
| Flag off, A capots, B won declarations | A cards 252, B decl 50 | A +252, B +0 |

</frozen-after-approval>

## Code Map

- `server/internal/game/scoring.go` `scoreHand` -- the `failedContract` and `isCapot` branches award `allPoints`; the change goes here.
- `server/internal/game/types.go` `VariantRules`/`RulesFor`; `state.go` `NewGame` (positional room flags) and the `GameState` wire mirrors; `rules_engine.go` `RefreshDerivedFlags`.
- `server/internal/game/testfixtures/fixtures.go` -- `WithStopAtTarget` is the fixture template; `stop_at_target_test.go` is the test template.
- `server/internal/match/live_match.go` `Manager.StartMatch` (calls `NewGame`; persists flags at ~L1002), `match/model.go` Match row; `user/handler.go` `MatchListItem`/`buildMatchListItem`.
- `server/internal/room/model.go` Room; `handler.go` `CreateRoomRequest` (`*bool`), `MatchStarter`, `roomLifecyclePayload`, `CreateRoom` defaults, both `StartMatch` call sites, and the QuickPlay Room literal plus its `system:room_created` map; `lobby_disconnect.go` `broadcastRoomUpdated` (must match `roomLifecyclePayload`).
- `server/migrations/` -- latest is 000030. Mirror 000022 and 000023, with no GORM `default` tag on the bool.
- `server/internal/ws/testdata/events/event_match_state.json`, `events_contract_test.go` -- golden for the wire mirror.
- Client contract: `shared/types/apiTypes.ts` (Room optional, CreateRoomRequest required), `wsEvents.ts` room payloads (optional), `wsEvents.schemas.ts` (`z.strictObject`, so the field must be added), `matchTypes.ts` MatchState, `shared/api/matches.ts` MatchListItem.
- Client UI (follow `stopAtTarget` in each):
  - `features/room/CreateRoomModal.tsx`: state, `advancedChangedCount`, payload, reset, Segmented field, `PreviewCard` chip.
  - Chips in `features/lobby/components/RoomCard.tsx`, `features/room/RoomPage.tsx`, `features/match/components/ScorePanel.tsx` (prop passed from `MatchPage.tsx`) and `shared/components/matchStats/MatchStatsCard.tsx`.
  - Note in `features/match/components/RulesDialog.tsx`.
  - `features/match/components/ScoreReveal.tsx`: the `subtitleFailed*` copy says "all points".
- `client/src/shared/i18n/{en,sr,hr,mk}.json` -- `lobby.createRoomModal.*`, `lobby.card.*`, `match.rules.*`, `match.scoreReveal.*`, `advanced.summary`.

## Tasks & Acceptance

**Execution:**
- [x] `server/migrations/000031_add_declarations_transfer.{up,down}.sql` -- add `declarations_transfer BOOLEAN NOT NULL DEFAULT TRUE` to `rooms` and `matches`; down drops both -- existing rows keep today's rule.
- [x] `server/internal/game/{types,state,rules_engine}.go` -- `VariantRules.DeclarationsTransfer` (true in both presets), `NewGame` positional `declarationsTransfer bool` after `stopAtTarget`, wire mirror `declarationsTransfer` -- the positional argument makes a forgotten flag a compile error.
- [x] `server/internal/game/scoring.go` -- when the flag is off, award per the I/O matrix and decisions; the `HandScore` fields keep raw per-team declarations, and the totals carry the awarded amounts.
- [x] `server/internal/game/testfixtures/fixtures.go` and a new `declarations_transfer_test.go` -- fixture `WithoutDeclarationsTransfer`; a table test of every I/O row.
- [x] `server/internal/match/{live_match,model}.go`, `server/internal/user/handler.go` -- thread the flag into `NewGame`, persist it on the Match row, and expose it in `MatchListItem`.
- [x] `server/internal/room/{model,handler,lobby_disconnect}.go` -- Room field, request default true, `MatchStarter` signature and both call sites, every hand-built payload map, and the QuickPlay literal.
- [x] Server tests -- fix positional-argument fallout and update the ws golden and the payload parity test; add a room handler test (omitted means true, explicit false persists) and a match session test that the flag reaches the engine.
- [x] Client types (listed in the Code Map) -- add `declarationsTransfer`.
- [x] `CreateRoomModal.tsx` plus the chip, note and subtitle surfaces in the Code Map -- show the rule only when it is off, and change the ScoreReveal fall subtitle to "card points" wording when it is off.
- [x] i18n, all four locales -- new keys, and the advanced summary updated.
- [x] Client tests -- CreateRoomModal (sends false when chosen, true by default), RoomCard, ScorePanel, ScoreReveal, RulesDialog, MatchStatsCard, and the MatchState fixture fields.

**Acceptance Criteria:**
- Given a create-room request without the field, when the match starts, then `declarationsTransfer` is true on the wire and every existing scoring test passes unchanged.
- Given the owner sets the rule off, when players see the lobby card, room page, score panel, rules dialog and the finished match's history, then each shows that declarations do not transfer.
- Given the flag is off and the taker falls, when the score reveal opens, then the subtitle no longer claims "all points" and the hand-total row matches the server award.

## Implementation Notes

- `scoreHand` routes both whole-hand branches (Capot and fall) through one helper, `wholeHandAward`, so the flag is read in one place.
- Beyond the Code Map: the abandoned-match row written on a reconnect timeout (`match/reconnect.go`) and the one written for a stale room at boot (`match/reconcile.go`, via `StaleRoom` and `room/stale_room_adapter.go`) now record the flag too. Without that, GORM would write `false` there and history would label every abandoned match "No declaration transfer". The sibling `declarationsEnabled` and `stopAtTarget` flags are still missing on those two rows, a pre-existing gap left as is.
- The create-room field sits directly under "Declarations" and is hidden while declarations are off. A room without declarations always submits and stores `declarationsTransfer: true`, which `CreateRoom` also enforces.
- Client tests must run on the pinned Node 22 (`mise exec --`); on Node 24 `MatchPage.test.tsx` fails 81 tests at baseline as well.
- Follow-up commit, at the owner's request:
  - Fixed both deferred triage items: abandoned and reconciled rows now record all three rule flags, and `.claude/rules/game-rules.md` documents the rule.
  - Renamed Capot in the mk locale and rules content from „капот“ to „чаљо“.

## Spec Change Log

## Review Triage Log

| # | Source | Finding | Verdict | Evidence | Route |
|---|--------|---------|---------|----------|-------|
| 1 | edge-case | Fall subtitle says "card points to us" while the award includes the winners' own declarations | false | The copy is true: every card point went to the winners, and it never says "only". The winners' own declarations show in the Declarations row and in the Hand total. | rejected |
| 2 | edge-case, blind | Transfer setting is still offered and stored when declarations are off, so contradictory "No declarations" and "No declaration transfer" chips and notes appear | medium | `CreateRoomModal.tsx` always renders the segment, and `CreateRoom` stores any combination. Reachable from the UI. | patch |
| 3 | edge-case, blind, verification-gap | Abandoned and reconciled match rows omit `DeclarationsEnabled` and `StopAtTarget` | medium | `reconnect.go` and `reconcile.go` literals leave them unset, so GORM writes false and history shows "No declarations". Pre-existing. | defer |
| 4 | blind | Fall subtitle changes wording even when nothing was voided | false | "Card points to us" is still a true statement in that case; no wrong output. | rejected |
| 5 | blind | Voided declarations show in the breakdown as if they counted; no voided field | false | The Declarations row is the raw breakdown by design. With transfer on, a fallen team's row already shows its declarations next to a 0 total. | rejected |
| 6 | blind | sr/hr/mk use "ruka/рака" (hand) for trick; en mixes "takes no trick" with "is swept" | medium | The same locales use "Pala ruka/Падната рака" for a hand and "štih/штих" for a trick, so the hint, aria label and note read "takes no hand". | patch |
| 7 | blind, verification-gap | `.claude/rules/game-rules.md` Scoring section is not updated for the per-room rule | medium | Line 43 states all points go to the opponents unconditionally. The fix edits an agent-context rules file. | defer |
| 8 | blind | Rules note lacks the "scoring chapter below describes the usual rule" sentence, and the chapter contradicts it | low | `rules/content/en.ts` fall and Capot rules say the winners take both teams' declarations; the sibling `stopAtTargetNote` points this out. | patch |
| 9 | blind | Chip meaning is only in `aria-label`, with no visible explanation | false | The label names the rule in players' terms, like the sibling "Stop at target" and "No declarations" chips. The in-match rules note explains it. | rejected |
| 10 | blind | ScorePanel separator guards repeat every condition | low | Developer-only. It would bite only when the next flag is added, and the fix is a refactor. | rejected |
| 11 | blind | Rule-chip markup is duplicated across five surfaces | low | Follows the existing `stopAtTarget` pattern. The fix is a refactor with no current defect. | rejected |
| 12 | blind | Voiding cases untested: opponents' Belote under the taker's Capot, decl-off with transfer-off, ScoreReveal Capot, history grid | low | Row 10 covers declarations but not Belote under a taker Capot; a one-row addition. The other cases have no new logic, or are covered by the #2 patch. | patch |
| 13 | blind | Abandoned and reconciled session tests lack `t.Run`, wait a real 1s window, and skip the sibling flags | low | Developer-only test ergonomics; the sibling-flag part belongs to #3. | rejected |
| 14 | verification-gap | MatchPage never checks that `declarationsTransfer` reaches ScoreReveal and RulesDialog | medium | Pre-verified gap: removing either prop at `MatchPage.tsx:1820` or `:1854` fails no test. | patch |
| 15 | verification-gap | Quick Play auto-start never checks the `declarationsTransfer` argument | medium | Pre-verified gap: `seedQuickPlayRoomWithStake` leaves the flag false, and no auto-start test asserts `lastDeclarationsTransfer`. | patch |

## Design Notes

Proposed copy (en / mk), adjustable at review:
- Create-room field: "Declarations on a fall": "To the winners" (default) / "Voided"; mk "Најави при пад": "Кај победниците" / "Се поништуваат".
- Chip when off: "No declaration transfer"; mk "Без пренос на најави".
- Rules note when off: "On this table, the declarations and Belote of a team that goes down or is swept are voided instead of going to the other team."
- Fall subtitle when off: "Went down · card points to us/them"; mk "Падната · поените од карти за нас/нив".

## Verification

**Commands:**
- `make lint` -- expected: client and server clean
- `make test` -- expected: all pass (DB tests need `BELJOT_TEST_DB_URL` after `make dev`)
