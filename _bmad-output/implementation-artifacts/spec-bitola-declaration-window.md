---
title: 'Bitola declaration window and declaration cycle'
type: 'feature'
created: '2026-11-09'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '2698214459e26d502c911f0154f9fef713a4ef5d'
context:
  - '{project-root}/.claude/rules/game-rules.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-clockwise-play-direction.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Bitola declarations are welded to trick 1 and revealed at the start of trick 2. The owner wants a Bitola player to be able to declare before their card in any of tricks 1..N, with N chosen per room. A declaration then gives the other players one chance to declare too, and the contest resolves at once.

**Approach:** Bitola gets its own `DeclarationTiming`. A turn prompt reuses `AwaitingDeclaration`. Declaring opens Croatian's existing simultaneous `declaring` phase mid-trick, with the declarer already answered. When the phase closes, the contest resolves, play resumes at the declarer's card, and declarations are over for the hand. A room setting `declarationLastTrick` (1..3, default 3, Bitola with declarations on only) carries N.

**Decisions (owner, 2026-11-09):**
- Declarations happen once per hand. The first resolution closes declarations for the rest of the hand, whether it came in trick 1, 2 or 3.
- The cycle is Croatian's dialog and clock: every seat sees it at once, with the same 8 s client countdown and the same server window. A declaration inside it does not restart the timer, and nobody is asked twice.
- Bitola keeps today's declaration names. No new names and no per-variant meld label map.
- The pass button says "Continue" on a turn prompt before the last window trick, and "Skip" where passing is final: the turn prompt in trick N, and the 8 s cycle dialog every seat sees once someone has declared.
- One spec, no split.

## Boundaries & Constraints

**Always:**
- The engine branches on `VariantRules` fields only (`DeclarationTiming`, `DeclarationLastTrick`), never on the variant name.
- The client sends only `action:declare` or `action:skip_declare`. The server computes the melds from the seat's current hand with `detectDeclarations` and Bitola's no-overlap rule.
- A turn prompt appears only for the active seat, before its card, in a trick ≤ N, while declarations are still open and the seat holds a meld.
- The declarer is answered on entry to the cycle and is never asked in it. When the cycle closes, the contest resolves immediately, play resumes at the declarer's card with no new prompt, and no seat is prompted again that hand.
- Every seat sees the cycle dialog. A seat holding nothing sees it with Declare disabled and auto-skips, exactly as in Croatian.
- Before resolution, other seats learn only that the triggering seat declared. Only the winning team's declarations are revealed.
- The server rejects a declare or skip out of turn, after trick N, after the hand's resolution, a second time, or without a meld.
- `declarationLastTrick` is validated to 1..3 on create and persisted on rooms and matches. Legacy rows read as 3. Rooms that are not Bitola with declarations on store 3.
- Bitola bots declare at once (`botDelayMin`) whenever they hold a meld at any prompt, and skip otherwise.
- WS contract edits land in `events.go` and `wsEvents.ts` together. New strings go into all four locales. The rules text follows "less is more".

**Never:**
- Change Croatian behaviour, its tests, its rules text, or any meld detection, comparison or value.
- Leak another seat's prompt flag or pending melds through projection, and let a bot read hidden melds.
- Add a room-settings edit endpoint.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Pass through the window | No seat declares in tricks 1..N | A prompt appears only for meld holders. No prompt from trick N+1. No reveal. | Declare after N → `ErrWrongPhase` |
| Declare at turn | Active seat, trick ≤ N, holds a meld | `player_declared`, then phase `declaring` with the trick intact and the declarer answered | Not the active seat → `ErrNotYourTurn` |
| Cycle closes | All connected seats answered, or the window elapsed | Resolve, `declarations_resolved`, phase `playing`, same trick, the declarer plays with a fresh window and no prompt | — |
| Several declarers | Others declare inside the cycle | All their melds compete in the single resolution. The timer is not restarted. | Declare without a meld → `ErrDeclarationNotAvailable` |
| After resolution | Any later trick ≤ N | No prompts for anyone | Declare → `ErrWrongPhase` |
| Reduced meld | Seat played a run card earlier | The prompt and the server melds use the shorter run, or nothing | — |
| Legacy room | Row without the column | Reads 3 | Value outside 1..3 → 400 |
| Croatian or declarations off | Any | Behaviour identical to today | — |

