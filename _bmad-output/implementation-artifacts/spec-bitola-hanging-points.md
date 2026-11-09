---
title: 'Bitola hanging points (виси) tie rule'
type: 'feature'
created: '2026-11-09'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '9187c886ca5c13548ea26fd1c8a6eb1eb84fd8e5'
context:
  - '{project-root}/.claude/rules/game-rules.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Bitola settles a tied hand like Croatian (a tie is a fall and the opponents take everything), but the real Bitola rule hangs the taker's half over to the next hand. `VariantRules.TieRule` already names `TieRuleHangingPoints` for Bitola, but nothing reads it.

**Approach:** Add one pure settlement function in `internal/game`. It takes the hand tally and the current hanging entry and returns the per-team awards, the contract outcome, the new hanging entry and the payout. `scoreHand` and the bot search both call it. Store the single nullable hanging entry on `GameState`, put the tie and payout on `HandScore`, the `hand_scored` event and the `hand_results` rows, render them in the Bitola UI, and document the rule in the four locales.

## Boundaries & Constraints

**Always:**
- Bitola tie (caller total == opponents' total, declarations and Belote counted, no Capot): the caller banks 0, the opponents bank their own total, and the caller's total becomes the hanging entry `{amount, team}`.
- Paying out the hanging points H on the next scored hand: H goes to the team that takes that hand. That is the caller if it strictly passes, the Capot team on a Capot, and the caller's opponents on a fall or a tie. The owner never decides the payout. H is fully settled that hand, and only a tie creates a new entry.
- At most one entry. `GameState.HangingPoints` is `*HangingPoints` and goes out to every seat unmasked.
- `HandScore` totals include any H paid that hand, so per-hand totals still add up to the match score.
- Gate on `Rules.TieRule`, never on `state.Variant`. Croatian output is bit-for-bit unchanged.
- One implementation. The bot's `search.SettleTotals` delegates to the game function, and the client renders server fields without recomputing anything.
- WS contract: `events.go`, the goldens, `wsEvents.ts` and `wsEvents.schemas.ts` change together. Every UI string goes into all four locales.

**Never:**
- No live-state database persistence. Live matches are in memory, and a restart abandons them, as it does today.
- No bidding-table retune or other bot strategy redesign.
- Nothing extra in the Croatian UI or rules.
- Don't touch `docs/bot/lab/`. It's a research snapshot outside the Go module.

## I/O & Edge-Case Matrix

A = owner of H, B = the other team. The prior entry is H = 81.

| Scenario | Hand | Expected |
|---|---|---|
| Tie, no H | B calls, 81:81 | A +81, B 0, entry {81, B} |
| A calls, passes | A 100 : B 62 | A +100+81, B +62, no entry |
| A calls, falls | A 60 : B 102 | B +162+81, no entry |
| A calls, ties | 81:81 | B +81+81, A 0, entry {81, A} |
| B calls, passes | B 100 : A 62 | B +100+81, A +62, no entry |
| B calls, falls | B 60 : A 102 | A +162+81, no entry |
| B calls, ties | 81:81 | A +81+81, B 0, entry {81, B} |
| Capot with H | either side | Capot team +whole hand+81, no entry |
| Croatian tie | 81:81 | opponents +162, no entry (unchanged) |
| Reshuffle with H | all pass in round 2 | H stays until the next scored hand |

## Decisions (owner, 2026-11-09)

1. A Bitola tie is its own outcome, labelled as a tie that hangs (`failedContract=false`; the overlay reads "Tie · our 81 hangs"). Croatian ties stay falls.
2. Match end: the opponents' immediate half can cross the target and end the match. H paid that hand counts toward the target in the same end-of-hand check. H still hanging at match end (target, all-eight-trumps deal, surrender, abandonment) is discarded and counts toward nothing (score, XP, coins, SP).
3. Stop at target: running totals exclude H, and a mid-hand stop discards it.
4. Only reconnects carry H (`match_state`), plus the `hand_results` history. No live-state persistence: a restart abandons a match, as before.
5. Bots follow the same rules as humans: the search values a hand by exactly what `SettleHand` writes to the score that hand. A new entry counts for nobody until it is paid. The bidding table is unchanged.
6. Rules text goes on the rules page and in the in-match dialog only: scoring item 1 is split into a Bitola and a Croatian item. The landing page and the create-room hint stay unchanged.
7. Wording: "the next hand's taker gets it by winning that hand, otherwise their opponents do".
8. Terms: en "hanging points"/"hangs", mk "виси"/"висечки поени", sr "visi"/"viseći poeni", hr "visi"/"viseći bodovi".
9. UI: a score-panel chip while H is outstanding, an overlay tie subtitle and payout row, and match-history hand notes. All Bitola-only by data.
10. One spec, over the token guideline, by owner choice.

</frozen-after-approval>

## Code Map

- `server/internal/game/scoring.go` -- `scoreHand` (`:57` `contractingTotal <= opposingTotal`), `wholeHandAward`, `teamRunningTotal`, `stopAtTargetIfReached`, `determineMatchWinner`. Replace the inline settle with a call to the new pure function and keep the bonus and target code.
- `server/internal/game/types.go:107-178` -- `TieRule` and the presets already exist. Add the `HangingPoints` type next to them.
- `server/internal/game/state.go:44-60,64-123` -- `HandScore` and `GameState`. Add `HangingPoints *HangingPoints` (public) and `HandScore.HangingCreated`/`HangingPaid`.
- `server/internal/game/bidding.go:188` -- `cloneGameState` must deep-copy the new pointer.
- `server/internal/game/testfixtures/fixtures.go:282` -- `NewGameLastTrick` is Bitola. Existing tie tests in `scoring_test.go:109-149` assert the Croatian rule and must move to Croatian rules.
- `server/internal/match/live_match.go:719-742` -- `hand_scored` map. `:920-961` `bufferHandResultIfScored`.
- `server/internal/match/hand_result.go`, `migrations/000038_*` -- four nullable hanging columns on `hand_results`.
- `server/internal/user/handler.go:161-177,895-912` -- `MatchHandView` DTO for match history.
- `server/internal/ws/events_contract_test.go`, `ws/testdata/events/*.json` -- goldens. Regenerate with `UPDATE_GOLDENS=1`.
- `server/internal/bot/search/settle.go` -- duplicates `scoreHand`. Delegate it, and add `TieRule` and `Hanging` to `Settlement`. `settle_test.go:52-66,297-323,391-397` and `pimc_internal_test.go:24` encode tie = fall.
- `server/internal/bot/view.go`, `search_bridge.go:96-101,331-340`, `match/bot_driver.go:295-341`, `bot/bot_test.go:19`, `match/export_test.go:336` -- add View fields and plumbing. `couldDecideThisHand` adds H to its bound.
- `client/src/shared/types/{matchTypes.ts,wsEvents.ts,wsEvents.schemas.ts}`, `shared/api/matches.ts` -- types and strict Zod schemas.
- `client/src/features/match/components/{ScorePanel.tsx,ScoreReveal.tsx}`, `MatchPage.tsx:580-595,1213-1229` -- UI and the reconnect rebuild.
- `client/src/shared/components/matchStats/MatchStatsCard.tsx:128-340` -- hand notes.
- `client/src/features/rules/content/{en,mk,sr,hr}.ts:167-191` -- scoring list. The parity test requires paired variant-scoped items.
- `.claude/rules/game-rules.md`, `_bmad-output/project-context.md:130`, `docs/bot/BOT_RULES.md:297` -- remove the "stand-in" wording.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/game/{types,state,scoring,bidding}.go` -- `HangingPoints`, `HandTally`, `HandSettlement`, `SettleHand`, wiring, clone -- the single pure rule.
- [x] `server/internal/game/hanging_points_test.go` (+ fix `scoring_test.go`) -- every matrix row, chains tie→tie→tie, tie→pass, tie→fall, game-end cases, Croatian regression, projection and clone.
- [x] `server/migrations/000038_add_hand_results_hanging_points.{up,down}.sql`, `match/hand_result.go`, `live_match.go`, `user/handler.go` -- persist and serve, plus a DB round-trip test.
- [x] `server/internal/ws/*` goldens and the client types and schemas -- the contract.
- [x] `server/internal/bot/**`, `match/bot_driver.go` -- delegate settle, plumb the View, add tests with a Croatian regression.
- [x] Client UI and i18n (four locales) and tests.
- [x] Rules content in four locales, plus dev docs.

**Acceptance Criteria:**
- Given a Croatian match, when any hand is scored, then `TeamScores`, `HandScore` and the bot `Settle` output equal the pre-change output.
- Given H is outstanding, when a client reconnects, then `match_state` carries `hangingPoints`.
- Given a finished Bitola match with ties, when its history is fetched, then each hand shows its tie and payout.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| # | Finding (layer) | Verdict | Evidence | Route |
|---|---|---|---|---|
| 1 | All-eight-trumps discard untested (verification-gap, blind) | medium | No test reaches `handlePickTrump`'s instant win with an entry; deleting the nil-out passes every test | patch |
| 2 | Abandonment keeps `HangingPoints` (verification-gap, edge-case) | medium | `abandonMatchLocked` (`reconnect.go:430`) sets match end without clearing; final `match_state` frames carry the entry, so the chip stays | patch |
| 3 | Career best hand counts paid H (verification-gap) | medium | `gorm_repo.go:265` ranks `team_*_hand_total`, which now include an earlier hand's payout | patch |
| 4 | Tie ending the match labelled "hangs" (edge-case, blind) | medium | `handleScoreRevealContinue` shows `ScoreReveal` before `match_result`; history's final tie row shows "hangs" with no payout | patch |
| 5 | Stop-at-target reach ignores H (edge-case, blind) | medium | `withinStopAtTargetReach` uses card points + 10 only; a payout-only target crossing gets no race | patch |
| 6 | Race test settles Bitola as Croatian (edge-case) | low | `race_crossing_test.go:36` builds `Settlement` without `TieRule`/`Hanging` | patch |
| 7 | Rules omit discard and target interplay (blind) | low | True omission, but the owner approved the wording (decision 7) and asked for "less is more" | reject (intent) |
| 8 | mk "виси нашите N" agreement (blind) | low | Owner-specified term; flagged in the final report for owner review | reject |
| 9 | Tie golden unreachable (blind) | low | B 91 cards + 10 = 101 vs A 71 + 20 = 91 is a fall | patch |
| 10 | Impossible fixtures (blind) | low | `MatchPage.test.tsx:1830` caller team 1 with entry team 0; `gorm_repo_test.go:590` payout hand at 162/162 | patch |
| 11 | `team` is owner or recipient by context (blind) | low | Documented on every type; fix is a cross-contract rename | reject |
| 12 | Migration lacks CHECKs (blind) | false | The only writer (`hangingColumns`) sets both columns or neither; a half-set row is unreachable | reject |
| 13 | H under "Declarations on a fall: Off" (blind) | low | H is a fixed amount per owner spec ("H is written to B"); the rules file doesn't say so | defer |
| 14 | `CallerMade` doc stale (blind) | low | Doc names fall/Capot only; a Bitola tie also returns false | patch |
| 15 | Tie column doesn't add up in overlay (blind) | false | A fall already shows caller rows then 0, with the subtitle explaining; same shape | reject |

## Design Notes

Shape of the pure function (names may vary, the contract may not):

```go
type HandTally struct { CardPoints, DeclPoints [2]int; CallerTeam int; Capot bool; CapotTeam int; DeclarationsTransfer bool; TieRule TieRule }
type HandSettlement struct { Awarded [2]int; FailedContract bool; Hanging, Paid *HangingPoints }
func SettleHand(t HandTally, hanging *HangingPoints) HandSettlement
```

- `CardPoints` already include the last-trick or Capot bonus, and `DeclPoints` include Belote. Precedence stays: Capot, then pass (`caller > opp`), then the Bitola tie, then the fall (whole hand via today's `wholeHandAward` logic). The taker of H is the Capot team, else the caller on a pass, else the opponents.
- `FailedContract` keeps today's `caller <= opp` (even on a Capot) under `TieRuleAllToOpponents`. Under `TieRuleHangingPoints` it is `caller < opp`. A Bitola tie is marked by `HangingCreated != nil`.
- `HandScore.TeamXHandTotal` = `Awarded` (H included). `MatchStatsCard` derives an "unfinished hand" row from the final score minus the sum of hand totals; that stays right because a discarded H was never written.
- Bot: `SettleTotals` keeps its card split and Capot detection, then calls `game.SettleHand`. `View` gets `HangingPoints` and `TieRule`. `CallerMade` keeps comparing awarded totals. The engine-parity tests must still agree, since both sides now use one function.
- Client: render hanging UI only when the data is present. No client mirror of `TieRule`, and no client-side settlement maths.
- Rules: the Bitola item's `otherVariantNote` reads "Croatian: a tie is a fall too."; the Croatian item's reads "Bitola: on a tie the taker's half hangs until the next hand." Translate both, and the decided EN Bitola text, into mk/sr/hr with the decided terms and each locale's existing words (mk "земач", "паѓање", "рака"; sr/hr "Ko/Tko zove adut", "pad", "ruka", poeni/bodovi).
- A comment-policy hook rejects comments other than short doc comments above declarations.

## Verification

**Commands:**
- `make lint && make test` -- green
- `BELJOT_BOT_SIM=1 go test -tags nodynamic ./internal/bot/` (in `server/`) -- gates pass
- `BELJOT_TEST_DB_URL=… go test -tags nodynamic ./internal/match/ ./internal/user/` -- DB round-trip green
