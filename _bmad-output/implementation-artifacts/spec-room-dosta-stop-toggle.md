---
title: 'Room option: stop at the target ("dosta") or finish the hand'
type: 'feature'
created: '2026-08-24'
status: 'done'
review_loop_iteration: 2
baseline_commit: '0bb76c62587e1afff9c0219dac0284a3db4d8a8a'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** A match always plays the current hand to its end before the 1001/501 target is checked, so a team that crossed at trick 4 keeps playing and the final score sails above the target. Many tables play the opposite way — "dosta" (enough): the moment a team reaches the target the match is over, hand unfinished. There is no way to configure it, and joiners cannot see which kind of table they are sitting at.

**Approach:** Add a room-level `stopAtTarget` setting (default OFF = finish the hand, today's behaviour) chosen in the create-room dialog, persisted on `rooms`, carried into the resolved `VariantRules` at game init, and read by the engine so that when it is ON the match ends the instant a team's running total (match score + points won so far this hand) reaches the target. Automatic — no button and no player action. Applies to **both** variants. Surface it as a chip on the lobby card, the waiting room, and the in-match scoreboard, rendered only when it is ON.

## Boundaries & Constraints

**Always:**
- D-VAR-1 holds: the switch is a named field on `VariantRules`, resolved once at game init and read as config. No engine file compares `state.Variant` or reads a room struct. Both presets populate it `false`; the per-room choice is layered over the preset inside `NewGame`, the same seam `DeclarationsEnabled` uses.
- **Running total = `TeamScores[t] + HandPoints[t] + DeclarationPoints[t] + BelotPoints[t]`.** No last-trick +10 and no Capot +100 — the hand never completed, so neither bonus was earned. No failed-hand transfer either: the taker's "strictly more" test needs a finished hand.
- **Exactly three checkpoints, because those are the only three places mid-hand points are awarded**: a trick resolving (`playing.go:127`), the declaration contest resolving (`declarations.go:702`), a Belote announcement (`declarations.go:598`). Checked immediately after each, in the same `ApplyAction` call.
- **One deferral, and only one: the Belote checkpoint does not fire while a Bitola trick-1 meld contest is still open** — the condition is `TrickNumber == 1 && !DeclarationsResolved`. Bitola declares *during* trick 1 and resolves the contest only when the trick completes, so a +20 crossing mid-trick-1 would end the match before any meld was converted to points, silently discarding a declared quarte. The +20 is still banked; the stop is simply evaluated at the trick-1 resolution instead, where the contest has settled and the running total is complete. At most two or three further cards are played. The condition needs no variant check and no new state: Croatian has already resolved its dedicated phase by trick 1, and a declarations-off room has `DeclarationsResolved` seeded true, so neither defers. (Added 2026-08-24, review iteration 2, owner decision.)
- At the stop the engine commits the running totals into `TeamScores`, sets `WinnerTeam` via the existing `determineMatchWinner`, sets `PhaseMatchEnd`, and nils `LastHandResult`, `TurnExpiresAt`/`TurnTimeRemaining`, `AwaitingDeclaration`, `PendingBelotSeat` and `SurrenderProposerSeat`. **It leaves `HandPoints`/`DeclarationPoints`/`BelotPoints` populated** — the same state shape `scoreHand` already produces at a normal match end, which also banks a hand's points into `TeamScores` without clearing the accumulators. (Amended 2026-08-24, review iteration 1: the earlier "zeroes the three accumulators" rule broke the declaration reveal, whose `winnerTeam` is derived from `DeclarationPoints[team] > 0` at `live_match.go:1178-1184`. It was there to stop the scoreboard's "+N this hand" bar double-counting banked points — a display that already behaves exactly this way at every normal match end, so the rule solved nothing and cost a defect.)
- **`LastHandResult = nil` is load-bearing, not tidiness.** `handJustScored` (`live_match.go:885`) and `bufferHandResultIfScored` (`live_match.go:1186`) both gate on it being non-nil plus a transition into `match_end`; `startNewHand` deliberately never clears it. Leaving a hand-3 stop holding hand 2's result would emit a false `event:hand_scored` and write hand 2's numbers into the final `hand_results` row. The aborted hand deliberately gets **no** `hand_results` row (`CreateWithHands` handles zero hands).
- **Do not touch `ActivePlayerSeat` or trick state at the stop.** `trickResolvedWinnerSeat` (`live_match.go:825-834`) reads `ActivePlayerSeat` as the trick-1..7 winner fallback; overwriting it broadcasts the wrong winner on the final trick.
- Trick 8 is **not** a checkpoint. A completed hand always goes through `scoreHand` with its bonuses and its normal target check, in both settings. `stopAtTarget` can only ever shorten a match, never change how a finished hand scores.
- OFF must be byte-identical to today on the wire, and every pre-existing Bitola and Croatian test passes unchanged.
- `rooms.stop_at_target` carries **no GORM `default` tag** (the `AllowNewPlayers`/`DeclarationsEnabled` trap). Both hand-built `&Room{}` sites set it explicitly.
- The new plumbing parameters on `NewGame` and `StartMatch` are **positional**, matching the `DeclarationsEnabled` precedent.
- i18n lands in all four locales (en, mk, hr, sr) in the same commit. English says "Stop at target"; mk, hr and sr use their own native word for "enough" (the mk form in Cyrillic), never a calque of the English. Read and write the locale files with the Read/Edit tools, never through the cp1251 Bash console.

