---
title: "Timer auto-actions across all turn-blocking dialogs + in-dialog timer ring"
type: "feature"
created: "2026-05-02"
status: "done"
baseline_commit: "a48c15fd0643718ea9b22506a99a8a2d9388567e"
context:
  - "{project-root}/_bmad-output/project-context.md"
  - "{project-root}/_bmad-output/implementation-artifacts/4-5-per-move-timer-and-auto-play.md"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When the per-move timer expires while a turn-blocking modal is on screen (TrumpPrompt, BelotPrompt, DeclarationPrompt, ScoreReveal), (a) the active player's `TimerRing` lives on `PlayerSeat` and is hidden behind the modal, so the player sees no countdown next to the action they need; (b) `ScoreReveal` never auto-dismisses, so an AFK player blocks their own view of the next hand even after the server already auto-passed; (c) only auto-played cards announce themselves via toast — auto-pass / auto-skip-declare / auto-skip-belot are silent.

**Approach:** Render a compact `TimerRing` variant inside each prompt around its negative-action button (Pass / Decline / Skip). Auto-dismiss `ScoreReveal` on a fixed timer matching `DeclarationReveal` (8000ms / 1500ms reduced-motion). Server emits a new typed `event:auto_action` for `pass_trump` / `skip_declare` / `skip_belot` expiries; client surfaces an info toast naming the player. Card auto-play wire format (`event:card_played` + `autoPlayed: true`) is unchanged.

## Boundaries & Constraints

**Always:**
- Server is the only authority for the auto-action choice. New event is informational; authoritative state still rides `event:game_state`.
- Card auto-play rule unchanged: from `legalCards(state, seat)`, sort by suit (S → H → D → C) then rank (7 → 8 → 9 → T → J → Q → K → A); play first.
- In-dialog `TimerRing` reads `gameState.turnExpiresAt` / `timerDurationSec` — no parallel timer state.
- Dialog ring renders only when viewer **is** the active player AND room is per-move AND `turnExpiresAt !== null`.
- Both contract files (`wsEvents.ts` + `events.go`) updated in the same commit.
- New strings keyed in both `en.json` and `sr.json`.

