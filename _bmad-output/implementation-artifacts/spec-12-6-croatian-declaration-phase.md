---
title: "Croatian declaration phase"
type: "feature"
created: "2026-08-19"
status: "done"
review_loop_iteration: 0
context: ["{project-root}/_bmad-output/implementation-artifacts/epic-12-context.md"]
baseline_commit: "4639656e4da3d933dc6b7ab64e0d84829217fffa"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Declarations are welded to trick 1. `handleDeclare`/`handleSkipDeclare` hard-reject `TrickNumber != 1`, `checkDeclarationPrompt` returns early outside it, and the reveal is gated on `TrickNumber == 2`. Croatian declares in a dedicated phase between bidding and trick 1. Story 12.1 shipped `Rules.DeclarationTiming`; nothing reads it.

**Approach:** Add a `PhaseDeclaring` state-machine phase, entered from `handlePickTrump` when `DeclarationTiming` is `dedicated_phase`. Seats are prompted one at a time counter-clockwise from `(DealerSeat+1)%4` — the same seat that will lead trick 1 — reusing the existing `AwaitingDeclaration` + `ActivePlayerSeat` prompt mechanism, so no new event or action reaches the wire. When all four seats have answered, `resolveDeclarationsForHand` runs and the game moves to `PhasePlaying` at trick 1. Then teach every phase-switch in the match, bot, and reconnect layers about the new value, because each one's `default` arm is a silent stall.

## Boundaries & Constraints

**Always:**

- The phase is selected by `state.Rules.DeclarationTiming`, never by a variant name (D-VAR-1). Bitola keeps the existing `bidding.go` block verbatim and every pre-existing Bitola test passes unchanged.
- Only seats **holding a meld** are prompted; meld-less seats count as answered and the cursor advances past them without stopping. This is `checkDeclarationPrompt`'s existing rule, unchanged.
- The phase always terminates. Every seat is visited exactly once; a hand where no seat holds a meld resolves straight through to `PhasePlaying` without ever setting `AwaitingDeclaration`.
- Answer progress is a value-typed counter in `state.go`'s section 3 beside `DeclarationsResolved`, so `cloneGameState` needs no new line — and it is reset in `startNewHand` with the other declaration fields, or it leaks into hand 2.
- On resolution the game transitions to `PhasePlaying` immediately and arms the first leader's timer; `event:declarations_resolved` fires on the same `false→true` latch as today and its 8s panel floats over live play, exactly as it does at Bitola's trick 2.
- Timer expiry auto-skips the prompted seat and the phase continues to the next. The auto-action chain must be able to advance through the phase, not break out of it.
- Pause, surrender, disconnect, and reconnect all work in the new phase. A reconnecting player is restored into the phase with the turn cursor intact, and their turn timer re-armed if it is their turn.
- Bots answer when prompted. `bot.Decide` must not reach `chooseCard` in this phase — `LegalCards` is nil there and `chooseCard` panics on `legal[0]`.

**Ask First:**

- If prompting only meld-holders turns out to leak strictly more information than Bitola already does via `event:player_declared`, stop before changing who is prompted — that is a rules decision, not an implementation detail.
- If making the phase pausable or surrenderable requires more than extending the existing phase allowlists in `pause.go` and `surrender.go`, stop rather than reworking either feature.

**Never:**