**Ask First:**
- Any change to `scoreHand`'s bonus application, the failed-hand rule, or the 162-point hand total.
- Making the stop player-callable (a "Dosta!" button), or persisting the setting on `matches` / surfacing it in match history — both deliberately out of scope.

**Never:**
- No new WebSocket event and no new client-to-server action. The state snapshot carries the flag.
- Do not fabricate a `HandScore` for the aborted hand — no synthetic last-trick team, Capot flag or failed-hand verdict.
- Do not fix the pre-existing surrender-on-hand-2+ stale-`LastHandResult` re-buffer found in the same code path; file it to `deferred-work.md`.
- Do not touch Quick Play's rule set: synthesized rooms are `stopAtTarget: false`, explicitly.
- Do not edit the in-app rules reference (`features/rules/content/*`) — Story 12.9 owns that surface.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Create room, toggle untouched | `POST /rooms`, `stopAtTarget` omitted | Room persists `false`; no chip anywhere; match plays exactly as today | N/A |
| Create room, dosta ON | `POST /rooms` with `stopAtTarget: true` | Room persists `true`; chip on lobby card, waiting room, scoreboard | N/A |
| Trick 1-7 crossing | ON, A at 940, trick 5 gives A 68 | Match ends at that trick: `teamScores[A] = 1008`, `winnerTeam` A, `phase: match_end`; tricks 6-8 never played. `event:trick_resolved` still names the right winner, then `match_end` | N/A |
| Nobody crosses through trick 7 | ON, both under target at trick 8 | Hand completes; `scoreHand` applies last-trick/Capot and the failed-hand rule as always | N/A |
| Croatian declaring phase crossing | ON, B at 930, contest awards B 150 | Match ends inside `declaring` — `phase` never becomes `playing`, `trickNumber` stays 0. Reveal event still fires, then `match_end` | N/A |
| Belote crossing mid-trick | ON, A at 990, announces Belote on card 2 of the trick | Match ends on the +20. `event:belot_announced` fires; **no** `event:trick_resolved` | N/A |
| Belote crossing on the 4th card | ON, K/Q is the trick's 4th card and the +20 crosses | Match ends before the trick resolves; the deferred `trick_resolved` is suppressed | N/A |
| Belote DECLINED on the 4th card, trick crosses | ON, `skip_belot` awards nothing but `finishCardPlay` resolves the trick and its card points cross | The trick really resolved, so `event:trick_resolved` **is** emitted (and at Bitola trick 1, `event:declarations_resolved` too), then `match_end` | N/A |
| Belote announced but short, trick crosses | ON, the +20 leaves the team under target and the trick it then resolves crosses | Same as above: `belot_announced`, then `trick_resolved`, then `match_end` | N/A |
| Belote at Bitola trick 1 with the contest open | ON, +20 on card 2 of trick 1 would cross while melds are undeclared/unresolved | The stop DEFERS: the +20 is banked, trick 1 plays out, and the stop is evaluated at trick-1 resolution with the melds included. A declared quarte is never discarded | N/A |
| Crossing team is the taker's OPPONENTS | ON, the defenders' running total reaches the target mid-hand | Defenders win; no failed-hand transfer is evaluated and the taker keeps its own banked points | N/A |
| Declaration timeout crossing | ON, Croatian phase window elapses and the forced close crosses | `handleMatchEnd` runs: coins settled, XP, honor, match row, `event:match_end`, session removed | Without the new guard the table stalls forever — see Code Map |
| Capot in progress | ON, a team winning every trick crosses at trick 6 | Match ends; no +100 Capot bonus, no +10 | N/A |
| Setting OFF | `stopAtTarget: false`, any variant | Every checkpoint is a no-op; no state field differs from today except the new flag | N/A |
| Both teams already over target | ON, fixture seeds both above target, then a checkpoint fires | `determineMatchWinner` decides (higher score, then the taker); no panic | Unreachable in real play — only one team gains points per checkpoint |
| Reconnect mid-match | ON, player reconnects | Snapshot carries `stopAtTarget: true`; scoreboard chip restored | N/A |
| QuickPlay | Synthesized room | `stopAtTarget: false`; lobby card unchipped | Absent key reads as OFF via `=== true` |