**Ask First:**
- Replacing the existing `autoPlayed` flag on `event:card_played` (don't — keep both signals).
- Any change to backend `handleTimerExpiry` action selection beyond emitting the new event.

**Never:**
- Move auto-action selection logic to the client.
- Couple `ScoreReveal` auto-dismiss to `turnExpiresAt`; use the fixed 8000/1500 ms timer.
- Drop the existing 2s "Continue disabled" cushion.
- Add `TimerRing` to `TrumpReveal`, `BelotReveal`, `DeclarationReveal`, `MatchResult` — those self-close and aren't turn-gated.

## I/O & Edge-Case Matrix

| Scenario | State | Behavior |
|---|---|---|
| Bidder AFK, round 1 OR round 2 | `phase=bidding` | Server `pass_trump` → `event:auto_action {type:"pass_trump", playerSeat}` → `event:game_state`. All clients toast "Auto-passed: {player}" |
| Active player AFK at declaration prompt | `phase=playing`, `awaitingDeclaration` | `skip_declare` → `event:auto_action {type:"skip_declare"}` → state. Toast "Auto-skipped declaration: {player}" |
| Belote prompt holder AFK | `pendingBelotSeat==active` | `skip_belot` → `event:auto_action {type:"skip_belot"}` → state. Toast "Auto-declined Belote: {player}" |
| Active player AFK during regular play | `phase=playing`, no prompt | Unchanged: existing `event:card_played {autoPlayed:true}` + existing toast. **No** `event:auto_action`. |
| ScoreReveal sits 8s without click | `scoreRevealData != null` | All clients unmount overlay (`onContinue` fires). Reduced-motion: 1500ms |
| Manual Continue ≥2s | User clicks Continue | Existing path; auto-dismiss timer cleared on unmount |
| Relaxed timer mode | `timerStyle=="relaxed"` | No in-dialog ring rendered; server never fires expiry |

</frozen-after-approval>

## Code Map

- `server/internal/session/manager.go` — `handleTimerExpiry` (~lines 861-960) selects the auto-action; emit `event:auto_action` *before* `broadcastActionResult` for the three non-card cases.
- `server/internal/ws/events.go` — add `EventAutoAction` constant + `AutoActionPayload { PlayerSeat int; Type string }`.
- `server/internal/session/manager_test.go` — extend timer-expiry tests.
- `client/src/shared/types/wsEvents.ts` — mirror constant + `AutoActionType` union + payload interface.
- `client/src/shared/hooks/useWsDispatch.ts` — handle `EVENT_AUTO_ACTION`, fire toast keyed on `type`.
- `client/src/features/game/components/TimerRing.tsx` — add `size?: "seat" | "button"` (default `"seat"`).
- `client/src/features/game/components/{TrumpPrompt,BelotPrompt,DeclarationPrompt}.tsx` — accept `turnExpiresAt`, `timerDurationSec`; render `<TimerRing size="button">` overlaying Pass / Decline / Skip when active.
- `client/src/features/game/components/ScoreReveal.tsx` — fixed-timeout `useEffect` calling `onContinue`.
- `client/src/features/game/GamePage.tsx` — thread timer props into the three prompts.
- `client/src/shared/i18n/{en,sr}.json` — three new keys.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/ws/events.go` -- add `EventAutoAction` + `AutoActionPayload`
- [x] `server/internal/session/manager.go` -- in `handleTimerExpiry`, after `action` is built and **before** `broadcastActionResult`, broadcast `EventAutoAction {PlayerSeat: expectedSeat, Type: <"pass_trump"|"skip_declare"|"skip_belot">}` for those three cases. Card path unchanged.
- [x] `server/internal/session/manager_test.go` -- table-driven: bidding/declaration/belote expiry asserts the new event fires with right payload; card-play expiry asserts it does NOT
- [x] `client/src/shared/types/wsEvents.ts` -- `EVENT_AUTO_ACTION`, `AutoActionType = "pass_trump" | "skip_declare" | "skip_belot"`, `AutoActionPayload`
- [x] `client/src/shared/hooks/useWsDispatch.ts` -- handler: defensive payload check; resolve `players[playerSeat].username`; toast.info with i18n key per `type`, `duration: 3000`
- [x] `client/src/shared/hooks/useWsDispatch.test.ts` -- assert toast fires for each type; suppressed when `gameState === null`
- [x] `client/src/features/game/components/TimerRing.tsx` -- add `size?: "seat" | "button"` (button: `size=36`, `strokeWidth=2`, `text-[10px]`)
- [x] `client/src/features/game/components/TrumpPrompt.tsx` -- accept `turnExpiresAt`, `timerDurationSec`; wrap Pass button in `relative inline-block`; overlay `<TimerRing size="button">` when `isActiveBidder && turnExpiresAt`
- [x] `client/src/features/game/components/BelotPrompt.tsx` -- accept `turnExpiresAt`, `timerDurationSec`; same overlay around Decline
- [x] `client/src/features/game/components/DeclarationPrompt.tsx` -- accept `turnExpiresAt`, `timerDurationSec`; same overlay around Skip
- [x] `client/src/features/game/components/ScoreReveal.tsx` -- second `useEffect`: `setTimeout(onContinue, prefersReducedMotion ? 1500 : 8000)`; `clearTimeout` on unmount; existing 2s disable-cushion stays
- [x] `client/src/features/game/GamePage.tsx` -- pass `gameState.turnExpiresAt` and `gameState.timerDurationSec` to the three prompts
- [x] `client/src/features/game/components/TimerRing.test.tsx` -- cases for `size="button"` rendering
- [x] `client/src/features/game/components/{TrumpPrompt,BelotPrompt,DeclarationPrompt}.test.tsx` -- assert in-dialog ring renders only when active + `turnExpiresAt` set; not in relaxed mode
- [x] `client/src/features/game/components/ScoreReveal.test.tsx` -- `vi.useFakeTimers()`: advance 8000ms → `onContinue` fired; advance 7999ms → not yet. Reduced-motion mock: 1500ms behavior
- [x] `client/src/shared/i18n/{en,sr}.json` -- `game.timer.autoPassed`, `game.timer.autoSkippedDeclare`, `game.timer.autoSkippedBelot` with `{{player}}`

**Acceptance Criteria:**
- Given a per-move bidder is AFK in round 1 or round 2, when the timer reaches 0, then `event:auto_action {type:"pass_trump"}` precedes the next `event:game_state` and a toast naming the player appears for all 4 clients.
- Given the active player is AFK at the declaration prompt or holding a belote prompt, when the timer expires, then the matching `event:auto_action` fires, the prompt unmounts, and the toast appears.
- Given any of the three prompts is open AND viewer is the active player AND room is per-move, then a small `TimerRing` overlay is centered on the negative-action button. In relaxed-timer rooms no overlay renders.
- Given a `ScoreReveal` is shown, when 8000ms elapse without interaction (1500ms reduced-motion), then `onContinue` fires on all clients.
- Given a card auto-play, then no `event:auto_action` is emitted; the existing `autoPlayed: true` flag and "Auto-played: {card}" toast remain unchanged.
- `make lint` and `make test` pass.

## Design Notes

**Why a new event, not a flag on `event:game_state`?** Game-state broadcasts are authoritative snapshots; a one-shot "auto fired" flag creates ambiguity on reconnect/re-broadcast. A small standalone event mirrors `event:trump_selected` / `event:belot_announced` precedent for ephemeral one-shot info.

**Why fixed 8000/1500 ms on `ScoreReveal`, not `turnExpiresAt`-driven?** Bidder timer varies per room (10s vs 60s); tying the modal to it is non-uniform and closes instantly when the bidder picks/passes quickly. 8s mirrors `DeclarationReveal` precedent the user approved.

**In-dialog ring positioning:** wrap the action button in `<span class="relative inline-block">`, render the button, render `<TimerRing size="button">` as a sibling with `absolute inset-0 pointer-events-none`. No layout shift. Reduced-motion behavior of `TimerRing` carries over unchanged.

## Verification

**Commands:**
- `cd "d:/My Projects/beljot/server" && go test ./internal/session/... ./internal/ws/...` — all pass
- `cd "d:/My Projects/beljot/client" && npx vitest run` — all pass
- `cd "d:/My Projects/beljot" && make lint` — clean
- `cd "d:/My Projects/beljot/client" && npx prettier --write .` — clean diff

**Manual checks:**
- In a per-move room (short duration like 10s), reach a TrumpPrompt and let it expire — toast on all 4 clients, modal unmounts.
- Reach hand-end and let `ScoreReveal` sit — auto-dismisses at 8s on all clients.
- Toggle `prefers-reduced-motion` in browser → repeat → dismisses at 1.5s.
- Switch room to relaxed timer → no in-dialog ring, no auto-action toasts.

## Suggested Review Order

**Wire contract (start here)**

- New informational event for non-card auto-actions; card auto-play untouched.
  [`events.go:44`](../../server/internal/ws/events.go#L44)

- Typed payload + Go-side enum-as-string-alias.
  [`events.go:85`](../../server/internal/ws/events.go#L85)

- TS-side mirror — literal-union type, no enum, defensive Zod schema with seat range.
  [`wsEvents.ts:81`](../../client/src/shared/types/wsEvents.ts#L81)
  [`wsEvents.schemas.ts:232`](../../client/src/shared/types/wsEvents.schemas.ts#L232)

**Server: auto-action emission**

- Helper maps action type → wire `AutoActionType`; `play_card` returns `(_, false)` so card path stays on existing `autoPlayed` flag.
  [`manager.go:967`](../../server/internal/session/manager.go#L967)

- Emit `event:auto_action` ahead of `broadcastActionResult` for the three non-card cases.
  [`manager.go:947`](../../server/internal/session/manager.go#L947)

**Client: toast and defensive validation**

- Dispatcher: `Number.isInteger` + seat-range guard, then i18n toast.
  [`useWsDispatch.ts:230`](../../client/src/shared/hooks/useWsDispatch.ts#L230)

- New i18n keys (en + sr).
  [`en.json:307`](../../client/src/shared/i18n/en.json#L307)
  [`sr.json:307`](../../client/src/shared/i18n/sr.json#L307)

**Client: in-dialog timer ring**

- Compact size variant (36×36, smaller stroke/label) reuses the seat-variant rendering path.
  [`TimerRing.tsx:8`](../../client/src/features/game/components/TimerRing.tsx#L8)

- TrumpPrompt — explicit `(active && per-move && expiry)` guard; ring sibling next to Pass button.
  [`TrumpPrompt.tsx:47`](../../client/src/features/game/components/TrumpPrompt.tsx#L47)

- BelotPrompt — same guard with defensive `isActivePlayer` prop (default true).
  [`BelotPrompt.tsx:31`](../../client/src/features/game/components/BelotPrompt.tsx#L31)

- DeclarationPrompt — same pattern around Skip.
  [`DeclarationPrompt.tsx:43`](../../client/src/features/game/components/DeclarationPrompt.tsx#L43)

- GamePage threads `turnExpiresAt` + `timerDurationSec` into the three prompts.
  [`GamePage.tsx:614`](../../client/src/features/game/GamePage.tsx#L614)

**Client: ScoreReveal auto-dismiss**

- Ref-based `onContinue` so parent re-renders (e.g. `match_end` mid-reveal) don't reset the 8s clock.
  [`ScoreReveal.tsx:30`](../../client/src/features/game/components/ScoreReveal.tsx#L30)

**Tests (peripheral)**

- Wire-format mapping is the contract surface; locked here.
  [`auto_action_test.go:18`](../../server/internal/session/auto_action_test.go#L18)

- Bidding-phase expiry advances the seat without breaking the new broadcast path.
  [`auto_action_test.go:48`](../../server/internal/session/auto_action_test.go#L48)

- Drift gate adds the new payload to the parsed-golden table.
  [`wsEvents.contract.test.ts:31`](../../client/src/shared/types/wsEvents.contract.test.ts#L31)

- Regression guard for the ScoreReveal ref pattern.
  [`ScoreReveal.test.tsx:196`](../../client/src/features/game/components/ScoreReveal.test.tsx#L196)

- Dispatcher NaN/non-integer rejection.
  [`useWsDispatch.test.ts:872`](../../client/src/shared/hooks/useWsDispatch.test.ts#L872)