- Don't add a WS event, a client→server action, or a field to any existing payload. The phase rides `match_state` as a `phase` string; `awaitingDeclaration` + `activePlayerSeat` already express "prompt this one player".
- Don't add a client variant→rule fact. Nothing on the client derives behaviour from the variant here — it reacts to the server's phase. `variantRules.ts` stays as 12.5 left it.
- Don't touch `DeclarationReveal`'s centring, its 8s/1.5s reduced-motion timing, its geometry, or any `data-testid` — frozen by 12.5 and its upstream specs. Don't change `detectDeclarations`, `resolveDeclarations`, `declarationBeats`, or any meld value.
- Don't change Bitola's timing, its trick-2 reveal gate, or `event:player_declared`'s meaning.
- Don't add i18n keys — the prompt reuses `match.declaration.*` and expiry reuses `match.timer.autoSkippedDeclare`.
- Don't make Croatian selectable (12.8) or touch the tie rule (12.7).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Croatian trump picked | round 1 or 2, any picker | `Phase=declaring`, `TrickNumber=0`, `ActivePlayerSeat=(Dealer+1)%4`, cursor at seat 1 of 4 | N/A |
| Bitola trump picked | any | Unchanged: `Phase=playing`, `TrickNumber=1`, prompt inside trick 1 | N/A |
| Prompted seat declares | holds a meld, is active | Melds stored, `event:player_declared` fires, cursor advances | not active seat → `ErrNotYourTurn` |
| Prompted seat skips | holds a meld, is active | Nothing stored, cursor advances, no `player_declared` | wrong phase → `ErrWrongPhase` |
| Meld-less seats between | e.g. seats 2 and 3 hold nothing | Cursor jumps straight from seat 1 to seat 0; `AwaitingDeclaration` never set for 2 or 3 | N/A |
| No seat holds a meld | all four empty | Phase opens and resolves in the same transition; `DeclarationPoints` both zero | N/A |
| Timer expires while prompted | per-move room | Auto `skip_declare`, `event:auto_action` with `skip_declare`, phase continues to next seat | must not re-arm on the elapsed deadline (the `TODO(croatian-enablement)` hot-spin) |
| Last seat answers | 4 of 4 answered | Resolve, `event:declarations_resolved`, `Phase=playing`, `TrickNumber=1`, leader's timer armed | N/A |
| Bot is prompted | bot seat holds a meld | Declares after the normal think delay | must never emit `play_card` in this phase |
| Disconnect during phase | any seat drops | Reconnect window opens, turn timer frozen, phase preserved — same as a drop in bidding | `default` arm must not swallow it |
| Reconnect during phase | player returns | Snapshot restores `phase=declaring` and the cursor; prompt reappears only if still their turn | N/A |
| Play attempted in phase | `action:play_card` | Rejected | `ErrWrongPhase` |

</frozen-after-approval>

## Code Map

**Engine — `server/internal/game/`**

- `types.go:207-224` `Phase` constants — add `PhaseDeclaring = "declaring"`. `types.go:109-119` `DeclarationTiming` constants and `types.go:163-167` the field, whose comment says "Not read yet"; presets `:192` (croatia) / `:202` (bitola). This story is its first consumer.
- `bidding.go:176-182` in `handlePickTrump` — **the single insertion point**, and the only place `PhasePlaying` is ever set. Today: `Phase`, `ActivePlayerSeat=(Dealer+1)%4`, `TrickNumber=1`, `CurrentTrick={}`, then `checkDeclarationPrompt`. Both Croatian bidding paths converge here: round-1 free-suit (`:119-140`, no candidate) and the forced dealer (`handlePassTrump:51-54` rejects the 4th round-2 pass under `AllPassDealerMustPick`, so `reshuffleAndRedeal` is unreachable). Preceded by `mergeFaceDownCards:167` and the instant-win early return `:170-174`, which returns before any of this.
- `rules_engine.go:49-60` phase switch, `default: ErrWrongPhase` — a missing `case PhaseDeclaring` is a silent hang, not a compile error. Phase-independent action prefix at `:11-47`.
- `declarations.go:303-330` `handleDeclare` / `:333-348` `handleSkipDeclare` — both hard-require `TrickNumber == 1` (`:304`, `:334`); bodies are otherwise reusable. **`handleSkipDeclare` does not advance `ActivePlayerSeat`** (in Bitola the following `play_card` does) — the new phase must advance it itself.
- `declarations.go:487-499` `checkDeclarationPrompt` — the `TrickNumber != 1` guard at `:488`; `:493` treats stored `Declarations` as "already answered"; `:496` only prompts seats holding a meld, which is the rule to keep.
- `declarations.go:467-483` `resolveDeclarationsForHand` — **reusable as the reveal step unchanged**: `trickLeaderSeat := (DealerSeat+1)%4` at `:469` is correct here too, sets `DeclarationPoints`, nils the losing team's melds, sets `DeclarationsResolved`. Requires non-nil `TrumpSuit`, true post-bidding. Bitola's trick-2 orchestration is separate, in `resolveTrickWithDeclarations:445-464`.
- `state.go:127-133` section 3 (`TrickNumber`, `AwaitingDeclaration:132`, `DeclarationsResolved:133`) — the new counter belongs at `:133`, **not** the timer section. Section contract documented `:78-85`. `[4]bool` precedents: `HandCompleteReady:158`, `PausedPlayers:167`.
- `scoring.go:159-223` `startNewHand` — `AwaitingDeclaration`/`DeclarationsResolved` reset `:179-180`, `TurnExpiresAt:191`, `Players[i].Declarations = nil` `:203-207`. New field resets here.
- `bidding.go:255-320` `cloneGameState` — `:256` shallow copy covers all value types free; only slices/pointers/maps need a line at `:304-306`.
- `pause.go:10` and `surrender.go:13,16,49,53,84,88` — phase allowlists (`playing || bidding || paused`) that exclude the new phase. Turn cursor is `ActivePlayerSeat` (`state.go:161`); CCW is `(seat+1)%4` (`state.go:250`). There is no `CurrentPlayer` field.

