---
title: 'Room rule: clockwise direction of play'
type: 'feature'
created: '2026-10-25'
status: 'done'
baseline_commit: 'a4dd26f16e0f7831e5bd8410870396203d34dd74'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/.claude/rules/game-rules.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-declaration-transfer-rule.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Play always moves counter-clockwise (to the right), but some tables play clockwise, and a room cannot express that.

**Approach:** Add an advanced room rule, "Direction of play", that defaults to counter-clockwise (today's behavior). When it is set to clockwise, the whole game moves to the left: the deal, the bidding, the trick play and the passing of the deal.

## Boundaries & Constraints

**Always:**
- Default is counter-clockwise for new rooms, Quick Play, and rooms or matches that existed before the migration. The flag is a `clockwise` bool, and its zero value means today's behavior.
- The engine reads `state.Rules.Clockwise` through one seat-step helper and never hard-codes `+1`. Seats stay where they are on screen: clockwise only changes who acts next, so seat+3 (on the viewer's left) follows each seat.
- Everything ordered by play follows the flag: the first bidder, the deal order, Bitola round 2, the reshuffle and between-hand dealer moves, the trick-1 leader, the turn advance (including after a Belote prompt), the declaration tiebreak distance, and the bot's `seatsYetToPlay`.
- Fixed for the whole match, shown on every surface that shows `stopAtTarget`, and stored on the match row, including abandoned and reconciled rows.

**Never:** Change partner (`+2`) or team (`% 2`) logic, seat placement on screen, scoring, or bidding strength. Add ranked or season restrictions. Change behavior when the flag is off.

## I/O & Edge-Case Matrix

Dealer seat 0 in every row.

| Scenario | Counter-clockwise (flag off) | Clockwise (flag on) |
|----------|------------------------------|---------------------|
| First bidder; deal packet order | 1; 1,2,3,0 | 3; 3,2,1,0 |
| Bitola round 2 first bidder; second-deal order | 1; 1,2,3,0 | 3; 3,2,1,0 |
| Bitola 8 passes → reshuffle | dealer 1, first bidder 2 | dealer 3, first bidder 2 |
| Croatian: three passes | dealer must pick | dealer must pick (after 3,2,1) |
| Trick-1 leader; next after the leader plays | 1; 2 | 3; 2 |
| Next hand dealer | 1 | 3 |
| Declaration full tie between seats 1 and 2 | seat 1 wins (leader 1) | seat 2 wins (leader 3) |
| Bot `seatsYetToPlay` for seat 2 after the trick-1 leader played | 3, 0 | 1, 0 |
| Client deal animation | packets to 1,2,3,0 | packets to 3,2,1,0 |

</frozen-after-approval>

## Code Map

- `server/internal/game/types.go` `VariantRules` -- add `Clockwise`; put the seat-step helpers here (next seat, step n, play-order distance).
- Engine `+1` sites to route through the helper:
  - `state.go`: 164, 210, 216, 238, 245
  - `bidding.go`: 49, 55, 101, 129, 174, 181
  - `scoring.go`: 164, 206
  - `declarations.go`: 273, 306, 477, 530, and the tiebreak distance at 221-223
  - `playing.go`: 85
  The trick winner (`validation.go`) reads `TrickCard.PlayerSeat` and is already direction-safe.
- `game/state.go` `NewGame` (positional flags; add `clockwise bool` after `declarationsTransfer`), the wire mirror next to `declarationsTransfer`, and `rules_engine.go` `RefreshDerivedFlags`.
- `game/testfixtures/fixtures.go` -- add a `WithClockwise` option next to `WithoutDeclarationsTransfer`.
- `server/internal/bot/view.go` and `bot.go:710-718` `seatsYetToPlay` (five callers) -- add `View.Clockwise`; set it in `match/bot_driver.go` `buildBotView`.
- Plumbing: follow `declarationsTransfer` through each of these:
  - `match/live_match.go`: `StartMatch` and its persistence at about line 1004
  - `match/model.go`
  - `match/reconnect.go` and `match/reconcile.go`
  - `room/{model,handler,lobby_disconnect,stale_room_adapter}.go`, including both `StartMatch` call sites, the QuickPlay literal and the payload maps
  - `user/handler.go` `MatchListItem`
- `server/migrations/` -- the latest is 000031.
- `server/internal/ws/testdata/events/event_match_state.json` golden; `room/lobby_disconnect_payload_test.go` parity.
- Client types:
  - `shared/types/apiTypes.ts`: Room optional, CreateRoomRequest required
  - `wsEvents.ts`: room payloads
  - `wsEvents.schemas.ts`: strict object, so the new field must be added
  - `matchTypes.ts`: MatchState
  - `shared/api/matches.ts`: MatchListItem
- `features/match/lib/dealSchedule.ts:52` and `useDealController.ts:273` -- the only direction-dependent client code. The tests at `dealSchedule.test.ts:35` lock in the current order.
- UI (follow `declarationsTransfer` in each):
  - `CreateRoomModal.tsx`: state, `advancedChangedCount`, payload, reset, a Segmented field, and the PreviewCard chip
  - `RoomCard.tsx:146`
  - `RoomPage.tsx:1011`
  - `ScorePanel.tsx:325`, with the meta guard at 283 and the prop from `MatchPage.tsx`
  - `MatchStatsCard.tsx:536`
  - `RulesDialog.tsx:291`: a `TableRuleNote`
- Leave unchanged:
  - `features/rules/content/*.ts`: it describes the usual rightward play. The table note covers the exception.
  - Seat geometry: `trickLayout.ts` and `MatchPage.tsx` `SEAT_POSITIONS`.

## Tasks & Acceptance

**Execution:**
- [x] `server/migrations/000032_add_clockwise.{up,down}.sql` -- add `clockwise BOOLEAN NOT NULL DEFAULT FALSE` to `rooms` and `matches`; the down migration drops both.
- [x] `server/internal/game/{types,state,rules_engine,bidding,scoring,declarations,playing}.go` -- add the flag, the helper, the positional `NewGame` argument, the wire mirror `clockwise`, and replace every engine site in the Code Map.
- [x] `server/internal/game/clockwise_test.go` plus the fixture -- add a table test of every engine row in the I/O matrix, under both directions.
- [x] `server/internal/bot/{view,bot}.go`, `match/bot_driver.go` -- make `seatsYetToPlay` direction-aware; test the matrix row.
- [x] Server plumbing (match, room, user, reconnect, reconcile, stale adapter) -- thread and persist the flag. The create request uses a plain `bool` (an omitted field means false), and Quick Play is false.
- [x] Server tests -- fix the positional-argument fallout and update the golden and the parity test. Add a room handler test (an omitted field means false; an explicit true persists and reaches the payloads) and a session test (the flag reaches the engine and the bot view).
- [x] Client types (Code Map) -- add `clockwise`.
- [x] `dealSchedule.ts`, `useDealController.ts` -- the deal schedule takes the direction from the match state; extend the tests.
- [x] `CreateRoomModal.tsx` and the chip and note surfaces -- show the rule only when it is clockwise.
- [x] i18n, all four locales -- add the new keys and update the advanced summary.
- [x] Client tests -- CreateRoomModal (sends false by default and true when chosen), RoomCard, RoomPage, ScorePanel, RulesDialog, MatchStatsCard, MatchPage prop wiring, and the MatchState fixtures.
- [x] `.claude/rules/game-rules.md` -- under "Turn order", document the rule and the helper.

**Acceptance Criteria:**
- Given a create-room request without the field, when the match starts, then `clockwise` is false on the wire and every existing engine, bot and deal-schedule test passes unchanged.
- Given the owner picks clockwise, when players see the lobby card, room page, score panel, rules dialog and the finished match's history, then each shows clockwise play.
- Given a clockwise match, when a hand is dealt, then the on-screen turn indicator moves to the player on each seat's left, and the deal animation sweeps the same way.

## Implementation Notes

- `game.StepSeat(seat, n, clockwise)` in `types.go` is the only place that reads the direction. `VariantRules.NextSeat`, `SeatAfter` and `PlayDistance` wrap it for the engine, and the bot's `seatsYetToPlay` calls it with `View.Clockwise`. `resolveDeclarations` and `declarationBeats` now take the rules so the tiebreak distance follows the direction.
- `testfixtures.WithClockwise` sets both flags and re-seats the turn: a bidding fixture (either round) by its pass count, a declaring fixture or an empty trick-1 lead on the dealer's clockwise neighbour. Mid-trick fixtures keep their seats.
- Tests: the engine rows of the matrix are in `game/clockwise_test.go`. The deal packet order needs the unexported `dealCards`, so it sits in `game/clockwise_internal_test.go` with the helper tests. The bot row is `bot/seats_yet_to_play_internal_test.go`, and the session test is `match/clockwise_session_internal_test.go` because `buildBotView` is unexported. The existing rule-flag tests for completed, abandoned and reconciled match rows, match history and the lobby disconnect payload now carry `clockwise` too.
- The create-room field sits under "Match end", a Segmented with `RotateCcw` / `RotateCw` icons. Every chip and note renders only on `clockwise === true`.
- DB tests ran against a throwaway `postgres:16-alpine` on port 5544 (migrate up, down 1, up again), not the dev database.
- Follow-up commit, at the owner's request, fixed the three deferred triage items (rows 1, 9 and 14):
  - The direction rule is documented in `.claude/rules/game-rules.md` (Croatian bidding), `_bmad-output/project-context.md` (the turn-order line and Croatian bidding), and the root CLAUDE.md list of constants mirrored by hand (the seat step).
  - Every lobby-wide room broadcast now goes through one package-level `roomLifecyclePayload(repo, room)`: `room_created` from CreateRoom and QuickPlay, `room_updated` from the room handler, and the lobby-disconnect `room_updated`. QuickPlay's `room_created` used to omit `players`, `createdAt`, `updatedAt`, the owner fields and the privacy and honor gates. The disconnect frame used to omit `players`, `coinBuyIn`, `isPrivate`, `minHonor`, `allowNewPlayers` and `isQuickPlay`, which left the lobby's seat chips stale after a lobby disconnect.

## Spec Change Log

## Review Triage Log

| # | Source | Finding | Verdict | Evidence | Route |
|---|--------|---------|---------|----------|-------|
| 1 | blind, edge-case | Agent docs still say Croatian bidding starts to the dealer's right, and project-context says the next seat is `(seat + 1) % 4` | medium | `.claude/rules/game-rules.md` Croatian bullet and `_bmad-output/project-context.md` ~122/~126 state it unconditionally; an agent could hard-code `+1`. | defer |
| 2 | blind, edge-case | `WithClockwise` re-seats only the opening bid and lead, so mid-bidding and declaring fixtures stay counter-clockwise | low | `fixtures.go` checks only `BiddingPassCount == 0` and an empty trick-1 lead. `WithClockwise(NewGameMidBidding(2))` leaves seat 3 active instead of 1. A test helper that is likely to be reused for the next clockwise test. | patch |
| 3 | blind | Bot view reads the wire mirror `gs.Clockwise` while the engine reads `gs.Rules.Clockwise` | low | Production keeps them in sync (`NewGame`, `RefreshDerivedFlags`), but `SetGameStateForTest` does not refresh mirrors, so the bot can disagree with the engine in tests. One-token fix. | patch |
| 4 | blind | No end-to-end clockwise bot match, and no test that the `event:match_state` frame carries `clockwise` | false | `ProjectForSeat` clones the whole state, so the frame keeps the field. The ws golden covers its wire name, the session test covers the flag reaching the engine and bot view, and each engine order has its own test. | rejected |
| 5 | blind | Bot direction coverage is one `seatsYetToPlay` case, with no decision-level test | low | The helper is one `StepSeat` call and the matrix row is covered. A decision-level test is new test scaffolding for a defect nobody showed. | rejected |
| 6 | blind | Deleting `clockwise: cur.clockwise` in the `candidateSecond` detection fails no test | false | `DetectedDeal.clockwise` is a required field, so deleting the line fails `tsc` in `make lint`. | rejected |
| 7 | blind | `buildDealSchedule`'s `clockwise = false` default lets a future caller forget the direction | low | It has one production caller, which passes the flag. Making the argument required would touch every test call for a hypothetical caller. | rejected |
| 8 | blind | ScorePanel's separator guard repeats every earlier condition | low | Developer-only; the existing `declarationsTransfer` pattern, already rejected in its own triage. The fix is a refactor. | rejected |
| 9 | blind | Room broadcast payload is hand-built in three maps; the QuickPlay map has drifted (no `createdAt` etc.) | maybe-false | Pre-existing drift at `room/handler.go` ~3104. It needs a check of whether Quick Play rooms reach a lobby list that renders `createdAt`; medium if so. | defer |
| 10 | blind | The direction hint never says what either option does, and en says clockwise where sr/hr/mk say left/right | low | `directionHint` reads "Who deals, bids and plays next" in every locale. The fix is a string change that bridges both vocabularies. | patch |
| 11 | blind | `PlayDistance`'s doc comment is garbled | low | `types.go`: "how many turns after from the seat to acts". | patch |
| 12 | blind | No real-database test for `matches.clockwise` | low | The match package has no database tests for any rule flag. The column is a plain additive ALTER, and the struct has no GORM `default` tag. | rejected |
| 13 | blind | Clockwise deal-schedule test names say "from the seat before the dealer" | low | Seat 3 comes after the dealer in clockwise play (the dealer's left); the existing counter-clockwise name "from the dealer's left" is also backwards. Rename only. | patch |
| 14 | blind | CLAUDE.md's list of hand-mirrored constants lacks the seat step | low | `dealSchedule.ts` `seatAfter` hand-copies `game.StepSeat`. The fix edits an agent-context file. | defer |

## Design Notes

Proposed copy (en / mk), adjustable at review; sr/hr follow the same pattern:
- Create-room field: "Direction of play": "Counter-clockwise" (default) / "Clockwise"; mk "Насока на играта": "Надесно" / "Налево". Hint: "Who deals, bids and plays next." / „Кој дели, лицитира и игра следен.“
- Chip when clockwise: "Clockwise" (icon `RotateCw`); mk „Игра налево“.
- Rules note: "On this table, play moves to the left (clockwise): the deal, the bidding, the tricks and the next dealer all go left. The chapters below describe the usual rightward play."

## Verification

**Commands:**
- `make lint` -- expected: client and server clean
- `make test` -- expected: all pass. Run client tests on the pinned Node 22 (`mise exec --`). DB tests need `BELJOT_TEST_DB_URL` after `make dev`.