</frozen-after-approval>

## Code Map

**Engine (`server/internal/game/`)**
- `types.go:153-208` -- `VariantRules`; `types.go:214-234` -- `RulesFor`. Add `StopAtTarget bool`, `false` in **both** presets.
- `state.go:146-178` -- metadata section. Add `StopAtTarget bool json:"stopAtTarget"` beside `DeclarationsEnabled:178`. Story 12.10 triage: **PUBLIC** (room config, identical for all four seats, reveals no cards) — no `projection.go` change.
- `state.go:335-380` -- `NewGame(...)`. Add trailing `stopAtTarget bool`; `rules.StopAtTarget = stopAtTarget` beside `:356`; seed the wire field beside `:362`.
- `rules_engine.go:40-42` -- `RefreshDerivedFlags`; mirror `Rules.StopAtTarget` so the wire field has one writer.
- `scoring.go` -- **new** `teamRunningTotal(state, team) int` and `stopAtTargetIfReached(state) bool`. No such helper exists today (`scoring.go:56-59` is post-bonus and not reusable). Reuse `matchTarget` (`:286-291`) and `determineMatchWinner` (`:266-283`) — the latter dereferences `TrumpCallerSeat`, always non-nil at all three checkpoints.
- `declarations.go:644-663` -- `resolveTrickWithDeclarations`. Call the check after the two `resolveDeclarationsForHand` arms, and **only when `Phase != PhaseHandScoring`** so trick 8 keeps its normal `scoreHand` path. This one site covers tricks 1-7 *and* Bitola's trick-1 declaration resolve.
- `declarations.go:419-430` -- `closeDeclarationPhase`, the single choke point for Croatian `declaring -> playing` (both the answer path and `ForceCloseDeclarationPhase:439`). Call the check at the very end, after `:426` sets `PhasePlaying`, so the stop wins.
- `declarations.go:588-606` -- `handleAnnounceBelot`. Call the check after `BelotPoints += 20` at `:598` and **return before** `finishCardPlay:603` when it fires. `handleSkipBelot:609` awards nothing — no hook.
- `testfixtures/fixtures.go:866-871` -- `WithoutDeclarations` mutator, the pattern to copy. Add `WithStopAtTarget(gs)` and a `NewGameMidPlayNearEnd(trickNum, teamAScore, teamBScore)` factory (`NewGameNearEnd:535` forces trick 8, so it cannot test a mid-hand stop). Never raw `GameState{}` literals.

**Match layer (`server/internal/match/`)**
- `live_match.go:227-235` -- `StartMatch(...)`; `:251` -- `game.NewGame(...)`. Add the trailing param and thread it.
- `live_match.go:978-1004` -- `broadcastActionResult`'s `ActionDeclare/ActionSkipDeclare` arm. Add `if newState.Phase == game.PhaseMatchEnd { return }` **after** `broadcastDeclarationsResolvedIfTransition:1000`, before `broadcastState:1004` — otherwise `match_state` ships ahead of `match_end` and races MatchPage's stale-state redirect (8.5-1 AC4).
- `live_match.go:1007-1047` -- the `ActionAnnounceBelot/ActionSkipBelot` arm. This arm is **shared by announce and skip**, and only one of the three crossings it can carry leaves the trick unresolved, so the guard cannot simply precede the deferred-trick block. Gate that block on the trick having **actually** resolved — `len(oldState.CurrentTrick) == 4 && newState.TrickNumber != oldState.TrickNumber` — and put the `PhaseMatchEnd` return **after** it, mirroring the `ActionPlayCard` and declare arms. The three cases: an `announce_belot` whose +20 crosses returns from the engine before `finishCardPlay`, so `TrickNumber` is unchanged and no `trick_resolved` may be sent; a `skip_belot` awards nothing but still runs `finishCardPlay`, so the trick resolves and a crossing there **must** still emit `trick_resolved` (and `declarations_resolved` at Bitola trick 1); an `announce_belot` whose +20 falls short behaves like the skip case.
- `live_match.go:1541-1574` -- `handleDeclarationTimeout`. **No `PhaseMatchEnd` check today.** Skip `setTurnExpiry`/`startTimerLocked` (`:1560-1561`) and, after the reveal broadcast, call `handleMatchEnd` and return instead of `broadcastState`/`maybeScheduleBotAction`. Capture `session.startedAt` before the unlock. Mirror `handleHandCompleteTimeout:1616-1622` exactly. Without this, a timeout-driven close that crosses ends the match with no persistence, no settlement, no `event:match_end`, and a session that is never removed — the table hangs permanently.
- `live_match.go:839-919` -- the `ActionPlayCard` arm needs **no change**: its existing `PhaseMatchEnd` guard (`:917`) already sits after `trick_resolved` and the reveal, and `handJustScored:885` self-suppresses on the nil `LastHandResult`.
- `live_match.go:597-611`, `:1241-1348` -- the match-end detector and `handleMatchEnd`. Verified `LastHandResult`-agnostic: reads only `WinnerTeam`, `TeamScores`, per-player flags. No change.
- `live_match.go:468-579` -- the timer arm ladder matches no arm on `PhaseMatchEnd`, and `:406` cancels pre-mutation. No change.
- `bot_driver.go:84-138` -- `botDecisionSeats` returns zero seats for `match_end`. No change.