**Session manager — `server/internal/match/`**

- `live_match.go:468`, `:1465`, `:1664` — timer-arm guards `Phase == PhasePlaying || PhaseBidding`. Without the new phase, the prompt has **no clock at all**. `:468` also computes `preserveTimer` (same seat + same phase = no fresh window), which is what makes declare/skip not cost a new window today.
- `live_match.go:1493` `handleTimerExpiry` — auto-action if-chain `:1506-1568`; `PhasePlaying && AwaitingDeclaration → skip_declare` at `:1529`; **`default:` at `:1564` returns with no timer re-armed** → permanent stall. `:1541` would otherwise try `AutoPlay` in a phase with no legal card. `TODO(croatian-enablement)` at `:1507-1524` documents the hot-spin when an auto-action is rejected and the re-arm uses an already-elapsed deadline — this story's auto-skip must not reproduce it.
- `live_match.go:1599-1652` auto-action chain, hard-gated `if cur.Phase != PhasePlaying { break }` at `:1614`; the `AwaitingDeclaration` case is at `:1624`. `:1738-1748` `autoActionTypeFor` already maps `skip_declare` → `AutoActionSkipDeclare`.
- `live_match.go:913-930` the `ActionDeclare`/`ActionSkipDeclare` broadcast arm — emits `EventPlayerDeclared` then the latch; its comment at `:922-925` already anticipates this story. `:1110` `broadcastDeclarationsResolvedIfTransition`, fire-once on `false→true`. `:1053` `broadcastActionResult` `default` = bare state broadcast.
- `live_match.go:1344` `buildMessage` — `event:match_state` is `json.Marshal` of the raw `*GameState`, so a new phase value and any new exported field reach clients with **no plumbing**. `:300-302` shows the precedent of a phase promotion living in the match layer.
- `bot_driver.go:78-107` `botDecisionSeats` — phase switch, `default:` returns empty at `:104`, so bots are never scheduled and the table stalls silently. Companions: `botDecisionContext:52` / `botDecisionContextFor:59` (re-arm staleness check), `botThinkDelay:121`, `buildBotView:204` (`LegalCards` only populated when `PhasePlaying`, `:226`). Trigger `maybeScheduleBotAction:23`, called from `live_match.go:310,553,1487,1731`, `reconnect.go:468`. `observeBotMemory:242` fires on `DeclarationsResolved` at `:255` — timing-agnostic, works unchanged and simply lands earlier in the hand.
- `reconnect.go:92-109` `HandleDisconnect` phase switch — **`default:` at `:105` silently returns**: no `Connected=false`, no reconnect window, no timer freeze. Real matrix hole. Timer freeze `:170-182`, phase save `:186-192`. `reconnect.go:406-451` restore; the re-arm branch at `:420` is gated `PhasePlaying || PhaseBidding`. `SyncStateOnConnect:488` does no per-seat masking — face-down secrecy is the `json:"-"` tag at `state.go:44` plus the per-seat replay at `:509-517`; nothing new is secret here.