</frozen-after-approval>

## Code Map

**Engine `server/internal/game/`**
- `types.go`: `DeclarationTimingDuringFirstTrick` has Bitola as its only user. Replace it with `DeclarationTimingTurnWindow`. Add `VariantRules.DeclarationLastTrick` (Bitola preset 3, Croatian 0).
- `state.go`: add a `NewGame` positional argument `declarationLastTrick` after `clockwise`. Add a public `GameState.DeclarationLastTrick`, refreshed in `rules_engine.go` `RefreshDerivedFlags`.
- `declarations.go` (or a new `declaration_window.go` for the Bitola-only helpers):
  - `checkDeclarationPrompt`: replace the trick-1 gate with `trick ≤ N && !DeclarationsResolved && holds meld`. Call it at every turn start in `playing`: `handlePickTrump`, the card advance in `handlePlayCard` and `finishCardPlay`, and the next leader after `resolveTrick`.
  - `checkDeclarationAnswerable` uses the same window.
  - `handleDeclare` in `playing` opens the cycle: `PhaseDeclaring`, trick kept, `ActivePlayerSeat` kept as the resume seat, declarer answered.
  - `closeDeclarationPhase` branches on timing. Croatian is verbatim. Turn-window resolves, returns to `playing`, keeps the trick and seat, then runs `stopAtTargetIfReached`.
  - Remove `resolveTrickWithDeclarations`'s trick-2 and hand-scoring blocks and `handleAnnounceBelot`'s trick-1 deferral. Only Bitola reached them, and nothing is pending during card play any more.
- `projection.go`: mask other seats' `AwaitingDeclaration`.
- `testfixtures/fixtures.go`: Bitola window factories.

**Session `server/internal/match/`**
- `live_match.go`:
  - `StartMatch` → `NewGame`.
  - Entering `PhaseDeclaring` from `playing` already arms `declarationAutoClose` and clears `TurnExpiresAt` (`:441-450`). `handleDeclarationTimeout` re-arms the resume seat.
  - `broadcastActionResult`: `player_declared` already fires only outside `PhaseDeclaring`. A `skip_declare` in `playing` sends its state frame to the actor only.
  - Persist the field on completed matches.
- `bot_driver.go`: `botThinkDelay` uses `botDelayMin` while `AwaitingDeclaration`. The `PhaseDeclaring` scheduling, `HasDeclarations` and memory work unchanged.
- `reconnect.go`, `reconcile.go`, `model.go`: record fields. `PhaseDeclaring` reconnect (fresh window) already exists.
- Bitola tests to update to the new semantics: `manager_test.go` `firstTrickStateAt` and its declare/skip tests, `auto_action_test.go:83`, `live_match_internal_test.go:320`, `stop_at_target_session_test.go:387/686`.

**Bot `server/internal/bot/`**: `bot.go` `Decide` already declares on `AwaitingDeclaration` and in `PhaseDeclaring`. There is no conceal heuristic. In `simulation_test.go`, the random baseline must not declare a meldless seat. Add a four-bot Bitola hand.

**Room `server/internal/room/` + `server/migrations/`**
- `000037_add_declaration_last_trick`: `SMALLINT NOT NULL DEFAULT 3` on `rooms` and `matches`.
- `model.go`, `handler.go`:
  - `CreateRoomRequest` gets `*int`. Validate with a new `apperr` like `ErrInvalidMinHonor`, then normalise like `declarationsTransfer` (`:445-458`).
  - `MatchStarter.StartMatch` and both call sites, `roomLifecyclePayload`, and the QuickPlay literal.