**Room domain (`server/internal/room/`)**
- `model.go:79-102` -- add `StopAtTarget bool` with `gorm:"not null"` beside `DeclarationsEnabled:102`, copying the no-`default`-tag rationale. Note the trap **inverts** here: the safe default is `false`, so a forgotten field inserts the safe value — the tag is still omitted, to keep an explicit `true` insertable.
- `handler.go:166` -- `CreateRoomRequest`; add `StopAtTarget *bool` (nil -> `false`).
- `handler.go:578-585` -- the `declarationsEnabled` nil-resolution block; add the mirror (nil -> `false`).
- `handler.go:648-669` -- `CreateRoom`'s `&Room{`; `handler.go:3790-3829` -- QuickPlay's `&Room{` (set `false` explicitly). `handler.go:594`'s `gateProbe` is not persisted — leave it.
- Room payload maps — **three** in Go, all need the key: `handler.go:429` (`roomLifecyclePayload`, feeds `room_created` *and* `room_updated`), `handler.go:3898` (QuickPlay's own `room_created`), `lobby_disconnect.go:259` (`broadcastRoomUpdated`).
- `handler.go:172` -- `MatchStarter` interface; `handler.go:2508` and `:3677` -- the two `StartMatch` call sites.

**Migration**
- `server/migrations/000022_add_stop_at_target_to_rooms.{up,down}.sql` -- `ALTER TABLE rooms ADD COLUMN stop_at_target BOOLEAN NOT NULL DEFAULT FALSE;` / `DROP COLUMN`. Copy the commentary style of `000021_add_declarations_to_rooms.up.sql`; `DEFAULT FALSE` is the backfill — every existing room keeps finishing its hand.

**Wire contract (same commit)**
- `server/internal/ws/events_contract_test.go:53` -- golden `GameState` fixture; add the field. Regenerate `server/internal/ws/testdata/events/event_match_state.json` with `UPDATE_GOLDENS=1 go test ./internal/ws/...`.
- `client/src/shared/types/wsEvents.schemas.ts:140` -- `EventMatchStateSchema` (`z.strictObject`; a Go-side key fails the parse until this lands); conformance gate at `:409-410`.
- `client/src/shared/types/matchTypes.ts:229` -- `MatchState` (required, the schema validates it).
- `client/src/shared/types/wsEvents.ts:373`, `:415` -- `RoomCreatedPayload` / `RoomUpdatedPayload` (optional).
- `client/src/shared/types/apiTypes.ts:171` (`Room`, optional), `:205` (`CreateRoomRequest`, required).

**Client UI**
- `client/src/features/room/CreateRoomModal.tsx` -- state at `:125`, options list at `:305-317`, `Field`+`Segmented` JSX at `:419-430` (insert the sibling immediately after), mutation payload at `:213`, reset at `:275`, `PreviewCard` prop at `:722`/`:765`/`:777`, preview chip at `:821`.
- `client/src/features/lobby/components/RoomCard.tsx:128` -- ON-only chip in the meta row (`=== true`, never truthiness — the QuickPlay map is a known omitter).
- `client/src/features/room/RoomPage.tsx:1220` -- waiting-room `Badge` in `room-info-badges`.
- `client/src/features/match/components/ScorePanel.tsx:29,163,305,319-332` -- the `score-meta` band; `MatchPage.tsx:1596` passes the prop straight through (strict schema guarantees the key).
- `client/src/shared/i18n/{en,mk,hr,sr}.json` -- `lobby.createRoomModal.matchEnd` / `matchEndFinish` / `matchEndStop` / `matchEndHint` beside `:699-702`, and `lobby.card.stopAtTarget` / `stopAtTargetAriaLabel` beside `:774-775`. `i18n.parity.test.ts` gates all four.

**Positional-arg blast radius (mechanical, compile-enforced):** 94 `StartMatch(` call sites across 15 `server/internal/match/*_test.go` files and 12 `game.NewGame(` call sites (`internal/game/*_test.go` plus `internal/bot/simulation_test.go:34,276`) all gain one trailing argument. Plus `handler_test.go:2407`'s `fakeMatchStarter.StartMatch`. Client-side: the `MatchState` fixtures at `MatchPage.test.tsx:79`, `matchStore.test.ts:21`, `useWsDispatch.test.ts:46`, `useReconnectionRedirect.test.tsx:46`, `legalCards.test.ts:65`, `matchTypes.test.ts:94` all gain the required key.

## Tasks & Acceptance

**Execution:**
- [x] `server/migrations/000022_add_stop_at_target_to_rooms.{up,down}.sql` -- add `stop_at_target BOOLEAN NOT NULL DEFAULT FALSE` with a reversing down -- persistence, default as backfill.
- [x] `server/internal/room/model.go` -- add `StopAtTarget bool`, no GORM `default` tag -- documented trap, inverted polarity noted.
- [x] `server/internal/game/types.go` -- add `StopAtTarget` to `VariantRules`, `false` in both presets -- D-VAR-1 config field, fully populated presets.
- [x] `server/internal/game/state.go` + `rules_engine.go` -- `NewGame` trailing param, override on `Rules`, public `stopAtTarget` wire field mirrored in `RefreshDerivedFlags` -- one writer, no drift.
- [x] `server/internal/game/scoring.go` -- `teamRunningTotal` + `stopAtTargetIfReached` (commit, zero accumulators, nil `LastHandResult` and the timer/prompt fields, `determineMatchWinner`, `PhaseMatchEnd`) -- the whole rule in one pure helper.
- [x] `server/internal/game/declarations.go` -- wire the check into `resolveTrickWithDeclarations`, `closeDeclarationPhase` and `handleAnnounceBelot` -- the three and only three point-award moments.
- [x] `server/internal/match/live_match.go` -- widen `StartMatch`, thread to `NewGame`; add the `PhaseMatchEnd` guards to the declare and Belote broadcast arms; add the match-end branch to `handleDeclarationTimeout` -- event ordering and the stalled-table fix.
- [x] `server/internal/room/handler.go` + `lobby_disconnect.go` -- request field, nil-resolution, both `&Room{}` sites, all three payload maps, `MatchStarter`, both `StartMatch` call sites -- end-to-end HTTP + lobby broadcast.
- [x] `server/internal/game/testfixtures/fixtures.go` -- `WithStopAtTarget` mutator + `NewGameMidPlayNearEnd` factory -- factories are the single update point.
- [x] `server/internal/game/stop_at_target_test.go` (NEW FILE) -- table-driven, through `ApplyAction` only: every engine row of the I/O Matrix, each paired with an OFF control proving the same fixture plays on; plus trick-8-still-scores-normally, no-Capot/no-last-trick at the stop, `LastHandResult` nil, accumulators zeroed, `ActivePlayerSeat` untouched, and the both-teams-over defensive case -- edge-case coverage in one dedicated file, since the feature is one cross-cutting gate.
- [x] `server/internal/match/stop_at_target_session_test.go` (NEW FILE) -- the flag reaches `gs.Rules` in both variants; a Belote stop emits `belot_announced` then `match_end` with **no** `trick_resolved`; a declaration-timeout stop runs `handleMatchEnd` (match persisted, session removed) instead of hanging -- the seams with no coverage of their own.
- [x] `server/internal/room/stop_at_target_handler_test.go` (NEW FILE) + `handler_test.go` (`fakeMatchStarter` widened) -- default-false, explicit-true, quick-play-is-false, `StartMatch` plumbing, DB round-trip of `true`, raw INSERT omitting the column lands FALSE, and each of the three payload maps carries the key -- request/persistence contract.
- [x] `server/internal/ws/events_contract_test.go` + `testdata/events/event_match_state.json` -- regenerate golden -- wire drift gate.
- [x] `client/src/shared/types/{matchTypes,wsEvents,wsEvents.schemas,apiTypes}.ts` -- add the field across type, Zod schema and room payloads -- strict-object parse fails until all land.
- [x] `client/src/features/room/CreateRoomModal.tsx` (+ `.test.tsx`) -- segmented control defaulting to "finish the hand", submitted and previewed -- the user-facing control.
- [x] `client/src/features/lobby/components/RoomCard.tsx`, `client/src/features/room/RoomPage.tsx`, `client/src/features/match/components/ScorePanel.tsx` + `MatchPage.tsx` (+ tests) -- ON-only chip via `=== true` -- joiners and players can see the rule.
- [x] `client/src/shared/i18n/{en,mk,hr,sr}.json` -- six new keys in all four locales -- parity test gates it.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- append the surrender stale-`LastHandResult` re-buffer finding -- discovered here, out of scope here.

- [x] `server/internal/room/lobby_disconnect.go` + `lobby_disconnect_payload_test.go` (NEW FILE, not in the original plan) -- narrow the `hub` field from `*ws.Hub` to the package's existing `Broadcaster` interface and assert both rule flags are present in the disconnect-driven room payload -- the third map was otherwise untestable, and it is the one that actually shipped broken for `declarationsEnabled`. Verified by mutation: removing the key fails the test, including on the row where the expected value is `false`.
- [x] `server/internal/game/stop_at_target_test.go` -- add `TestStopAtTargetSurvivesPerSeatProjection` -- the reconnect matrix row's mechanism (`ProjectForSeat`, which masks by enumeration) had no direct assertion.
- [x] `server/internal/match/stop_at_target_session_test.go` -- assert no `event:hand_scored` and no `event:match_state` ahead of `event:match_end` on a Belote stop -- two acceptance criteria that were stated but unpinned. Verified by mutation: removing the Belote arm's guard produces `belot_announced, trick_resolved, match_state, match_end, match_state` and fails both.

**Acceptance Criteria:**
- Given the create-room dialog is opened, when the owner submits without touching the new control, then the room is created with `stopAtTarget: false`, no surface shows a chip, and matches play exactly as they did before this change.
- Given a room with dosta ON in either variant, when a team's running total reaches the target part-way through a hand, then the match ends on that very action with `phase: match_end` and that team as `winnerTeam`, the remaining tricks are never dealt or played, and `teamScores` equals the running totals that crossed.
- Given a room with dosta ON, when the match ends mid-hand, then `handleMatchEnd` completes in full — coins settled, XP and honor awarded, the match row persisted, `event:match_end` emitted before the final `event:match_state`, and the session removed — with no `event:hand_scored` and no `hand_results` row for the aborted hand.
- Given a room with dosta ON, when the Croatian declaration window times out and the forced close crosses the target, then the match ends through `handleMatchEnd` and the table does not stall.
- Given a room with dosta ON, when no team has crossed by the end of trick 8, then the hand is scored normally with the last-trick or Capot bonus and the failed-hand rule, identical to a dosta-OFF room.
- Given a room with dosta ON, when a player views the lobby card, the waiting room, or the in-match scoreboard, then a localized chip naming the rule is visible on each in all four locales.
- Given a room with dosta ON, when a player disconnects and reconnects mid-match, then the restored snapshot carries `stopAtTarget: true` and the scoreboard chip reappears.
- Given any room with dosta OFF, when a full match is played end to end, then every `event:match_state` field other than the new one, and every scoring outcome, is unchanged from before this change.

## Spec Change Log

### Iteration 1 (2026-08-24) — two design defects, both spec-rooted

Three independent review layers converged on the first finding. Both were verified against the
code before amending. The human chose a SCOPED fix over the workflow's full revert-and-re-derive:
the implementation was otherwise correct and passing every gate, and both defects are localized.

- **intent_gap (frozen block): "the stop zeroes `HandPoints`/`DeclarationPoints`/`BelotPoints`"
  broke the declaration reveal.** `broadcastDeclarationsResolvedIfTransition`
  (`live_match.go:1178-1184`) derives the reveal's `winnerTeam` from `DeclarationPoints[team] > 0`,
  so every declaration-driven stop broadcast a reveal with `winnerTeam: null` and no winner for the
  panel to anchor to. **Amended:** the stop no longer zeroes the accumulators. The known-bad state
  avoided is a Croatian contest or Bitola trick-1 crossing whose reveal renders nothing. The
  zeroing existed to stop the scoreboard's "+N this hand" bar double-counting banked points, but
  `scoreHand` does not clear those accumulators at a normal match end either — the display already
  behaves that way, so the rule bought nothing and cost a defect. Resolved by the human
  (2026-08-24) in favour of dropping it rather than rewriting the shared reveal derivation, which
  every non-dosta declaration path also uses.
- **bad_spec (Code Map): the Belote arm's guard placement suppressed events that really happened.**
  The Code Map said to place the `PhaseMatchEnd` return before the deferred-trick block. That is
  correct only for an `announce_belot` whose +20 itself crosses. The arm is shared with
  `skip_belot`, which awards nothing but still runs `finishCardPlay` — so the trick resolves and
  checkpoint 1 can cross on its card points — and with an `announce_belot` whose +20 falls short of
  the target. In both, the guard dropped a genuine `event:trick_resolved` (no collect animation on
  the final trick) and, at Bitola trick 1, the `event:declarations_resolved` reveal. **Amended:**
  the deferred-trick block is gated on the trick having actually resolved
  (`newState.TrickNumber != oldState.TrickNumber`) and the guard moved after it, matching the
  `ActionPlayCard` and declare arms. Three matrix rows added for the uncovered crossings.

**KEEP (must survive any re-derivation):**

- The three-checkpoint structure and `stopAtTargetIfReached` as a single pure helper. Neither
  defect was in the checkpoint design; both were in what happens at the stop and how the match
  layer broadcasts it.
- `LastHandResult = nil` at the stop, and the reasoning recorded for it. Unaffected by the
  amendment and independently load-bearing.
- Not touching `ActivePlayerSeat` or trick state. This is what keeps `trickResolvedWinnerSeat`
  correct on the final trick, and it is what makes the amended gate above possible.
- Every crossing test paired with an OFF control on the SAME fixture. This pairing is what proved
  the suppressed `trick_resolved` was one the layer really would otherwise send, and it is how
  defect A was demonstrable at all.
- Mutation-verifying each guard rather than trusting a green suite: removing the Belote guard
  yields `belot_announced, trick_resolved, match_state, match_end, match_state`, which is how its
  necessity was established. The same treatment now applies to the declare-arm guard, which the
  review found had no test at all.
- The `Broadcaster` narrowing in `lobby_disconnect.go` and its payload test, including the
  presence-vs-value assertion split that catches a dropped key even on rows where the expected
  value is `false`.

### Iteration 2 (2026-08-24) — one rules gap, three vacuous assertions

Second review round. Both iteration-1 fixes verified correct and independently mutation-tested in
both directions; neither is re-opened here.

- **intent_gap (frozen block): a Belote crossing during Bitola trick 1 silently discarded declared
  melds.** Bitola declares *during* trick 1 and resolves the contest only when the trick completes,
  but checkpoint 3 fires on the +20 before `finishCardPlay`. `DeclarationPoints` is written in
  exactly one place (`declarations.go:702`, reached only via `resolveDeclarationsForHand`), so a
  trick-1 Belote stop ended the match with every meld still unconverted — a player who declared a
  quarte simply lost it, and in a close match that decides the winner. **Amended:** the Belote
  checkpoint defers while `TrickNumber == 1 && !DeclarationsResolved`. Resolved by the owner
  (2026-08-24) in favour of finishing trick 1. The reviewer's proposed fix — resolve the contest at
  the Belote moment — was REJECTED and must not be reintroduced: at card 2 the later seats have not
  declared yet, so it would settle an incomplete contest and clear melds that were never compared.
- **patch: three assertions were vacuous, passing on fixture defaults rather than on the code.**
  Deleting `|| state.Phase == PhaseHandScoring` from the stop's no-op set failed nothing, because
  the only trick-8 fixture sits below the target at the checkpoint — while in production that guard
  is what keeps a trick-8 crossing on card points from bypassing `scoreHand` and its bonuses and
  failed-hand evaluation. `LastHandResult = nil` was unpinned because no fixture ever populates it,
  so the "load-bearing" line could be deleted silently. The timer/prompt clears were asserted only
  on fixtures where those fields were already zero.

**KEEP (in addition to iteration 1's list):**

- The deferral is expressed as `TrickNumber == 1 && !DeclarationsResolved` and needs no variant
  comparison and no new state field: Croatian has resolved its dedicated phase before trick 1, and
  a declarations-off room has `DeclarationsResolved` seeded true, so neither defers. Do not
  re-derive this as a variant check (D-VAR-1) or a new flag.
- Fixtures must be able to carry a populated `LastHandResult` and a hand number above 1, so the
  nil-ing is testable rather than incidentally true.

## Design Notes

**Why three checkpoints and not a per-action sweep.** Card points, declarations and Belote are the only three things that move a team's total mid-hand, and each has exactly one write site in the engine (`playing.go:127`, `declarations.go:702`, `declarations.go:598`). Checking immediately after each write is both complete and cheap; a blanket check at the top of `ApplyAction` would run on every pass, pause and legality rejection to catch nothing.

**Why the trick hook sits inside `resolveTrickWithDeclarations` rather than `resolveTrick`.** At trick 1 the declaration contest resolves *after* `resolveTrick` returns, in the same call. Hooking the outer function means one call site sees both the trick's card points and the freshly awarded meld points, and there is no window in which a half-updated total is tested.

**Why the Belote stop returns before `finishCardPlay`.** Letting the deferred turn flow run would advance play — or resolve the whole trick — with a team already over the target, which is the one thing dosta forbids. The cost is a trick abandoned with up to four cards face up, which is exactly where play stopped and is truthful to show.

**Why the zero value is safe here, unlike `DeclarationsEnabled`.** That field's zero value (`false`) is destructive, which is why the previous spec argued for positional parameters. `StopAtTarget`'s zero value is today's behaviour, so a forgotten argument degrades to correct rather than to a silently broken match. Positional is kept anyway for consistency with the shipped precedent — but this is the second room-level rule field, and the ~106 mechanical call-site edits it forces are the accepted cost. **If a third arrives, that is the moment to introduce a `RoomRules` struct with a constructor** rather than a fourth positional bool.

## Verification

**Commands:**
- `make lint` -- expected: clean, both stacks.
- `make test` -- expected: all Go and Vitest suites pass, including the pre-existing Bitola and Croatian engine suites with no behavioural edits (only the mechanical trailing argument).
- `UPDATE_GOLDENS=1 go test ./internal/ws/...` then `go test ./internal/ws/...` -- expected: golden regenerated once, then clean.
- `npx vitest run src/shared/types/wsEvents.contract.test.ts src/shared/i18n` -- expected: contract and locale-parity suites pass.
- `make migrate` -- expected: `000022` applies; down then up leaves `rooms` unchanged.

**Manual checks:**
- Create a Bitola and a Croatian room with dosta ON at 501, play until a team crosses mid-hand: the match ends on that trick, the scoreboard shows the crossing total with no "+N this hand" bar, and the match appears in history.
- Same two rooms with dosta OFF: hands play out and score with bonuses exactly as before.

## Suggested Review Order

**The rule itself**

- Start here: the whole stop in one pure helper, and the canonical statement of the three checkpoints.
  [`scoring.go:260`](../../server/internal/game/scoring.go#L260)

- The running total, and why it is deliberately not scoreHand's arithmetic.
  [`scoring.go:170`](../../server/internal/game/scoring.go#L170)

- Checkpoint 1: covers tricks 1-7 and Bitola's trick-1 contest in one settled total.
  [`declarations.go:705`](../../server/internal/game/declarations.go#L705)

- The one deferral, and the rejected alternative that must not come back.
  [`declarations.go:644`](../../server/internal/game/declarations.go#L644)

- Checkpoint 2: the single choke point for the Croatian declaring-to-playing move.
  [`declarations.go:431`](../../server/internal/game/declarations.go#L431)

**Rule config (D-VAR-1)**

- The config field both presets populate false; the room overrides it in NewGame.
  [`types.go:208`](../../server/internal/game/types.go#L208)

- The public wire mirror, with its Story 12.10 visibility triage.
  [`state.go:194`](../../server/internal/game/state.go#L194)

- One writer for the mirror, so it cannot drift from the config the engine reads.
  [`rules_engine.go:45`](../../server/internal/game/rules_engine.go#L45)

**Match layer: what the wire sees**

- The gate that tells a resolved trick from one abandoned mid-air. Highest-risk line here.
  [`live_match.go:1074`](../../server/internal/match/live_match.go#L1074)

- The timeout path that had no match-end check at all; without this the table hangs forever.
  [`live_match.go:1615`](../../server/internal/match/live_match.go#L1615)

- Positional plumbing, so a forgotten argument is a compile error.
  [`live_match.go:237`](../../server/internal/match/live_match.go#L237)

**Persistence and HTTP**

- The column, with the inverted GORM default-tag trap noted.
  [`model.go:134`](../../server/internal/room/model.go#L134)

- Absent means off: the nil-pointer resolution that makes that true.
  [`handler.go:603`](../../server/internal/room/handler.go#L603)

- The request field mirroring declarationsEnabled.
  [`handler.go:173`](../../server/internal/room/handler.go#L173)

**Client**

- The owner-facing control, defaulting to finishing the hand.
  [`CreateRoomModal.tsx:464`](../../client/src/features/room/CreateRoomModal.tsx#L464)

- Explicit === true, because the QuickPlay payload of an older server omits the key.
  [`RoomCard.tsx:151`](../../client/src/features/lobby/components/RoomCard.tsx#L151)

**Tests worth reading (they carry the reasoning)**

- The deferral's defect stated as a failure message: melds discarded, in points.
  [`stop_at_target_test.go:608`](../../server/internal/game/stop_at_target_test.go#L608)

- Why "only one team can cross per checkpoint" stopped being true.
  [`stop_at_target_test.go:936`](../../server/internal/game/stop_at_target_test.go#L936)

- The wire contract: a real trick still resolves, an abandoned one does not.
  [`stop_at_target_session_test.go:1`](../../server/internal/match/stop_at_target_session_test.go#L1)

- Presence asserted apart from value, so a dropped key fails even when false.
  [`lobby_disconnect_payload_test.go:39`](../../server/internal/room/lobby_disconnect_payload_test.go#L39)