**Bot — `server/internal/bot/`**

- `bot.go:14` `Decide` — flat priority ladder, not a phase switch: `:21` is the only phase comparison; `:33` `AwaitingDeclaration → ActionDeclare`; `:37` falls through to `chooseCard:178`. With `LegalCards` nil this reaches `legal[0]` at `bot.go:401` and **panics inside the session's critical section**. `botDecisionSeats`' empty default masks it today; fixing only the scheduler exposes it.
- Since only meld-holders are prompted, `ActionDeclare` stays the correct answer and the bot needs no meld detector — but the guard is still required, and `bot.Decide` must never be reached in this phase without `AwaitingDeclaration`.
- `memory.go:71` `ObserveDeclarations` — dedupes shared cards (12.5 hardening); unchanged.
- `view.go:13-52` `View` carries `Phase` but no `Rules`/`Variant`; keep it that way.

**Client — `client/src/`**

- `shared/types/matchTypes.ts:12-22` `Phase` — a **closed** string-literal union; must gain `"declaring"` or every comparison is a `tsc` error. `matchTypes.test.ts:25-40` asserts the phase array `toHaveLength(8)` — hard-coded, will fail, must be bumped.
- `shared/types/wsEvents.schemas.ts:97` `phase: z.string()` — plain string, **no Zod enum to update**, and `event_match_state.json` must stay byte-identical.
- `features/match/MatchPage.tsx:1490-1493` prompt gate is `awaitingDeclaration && activePlayerSeat === myPlayerSeat && trumpReveal === null` — phase-agnostic, so **the prompt works in the new phase with no change**. Meld derivation `:1311-1332`. `:1646` seat-ring predicate is `playing || bidding` → the active seat's countdown goes dark; `:1720` and `:1837` pause/surrender HUD clusters; `:1766-1768` and `:1888-1890` emote button. `:1462` `isMyTurn` requires `playing`, so cards are correctly unplayable. Table chrome `:1542-1546`, seats, `TrickArea`, `HandCards` are phase-independent — the phase renders a normal empty-trick table, not a blank one.
- `features/match/components/DeclarationPrompt.tsx` — testids `declaration-prompt`, `-total`, `-skip`, `-declare`; ring wraps Skip only, no client `onExpire` (`:122-124`, server-authoritative). Undismissable by construction: `OverlayBackdrop`'s dim is `pointer-events-none` and `useFocusTrap` is called without `onEscape`.
- `MatchPage.tsx:727-746` the `declRevealReady` latch defers the reveal behind the trick-1 collect sweep; in this phase `pendingResolvedTrick` is always null so it flips immediately — benign, but its rationale no longer applies.
- Stale trick-1 wording to correct where touched: `wsEvents.ts:189-192`, `useWsDispatch.ts:484-486`, `DeclareBanner.tsx:59-63`.

**Tests & fixtures**