- `stale_room_adapter.go`.
- `user/handler.go` `MatchListItem`.
- Tests: copy `declarations_transfer_handler_test.go`.

**Client `client/src/`**
- Types: `shared/types/{matchTypes,apiTypes,wsEvents,wsEvents.schemas}.ts`, `shared/api/matches.ts`, the `event_match_state.json` golden.
- `features/match/MatchPage.tsx` + `components/DeclarationPrompt.tsx`:
  - "Continue" vs "Skip" on the turn prompt.
  - In `declaring` with `trickNumber ≥ 1`, show "{{name}} declared" (the active seat).
  - Check that the trick area renders mid-trick.
- `features/room/CreateRoomModal.tsx` + `lib/roomFromForm.ts`: a 1/2/3 control only when `variant==="bitola" && declarationsEnabled`, default 3.
- `shared/components/roomRules/roomRuleLines.ts`: `MatchRuleSource` gets `variant`, plus a new line (`notUsed` otherwise).
- `features/match/components/RulesDialog.tsx`: table note.
- `features/rules/content/{en,sr,hr,mk}.ts`: the Bitola melds steps.
- `shared/i18n/*.json`: new keys, and landing `b2`.

## Tasks & Acceptance

**Execution:**
- [x] `server/migrations/000037_*`, `server/internal/room/*`, `server/internal/apperr/errors.go`, `server/internal/user/handler.go` -- column, validation, normalisation, payloads, StartMatch args -- the setting feeds the engine.
- [x] `server/internal/game/*` -- window, turn prompt, mid-trick cycle entry and exit, projection mask -- the rule change, isolated behind `DeclarationTimingTurnWindow`.
- [x] `server/internal/game/*_test.go` -- through `ApplyAction`:
  - Pass through to N; declare in tricks 1, 2 and 3 as the 1st to 4th player.
  - The declarer is answered and not asked; several declarers.
  - Nobody is prompted after resolution; resume at the declarer with no prompt.
  - A reduced or lost meld; N = 1, 2 and 3; no declare after N.
  - Croatian unchanged.
- [x] `server/internal/match/*` + tests -- StartMatch wiring, private skip frame, bot delay, records, and timeout, disconnect and reconnect mid-prompt and mid-cycle.
- [x] `server/internal/bot/simulation_test.go` -- a four-bot Bitola hand with no stall.
- [x] `server/internal/ws/*` goldens + `client/src/shared/types/*` -- the contract in one commit.
- [x] `client/src/features/{match,room}`, `shared/components/roomRules`, and tests -- form control, labels, cycle header.
- [x] `client/src/features/rules/content/*`, `client/src/shared/i18n/*.json` -- Bitola rules in all four locales.
- [x] `.claude/rules/game-rules.md`, `CLAUDE.md` mirrored constants, `docs/bot/BOT_RULES.md`, `_bmad-output/planning-artifacts/architecture.md` -- keep the documented rules current.

**Acceptance Criteria:**
- Given a Bitola room created without the field, then it stores and plays with 3. Given Croatian or declarations off, then the form hides the control and the server stores 3.
- Given a full Bitola hand with four bots, when it is played, then it scores with no stall or panic.
- Given any seat's projected state, then it never shows another seat's prompt flag or unresolved melds.
- Given the Croatian suites, then they pass unchanged.

## Implementation Notes