- `testfixtures/fixtures.go:651` `NewGameCroatianJustDealt` (`PhaseBidding`, no `TrumpSuit`, 6 open + 2 face-down, hands engineered so no pick triggers instant-win), `:746` `NewGameCroatianMidBidding`, `:790` `NewGameCroatianFirstTrick` (whose comment at `:786-787` says this story supersedes it). **No factory produces a Croatian post-bidding pre-trick-1 state.** Driving `ApplyAction(NewGameCroatianJustDealt(), pick_trump)` also exercises `mergeFaceDownCards`.
- `bot/simulation_test.go:84` full-hand loop `switch gs.Phase` with `default: t.Fatalf("unexpected phase")` at `:113`; hardcodes `VariantBitola` at `:37`. A Croatian run is the strongest deadlock proof.
- `bot/bot_test.go:17` `viewFromState` is a hand-written mirror of `buildBotView` — any `View` change lands in both. `TestDecide_AlwaysDeclares:280`.
- Must keep passing: `DeclarationPrompt.test.tsx` (12 cases, incl. `:164` "onSkip must NOT fire at ring zero"), `DeclarationReveal.test.tsx` (13 cases), `MatchPage.test.tsx:1449-1536` reveal lifetime and `:1621-1676` overlap wiring, `i18n.parity.test.ts`.

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/types.go` + `state.go` -- add `PhaseDeclaring`; add the value-typed answered-seat counter beside `DeclarationsResolved` in section 3 -- a counter cannot distinguish "skipped" from "not yet asked" if derived from stored `Declarations`, and section 3 keeps `cloneGameState` free.
- [x] `server/internal/game/bidding.go` -- branch `handlePickTrump:176-182` on `Rules.DeclarationTiming`: dedicated phase sets `PhaseDeclaring`, `TrickNumber=0`, cursor at `(Dealer+1)%4`; Bitola's block stays verbatim -- the single insertion point covering both Croatian bidding paths.
- [x] `server/internal/game/declarations.go` -- add the phase handler: prompt only meld-holders, advance the cursor on declare/skip/auto-advance, and on the fourth answer call `resolveDeclarationsForHand` and hand off to `PhasePlaying` at trick 1; relax the two `TrickNumber == 1` guards to admit the phase -- reuses detection, resolution, and the reveal latch untouched.
- [x] `server/internal/game/rules_engine.go` -- add `case PhaseDeclaring` -- `default: ErrWrongPhase` makes an omission a silent hang.
- [x] `server/internal/game/scoring.go` -- reset the new counter in `startNewHand` -- otherwise it leaks into hand 2.
- [x] `server/internal/game/pause.go` + `surrender.go` -- extend the phase allowlists -- a phase where pause and surrender silently fail is a hole, and disconnect handling depends on pause.
- [x] `server/internal/match/live_match.go` -- teach the timer-arm guards (`:468`, `:1465`, `:1664`), `handleTimerExpiry`'s auto-skip (`:1529` and the `:1564` default), and the auto-action chain gate (`:1614`) about the phase -- each default arm is a permanent stall; the auto-skip must not re-arm on an elapsed deadline.
- [x] `server/internal/match/bot_driver.go` -- add the phase to `botDecisionSeats`, `botDecisionContext`, and `botThinkDelay` -- an empty seat list stalls the table with no error and no log.
- [x] `server/internal/bot/bot.go` -- guard `Decide` so the declaring phase never falls through to `chooseCard` -- `LegalCards` is nil there and `legal[0]` panics inside the session's critical section.
- [x] `server/internal/match/reconnect.go` -- add the phase to `HandleDisconnect`'s switch and the reconnect timer re-arm at `:420` -- today a drop during the phase opens no reconnect window at all.
- [x] `server/internal/game/testfixtures/fixtures.go` -- add a Croatian declaring-phase factory -- no fixture produces this state and raw state literals are banned in tests.
- [x] `server/internal/game/declarations_test.go` -- table-driven through `ApplyAction`: entry from both Croatian bidding paths, turn order, meld-less skip-through, the no-melds-at-all hand, resolution and handoff to trick 1, and Bitola unchanged -- assertions order-agnostic per 12.5.
- [x] `server/internal/match/declaration_phase_test.go` -- timer expiry auto-skips and the phase continues; the phase is pausable; reconnect restores the cursor -- the matrix the epic calls the largest risk in this story.
- [x] `server/internal/bot/simulation_test.go` + `bot_test.go` -- add `PhaseDeclaring` to the simulation switch and run a full Croatian hand -- the strongest proof the phase cannot deadlock with bot seats.
- [x] `client/src/shared/types/matchTypes.ts` + `matchTypes.test.ts` -- widen the `Phase` union and bump the hard-coded length assertion -- the union is closed, so omitting it is a `tsc` error.
- [x] `client/src/features/match/MatchPage.tsx` -- add the phase to the seat-ring predicate (`:1646`), the pause/surrender HUD gates (`:1720`, `:1837`), and the emote gates (`:1766`, `:1888`) -- otherwise the active seat has no visible countdown and the controls vanish mid-hand.
- [x] `client/src/features/match/MatchPage.test.tsx` -- assert the declaration prompt renders and the seat ring shows in the declaring phase -- the prompt gate is phase-agnostic and would regress silently.
- [x] `_bmad-output/planning-artifacts/architecture.md` -- update the phase transition table -- named in the epic AC; it is the session-manager/engine contract.

**Acceptance Criteria:**

- Given `server/internal/game`, when searched for a variant-name comparison outside `RulesFor`, then there are none.
- Given the full pre-existing suite, when it runs, then every Bitola assertion passes unchanged and `internal/ws/testdata/events/event_match_state.json` is byte-identical to `HEAD`.
- Given a Croatian hand with four bot seats, when it is played end to end, then bidding reaches the declaring phase, every seat answers, declarations resolve, trick 1 begins, and the hand scores — with no stall, no engine rejection loop, and no panic.
- Given the wire, when this story is complete, then no event or action was added and no existing payload widened; `phase` is the only field carrying the new state.
- Given the client, when searched for a variant→behaviour mapping, then `variantRules.ts` is still the only one and it is unchanged by this story.
- Given the four locale files, when the parity test runs, then their key sets are unchanged and identical.
- Given the create-room surface and the server allowlist, when this story is complete, then Croatian is still not selectable.

## Spec Change Log

- **2026-08-19 — human decision at intent capture.** The epic context (`epic-12-context.md:60`) specifies "a blocking prompt for all four players at once". The human chose **sequential turn-order prompting** instead. Amendment: the phase walks seats counter-clockwise from `(Dealer+1)%4` and reuses the existing single-active-seat prompt. This avoids a known-bad state — a simultaneous phase would need a new per-seat timer model, since `TurnExpiresAt` is singular and tied to `ActivePlayerSeat` — and it removes the need for any new WS event, because `awaitingDeclaration` + `activePlayerSeat` already express "prompt this one player". KEEP: the epic's other constraints (auto-skip on expiry, reconnect restores the player's own answer state, undismissable prompt) are unaffected and still hold.
- **2026-08-19 — human decision at intent capture.** Only seats holding a meld are prompted (matching Bitola's `checkDeclarationPrompt:496`), and the game transitions to `PhasePlaying` immediately on resolution with the reveal floating over live play (matching Bitola's trick-2 reveal). The literal epic AC reading — all four prompted, trick 1 held until the reveal ends — was rejected. This avoids two known-bad states: a bot prompted without a meld gets `ErrDeclarationNotAvailable` and enters a reject/re-arm loop, and holding the phase for the reveal would require new server-side reveal timing machinery.

## Design Notes

**Why no new wire surface.** Sequential prompting collapses this story's wire needs to zero. `MatchPage.tsx:1490-1493` already gates the prompt on `awaitingDeclaration && activePlayerSeat === myPlayerSeat`, both of which ride `match_state`, and `event:match_state` is a raw marshal of `*GameState` — so the new phase value and the prompt reach clients with no plumbing. `event:player_declared` and `event:declarations_resolved` fire from the same call sites on the same latch. This satisfies the epic's same-commit contract-file AC vacuously; state that explicitly rather than inventing an event to satisfy it.

**The real work is the default arms.** Six `switch`/if-chains treat an unrecognized phase as "do nothing": `handleTimerExpiry:1564`, `botDecisionSeats:104`, `HandleDisconnect:105`, the three timer-arm guards, plus `rules_engine.go`'s `ErrWrongPhase` default. None fails loudly. A phase that is correct in the engine and missing from any one of these is a table that silently freezes — which is why the bot simulation and the timer/reconnect tests carry more weight here than the detection logic.

**Ordering at the handoff.** `resolveDeclarationsForHand` sets `DeclarationsResolved` before the phase flips to `PhasePlaying`, so `broadcastDeclarationsResolvedIfTransition`'s `false→true` latch fires on the same action that starts trick 1. The client receives the reveal and the playing-phase `match_state` in that order, which is what `declRevealReady` expects.

## Verification

**Commands:**

- `cd server && go test ./internal/game/...` -- expected: all pass, including untouched Bitola declaration tests.
- `cd server && go test ./internal/bot/... -run Simulation -count=5` -- expected: stable; no `unexpected phase` fatal, no panic.
- `cd server && go test ./...` -- expected: no regressions in `match`, `bot`, `ws`.
- `cd server && git diff --stat HEAD -- internal/ws/testdata/events/` -- expected: empty.
- `cd server && golangci-lint run ./... && gofmt -l .` -- expected: clean.
- `cd client && npx vitest run` -- expected: all pass, including `wsEvents.contract.test.ts` and `i18n.parity.test.ts`.
- `cd client && npx tsc -p tsconfig.build.json --noEmit` -- expected: clean. Not run in CI — run it manually.
- `cd client && npx eslint . && npx prettier --check .` -- expected: clean.
- `cd client && git diff --stat HEAD -- src/shared/i18n/ src/features/match/lib/variantRules.ts` -- expected: empty.

**Manual checks (if no CLI):**

- Grep `server/internal/game` for the variant constants — expected: `RulesFor` only.
- Grep the match, bot, and reconnect packages for each phase switch listed in the Code Map — expected: every one names the new phase.
- Confirm the create-room Croatian option is still `disabled` and the server allowlist still rejects it.

## Suggested Review Order

**Phase entry — the rule selector (start here)**

- The whole story branches here; Bitola's block below is untouched.
  [`bidding.go:180`](../../server/internal/game/bidding.go#L180)

- Config field, not a variant name — D-VAR-1's only sanctioned reading.
  [`types.go:218`](../../server/internal/game/types.go#L218)

**The phase state machine**

- The cursor opens at the seat that will lead trick 1.
  [`declarations.go:331`](../../server/internal/game/declarations.go#L331)

- Walks counter-clockwise, steps past meld-less seats, resolves into trick 1.
  [`declarations.go:358`](../../server/internal/game/declarations.go#L358)

- Action dispatch; the `default` arm made an omission a silent hang.
  [`rules_engine.go:52`](../../server/internal/game/rules_engine.go#L52)

- Value-typed and `json:"-"`: no clone line, no wire widening.
  [`state.go:145`](../../server/internal/game/state.go#L145)

- Pause and surrender allowlists — a phase without either is a hole.
  [`pause.go:14`](../../server/internal/game/pause.go#L14)

**Session manager — the silent defaults**

- Timer arming: without this the prompt has no clock at all.
  [`live_match.go:469`](../../server/internal/match/live_match.go#L469)

- Auto-skip on expiry; this action can never be engine-rejected.
  [`live_match.go:1541`](../../server/internal/match/live_match.go#L1541)

- Defensive re-arm, now loud and with a refreshed deadline (review fix).
  [`live_match.go:1554`](../../server/internal/match/live_match.go#L1554)

- The reveal broadcast a no-meld hand was silently losing (review fix).
  [`live_match.go:905`](../../server/internal/match/live_match.go#L905)

- Disconnects during the phase previously opened no reconnect window at all.
  [`reconnect.go:97`](../../server/internal/match/reconnect.go#L97)

**Bot participation**

- Schedules only the prompted seat; the empty default stalled the table.
  [`bot_driver.go:100`](../../server/internal/match/bot_driver.go#L100)

- Guard against `chooseCard`'s `legal[0]` panic; unreachable by construction.
  [`bot.go:41`](../../server/internal/bot/bot.go#L41)

**Client**

- Closed union, so omitting the member would have been a `tsc` error.
  [`matchTypes.ts:19`](../../client/src/shared/types/matchTypes.ts#L19)

- One derived predicate covers seat ring, pause, surrender and emotes.
  [`MatchPage.tsx:1425`](../../client/src/features/match/MatchPage.tsx#L1425)

**Tests & docs**

- Proves the reveal fires and precedes the trick-1 state (review fix).
  [`declaration_phase_test.go:302`](../../server/internal/match/declaration_phase_test.go#L302)

- The pause/disconnect/reconnect matrix the epic called this story's risk.
  [`declaration_phase_test.go:475`](../../server/internal/match/declaration_phase_test.go#L475)

- Full Croatian hands with bot seats — the deadlock proof.
  [`simulation_test.go:235`](../../server/internal/bot/simulation_test.go#L235)

- First fixture reaching a Croatian post-bidding, pre-trick-1 state.
  [`fixtures.go:797`](../../server/internal/game/testfixtures/fixtures.go#L797)

## Suggested Review Order

**Rule selection — start here**

- The whole story in one branch: dedicated timing opens the phase, Bitola's block is untouched.
  [`bidding.go:180`](../../server/internal/game/bidding.go#L180)

- The phase's three moving parts: dispatch, open at the dealer's left, walk the cursor.
  [`declarations.go:305`](../../server/internal/game/declarations.go#L305)

- Resolution and handoff — the fourth answer resolves and opens trick 1 in one action.
  [`declarations.go:358`](../../server/internal/game/declarations.go#L358)

- Answer progress is server-only and value-typed, so nothing widens and no clone line is needed.
  [`state.go:145`](../../server/internal/game/state.go#L145)

- Without this case the phase rejects every action silently, not at compile time.
  [`rules_engine.go:52`](../../server/internal/game/rules_engine.go#L52)

**The default arms — where this story could actually freeze a table**

- Auto-skip now covers both timings; the prompted seat is always the active seat.
  [`live_match.go:1541`](../../server/internal/match/live_match.go#L1541)

- The unreachable shape re-arms loudly and refreshes the deadline instead of returning silently.
  [`live_match.go:1554`](../../server/internal/match/live_match.go#L1554)

- The auto-action chain may now continue through the phase rather than breaking out.
  [`live_match.go:1654`](../../server/internal/match/live_match.go#L1654)

- Bots are scheduled for the prompted seat only, which is what keeps the guard below unreachable.
  [`bot_driver.go:100`](../../server/internal/match/bot_driver.go#L100)

- Returns before the card ladder; `LegalCards` is nil here and `chooseCard` would panic.
  [`bot.go:41`](../../server/internal/bot/bot.go#L41)

- A drop during the phase previously opened no reconnect window at all.
  [`reconnect.go:97`](../../server/internal/match/reconnect.go#L97)

**Wire — the reveal is the only thing that crosses**

- The reveal latch is consumed here on a meld-less hand, so this arm has to emit it.
  [`live_match.go:905`](../../server/internal/match/live_match.go#L905)

- Ordering contract: the reveal rides ahead of the state that opens trick 1.
  [`declaration_phase_test.go:130`](../../server/internal/match/declaration_phase_test.go#L130)

**Client — reacts to the phase, never to the variant**

- One derived predicate feeds the seat ring, both HUD clusters, pause, surrender and emotes.
  [`MatchPage.tsx:1425`](../../client/src/features/match/MatchPage.tsx#L1425)

- The closed union must carry the new value or every comparison is a `tsc` error.
  [`matchTypes.ts:19`](../../client/src/shared/types/matchTypes.ts#L19)

**Peripherals**

- Entry from both Croatian bidding paths, with the Bitola control beside it.
  [`declarations_test.go:1564`](../../server/internal/game/declarations_test.go#L1564)

- The deadlock proof: a full Croatian hand driven through the shared bot loop.
  [`simulation_test.go:206`](../../server/internal/bot/simulation_test.go#L206)

- The first fixture producing a Croatian post-bidding, pre-trick-1 state.
  [`fixtures.go:790`](../../server/internal/game/testfixtures/fixtures.go#L790)

Note: the `## Code Map` anchors above are pre-change positions captured during planning; the code this story added has shifted several by a few lines.