- The Bitola cycle reuses `PhaseDeclaring`. Bitola-only helpers are in `server/internal/game/declaration_window.go`, and `closeDeclarationPhase` branches on `DeclarationTimingTurnWindow`. `checkDeclarationPrompt` runs at every turn start via `handlePickTrump`, `handlePlayCard`, `finishCardPlay` and `resolveTrickAndAdvance`, which replaces `resolveTrickWithDeclarations`.
- `NewGame` applies `declarationLastTrick` only to turn-window rules within 1..3. A Croatian state therefore carries `declarationLastTrick: 0` on the wire, while its match row stores 3 through `recordedDeclarationLastTrick`. The client ignores the value outside Bitola.
- If nobody declares in a Bitola hand, `DeclarationsResolved` stays false for the whole hand and no reveal fires.
- Some Croatian-named engine tests drove overlap through the removed trick-1 path. Their helpers now go through the dedicated phase, and their assertions are unchanged. The three tests of the removed Belote trick-1 deferral were replaced with window and cycle stop-at-target tests.
- Rules text: the Bitola item in the melds list carries the whole flow. The parity test allows a variant note only on `p` blocks and needs equal item counts per variant, so a Bitola-only block would have forced new Croatian text. The Croatian item's note, which describes Bitola, was updated in all four locales.
- Added beyond the Code Map: a "Declare until trick" row in the /rules room options, and doc touch-ups in `_bmad-output/project-context.md` and `docs/bot/BOT_RULES.md`.
- Verified:
  - `make lint` is clean.
  - Server `make test` passes.
  - Client vitest passes under the pinned Node 22: 183 files, 2583 tests. Node 24 shows unrelated environment failures.
  - `BELJOT_BOT_SIM=1` bot tests pass.
  - The DB-backed room, match, user, dbmigrate and migrations tests pass against a throwaway Postgres 16.

## Spec Change Log

## Review Triage Log

| # | Source | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|
| 1 | edge | A timed-out Bitola turn prompt sends `auto_action: timeout` to others | medium | `autoActionTypeFor` maps `skip_declare` and `sendAutoAction` masks it as `timeout` for others. A plain card-play timeout sends them nothing, so the toast reveals a meld holder. | patch |
| 2 | edge | `handlePlayCard` error order reveals the masked prompt | medium | `AwaitingDeclaration`/`PendingBelotSeat` are checked before `ActivePlayerSeat`, so a non-active seat gets `ErrActionRequired` instead of `ErrNotYourTurn` exactly while the active seat is prompted. | patch |
| 3 | blind+edge | The prompt opens over the previous trick's sweep | medium | `showDeclarationPrompt` lacks the `pendingResolvedTrick === null` gate that `canPlayNow` has. Trick 2–3 leaders are now prompted the moment a trick resolves. | patch |
| 4 | blind | The declarer's own dialog shows their own name | low | `declaredBy` is built from `activePlayerSeat` with no self check. Met on every human declaration. | patch |
| 5 | blind | `variant === "bitola"` at three client sites bypasses `variantRules.ts` | low | `variantRules.ts` is the client's single variant→behaviour mirror. `roomRuleLines` treats unknown variants as unused while `rulesForVariant` falls back to Bitola, so the two diverge. | patch |
| 6 | blind+edge | 1..3 options and the default are literals in `RoomOptions` and `CreateRoomModal` | low | `standard: "3"`, `other: "1 · 2"` and `["1","2","3"]` are not tied to `DEFAULT_DECLARATION_LAST_TRICK`. | patch |
| 7 | blind | Two tests are named for a rule that no longer exists (a trick-2 declare is illegal) | low | They pass only because `NewGameMidPlay` sets `DeclarationsResolved`. | patch |
| 8 | blind | Dead reveal-latch calls in the play_card and Belote branches | low | `resolveTrickAndAdvance` never resolves declarations, so `DeclarationsResolved` cannot flip on those actions. | patch |
| 9 | verification-gap | The reconciled abandoned row's window is untested | low | `TestReconciledMatchRecord_CarriesRuleFlags` never sets or reads `DeclarationLastTrick` (pre-verified by the layer). | patch |
| 10 | edge | `PlayerDeclaredPayload` docs say "during trick 1" | low | `events.go:108`. It now fires in any window trick. | patch |
| 11 | blind+edge | The architecture 12.6 paragraph says `handlePickTrump` is the only entry into declaring and Bitola resolves at trick 2 | low | `architecture.md:1046`. `epics.md` FR8(f) is the historical epic-12 record and is left as is. | patch |
| 12 | blind | `BOT_RULES.md` drops the deferral note without naming the new search gap | low | The search does not model declarations still to come in tricks 1..N. | patch |
| 13 | blind+edge | `CLAUDE.md` lists the default but not the 1..3 range | low | Real, but the fix edits an agent-context file. | defer |
| 14 | blind | `game-rules.md` lacks the new privacy rules | low | Real, but the fix edits an agent-context file. | defer |
| 15 | edge | A double-click on a turn prompt sends a second answer and gets an error toast | low | `sendOnce` guards only `simultaneous`. Same in the pre-change trick-1 prompt. | defer |
| 16 | blind | The rules-mirror golden lacks `declarationLastTrick` | false | The golden lists per-variant preset facts only. The room overrides (clockwise, stopAtTarget, declarationsEnabled/Transfer) are absent too, and `declarationTiming` is present and updated. | reject |
| 17 | blind | Pre-change Bitola match rows read 3 in history | low | Real, but the frozen intent chose "existing records default 3". | reject (intent) |
| 18 | blind | No session test for a Bitola cycle crossing stop-at-target | low | The session path is shared with Croatian (`TestStopAtTarget_DeclareCrossingEmitsRevealThenMatchEnd`, `…DeclarationTimeoutRunsHandleMatchEnd`). The engine test covers the cycle. Adding it means new tests. | reject |
| 19 | blind | `INVALID_DECLARATION_LAST_TRICK` is not localized | low | Reachable only by a crafted request; the form offers 1..3. The fix adds a mapping plus four strings. | reject |
| 20 | blind+edge | A Croatian overlap test now runs a synthetic rules mix | false | Croatia never prompts during play (`declarationWindowOpen` needs a turn window, and `TestDeclarationWindowLeavesCroatiaAlone` pins it). The replaced row drove a trick-1 path the change removed for every variant. | reject |
| 21 | blind | The spec is missing from the change set | false | It is untracked in the tree and excluded from the review diff by the review protocol. | reject |
| 22 | blind | Landing `b2` states a fixed limit | false | "Up to trick three" is the true upper bound of the 1–3 setting. | reject |
| 23 | blind | sr/hr `declaredBy` repeats the banner wording | low | "ima zvanje" is gender-neutral and accurate. A sr/hr past tense would be gendered. | reject |

## Design Notes

Defaults chosen by the agent and stated to the owner:
- **Turn-prompt leak:** others never see `awaitingDeclaration`, and a skip's frame goes only to the actor, so others see only the normal move clock.
- **Hand contents:** the current hand only; the card on the table does not count.
- **Timers:** the turn prompt shares the per-move clock, and on expiry it is skipped and the card auto-played, counted as a miss, as today. The cycle uses Croatian's window: no miss counted, no pause, a fresh window on reconnect. There is no bot takeover; an absent seat forfeits via the reconnect timeout.
- **Other rules unchanged:** the trick-1-leader tiebreak, no trick-win requirement, Belote, the last-trick bonus, Capot and "declarations on a fall". Stop-at-target is checked after the resolution and after Belote with no deferral.
- **Accepted, as in Croatian:** meldless seats auto-skip at once, so a cycle that closes almost instantly hints that nobody else held a meld.

## Verification

**Commands:**
- `make lint && make test` -- expected: clean.
- `cd server && BELJOT_BOT_SIM=1 go test -tags nodynamic ./internal/bot/ -count=3` -- expected: pass.
- `cd server && go test -tags nodynamic ./internal/game/ ./internal/match/ -run 'Croatian|DeclarationPhase' -count=1` -- expected: unchanged pass.
