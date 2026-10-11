---
title: 'Fix in-game card flight animations'
type: 'bugfix'
created: '2026-05-10'
status: 'done'
context:
  - '{project-root}/_bmad-output/project-context.md'
baseline_commit: 'b3d0e643a5af4d2afd2beed372b1786d466e8967'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The four card-flight animations during a trick all read wrong. (1) Self-throw exits straight down with no convergence to center and no size morph, so it doesn't feel like the card is "going to the table." (2) Opponent throws emerge from arbitrary points inside the trick area, not from each opponent's `CardBackStack` next to their avatar. (3) Self-take of the resolved trick almost never animates — the cards disappear instantly. (4) Opponent-take also doesn't read as flowing toward the winner's deck because it uses the same arbitrary trick-local offsets as the throw.

**Approach:** (a) Render every card flight on a viewport-fixed `CardFlight` overlay anchored to the actual DOM rects of the source (hand card, opponent deck, trick slot) and destination (trick slot, winner's deck, screen-bottom-center). (b) Snapshot the resolved trick into the store on `event:trick_resolved` so the collect animation always has all four cards even when React batches the preceding `event:card_played` with it.

## Boundaries & Constraints

**Always:**
- Server stays authoritative for game state. Animations are pure presentation, driven by state transitions; no client-side game logic added.
- Respect `useReducedMotion()` — reduced-motion still skips flights and snaps cards into final positions exactly as today.
- Card rendering stays in a single `PlayingCard` component. New flight overlay reuses it; no parallel card markup.
- Source/destination rects come from `getBoundingClientRect()` of `data-testid` anchors (`hand-card-{id}`, `player-seat-{n}` deck stack, `trick-slot-{compass}`). No hard-coded viewport-pixel offsets for seat positions.
- Animation tokens stay centralised in `client/src/shared/lib/motion.ts`. New durations get named constants; do not inline magic numbers.

**Ask First:**
- If the chosen approach forces breaking changes to `TrickArea`'s public props or to `gameStore`'s shape beyond adding `pendingResolvedTrick`, surface the diff before committing.
- If the per-flight DOM measurement causes layout-thrash regressions on lower-end devices, raise it before adding `requestAnimationFrame` batching.

**Never:**
- Don't touch the server, WebSocket contract, or `wsEvents.ts` schemas. The existing `event:trick_resolved` payload already carries `cards` + `winnerSeat`; use what's there.
- Don't replace the existing `HandCards` fan layout, `TrickArea` slot diamond, or `PlayerSeat` chrome — only the flight visuals between them.
- Don't introduce a new animation library (framer-motion etc.). Stay with CSS keyframes + transitions.
- Don't auto-award/skip trick collection based on time alone — animation still keys off real state transitions.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Self plays a card | Click on playable card → `flyingCardId` set, `event:card_played` arrives | Card lifts off the hand position, arcs to viewport bottom-center, then continues up to the south trick slot, morphing from `lg` (88×128) to `md` (72×104). One continuous element, no double-paint. | If the source rect can't be read (mount race), fall back to today's slide-down + trick-land sequence. |
| Opponent plays a card | `event:card_played` for seat ≠ self | New card flight starts at that seat's `CardBackStack` rect (small), grows to `md`, lands in their compass trick slot. | If the seat rect can't be measured, fall back to a small offset behind the slot's edge. |
| Self wins trick | `event:trick_resolved` with `winnerSeat === myPlayerSeat`, `currentTrick` may flip 3→0 | All 4 trick cards visibly slide and shrink toward viewport bottom-center (where the hand sits) over `MOTION.TRICK_COLLECT`, then fade. | If `pendingResolvedTrick` is missing (legacy state), use whatever `displayTrick` holds. |
| Opponent wins trick | `event:trick_resolved` with `winnerSeat ≠ myPlayerSeat` | All 4 cards slide and shrink toward winner seat's `CardBackStack` rect, then fade. | Same fallback as above. |
| Reduced motion | `prefers-reduced-motion: reduce` | Cards snap into trick slots and clear instantly on resolve, exactly as today. | N/A |
| Reconnect mid-trick | Snapshot arrives with partial `currentTrick`, no resolution event | Cards render in slots without flight (no in-anim). No collect runs. | N/A |

</frozen-after-approval>

## Code Map

- `client/src/features/game/components/TrickArea.tsx` — Owns slot positions + incoming/collect animations; today computes `approachFrom`/`collectTo` as hard-coded local offsets. Refactor to read seat rects from the new anchors and to consume `pendingResolvedTrick` for the collect phase.
- `client/src/features/game/components/HandCards.tsx` — Owns the hand-throw exit. Add a `data-testid="hand-card-{id}"` anchor on each card so the flight overlay can read its rect. Keep the existing local fade as the fallback when the overlay isn't active.
- `client/src/features/game/components/PlayerSeat.tsx` / inner `CardBackStack` — Add `data-testid="player-seat-deck-{seat}"` (or expose a ref) on the deck-stack root so the overlay can anchor opponent flights to it.
- `client/src/features/game/components/CardFlight.tsx` (NEW) — Viewport-fixed overlay (`position: fixed`, `pointer-events: none`, top of GamePage z-stack). Renders one `<PlayingCard>` per active flight, animating from a captured source rect to a destination rect with width/height/scale interpolation. Pure presentational; receives flights via props from `GamePage`.
- `client/src/features/game/GamePage.tsx` — Wires the overlay. Captures source/dest rects on play and on resolve (using `requestAnimationFrame` to read after layout). Holds the active-flight queue; clears entries on animation end. Mounts `CardFlight` after all seats/trick area so its anchors exist when measured.
- `client/src/shared/stores/gameStore.ts` — Add `pendingResolvedTrick: { trick: TrickCard[]; winnerSeat: number } | null` plus `setPendingResolvedTrick`. Cleared on `clearGame`.
- `client/src/shared/hooks/useWsDispatch.ts` — In `event:trick_resolved` branch, BEFORE clearing `currentTrick`, snapshot `current.currentTrick` (already has all 4 cards because `event:card_played` ran first) into `pendingResolvedTrick`. The next `event:game_state` (or a TrickArea callback after `TRICK_RESOLVE_PAUSE + TRICK_COLLECT`) clears it.
- `client/src/shared/lib/motion.ts` — Add `CARD_FLIGHT_SELF_THROW`, `CARD_FLIGHT_OPPONENT_THROW`, `CARD_FLIGHT_COLLECT` (or repurpose existing constants); keep names event-shaped, not implementation-shaped.

## Tasks & Acceptance

**Execution:**
- [x] `gameStore.ts` — added `pendingResolvedTrick` field + `setPendingResolvedTrick` setter; included in `clearGame`/`reset` initial state; covered by 3 new tests in `gameStore.test.ts`.
- [x] `useWsDispatch.ts` — `EVENT_TRICK_RESOLVED` snapshots `current.currentTrick` into `pendingResolvedTrick` before clearing the live trick. Covered by two new tests in `useWsDispatch.test.ts` (snapshot fires; empty-trick edge case suppresses).
- [x] `HandCards.tsx` — `data-testid="hand-card-{id}"` per card root + `visibility: hidden` while a flight paints the same card.
- [x] `PlayerSeat.tsx` — `CardBackStack` now exposes `id="player-seat-deck-{seat}"` and `data-seat-deck="{seat}"` for `getBoundingClientRect()` lookups. Self-side anchors to the `hand-cards` container (already had a `data-testid`).
- [x] `CardFlight.tsx` (NEW) — viewport-fixed overlay; per-flight `<style>` block + scoped `@keyframes`; transform-only animation (no layout thrash). `animationend` listener attached natively via `addEventListener` in a `useEffect` (React 19 + jsdom synthetic-event delegation was unreliable for this event type). Co-located test: 4 cases including the no-flights / mount / keyframe-uniqueness / animationend → onComplete path.
- [x] `GamePage.tsx` — flights wired for all four scenarios: self play (in `handlePlayCard`), self auto-play (mirrors the auto-played-card effect), opponent throw (effect watching `currentTrick.length`), trick collect (effect watching `pendingResolvedTrick` after `MOTION.TRICK_RESOLVE_PAUSE`). Reduced-motion users skip flights and clear the snapshot after a short hold. `handleFlightComplete` clears the snapshot when the last collect flight ends.
- [x] `TrickArea.tsx` — rewrote: removed the local `trickLand` keyframe + `resolving`/`collecting` state machine; reads `pendingResolvedTrick` for the resolve-glow phase; filters `suppressedCardIds` so the overlay is the only painter for in-flight cards; exports `SLOT_POSITIONS`, `TRICK_SLOT_W`, `TRICK_SLOT_H` for the rect math.
- [x] `motion.ts` — added `CARD_FLIGHT_SELF_THROW`, `CARD_FLIGHT_OPPONENT_THROW`, `CARD_FLIGHT_COLLECT`. `FLAG_LIFETIME.FLYING_CARD` now uses `Math.max` of the legacy throw and the new self-throw flight so the flag outlives whichever runs.
- [x] Tests — `CardFlight.test.tsx` (new), `TrickArea.test.tsx` rewritten for the new prop surface, `gameStore.test.ts` + `useWsDispatch.test.ts` extended. Full client suite: 699 → 707 passing tests (8 new). The "3→0 trick still animates" assertion lives in the dispatcher test (snapshot captured) — TrickArea's prop-driven test confirms the snapshot path renders the four cards.

**Acceptance Criteria:**
- Given the local player taps a playable card, when the throw animation runs, then a single visible card element travels from its hand position to the south trick slot, passing through (or near) viewport bottom-center, and visibly scales from `lg` to `md` over `CARD_FLIGHT_SELF_THROW`.
- Given an opponent plays, when the `event:card_played` is dispatched, then a card visibly appears at that opponent's `CardBackStack` rect and travels to their compass trick slot, scaling from deck-size to `md`.
- Given a trick fills (4 cards) and `event:trick_resolved` arrives in the same tick as the 4th `event:card_played`, when React renders, then `pendingResolvedTrick` holds all 4 cards and the collect animation visibly runs for `MOTION.TRICK_COLLECT` before the slots clear.
- Given the local player wins the trick, when collect runs, then all 4 cards converge toward the bottom-center of the screen (hand region) before fading.
- Given an opponent wins the trick, when collect runs, then all 4 cards converge toward that seat's `CardBackStack` rect before fading.
- Given `prefers-reduced-motion: reduce`, when any of the above events fire, then no flight overlay renders and cards snap to/from slots exactly as the current reduced-motion path does today.
- Given a reconnect mid-trick where partial trick state arrives without a resolution event, then no flights run and the trick area paints from the snapshot directly.

## Spec Change Log

### 2026-05-10 — Review patches (no spec changes; code-only auto-fixes)

Three reviewers (Blind Hunter, Edge Case Hunter, Acceptance Auditor) ran against the post-implementation diff and produced 16 actionable findings — all classified as `patch` (no `intent_gap`/`bad_spec` loopback). Patches applied in-place:

- **Self-throw arc (AC1, Acceptance #2):** added `waypointRect` to `CardFlightDescriptor`; self-throws pass through viewport bottom-center via a 3-stop keyframe.
- **Slot-anchor measurement (Always rule, Acceptance #1):** TrickArea always renders `data-testid="trick-slot-{compass}"` (visibility-hidden when occupied); `measureTrickSlotRect` queries it first and falls back to trick-area + offsets only on zero-size measurement.
- **Reconnect: stuck `pendingResolvedTrick` (Edge Case #1):** dispatcher clears the snapshot on every authoritative `event:game_state`.
- **CardBackStack DOM anchor on last trick (Edge Case #3):** wrapper renders even when `count === 0` (visibility hidden) so the collect destination rect stays measurable.
- **Containing-block ancestors (Blind #1):** `CardFlight` measures its own viewport offset via `useLayoutEffect` and subtracts it from each flight rect.
- **Stale `gameState` closure in `handlePlayCard` (Blind #10 / Edge #7):** read via `useGameStore.getState()` at call time; dropped `gameState` from `useCallback` deps.
- **Legacy `handThrow` keyframe vs. visibility-hidden (Blind #9 / Acceptance #4):** removed the keyframe entirely from `HandCards`; the overlay is now the sole source of motion. `flyingCardId` only drives `visibility: hidden + pointer-events: none`.
- **`queueMicrotask` → direct `setState` (Blind #5):** `handleFlightComplete` now relies on React 18+ event-handler batching.
- **`playedByCompass` derived from `renderableTrick` (Blind #6 / Acceptance #5):** the dashed placeholder is visible during a flight so the slot reads as a silhouette.
- **`compassOffset` deduplication (Acceptance #6):** `GamePage` imports the typed `compassOffset` from `TrickArea`.
- **`pendingResolvedTrick` array shallow-clone (Acceptance #7):** snapshot owns its own array.
- **`animationend` target guard (Edge #6):** `e.target !== el` early-return defends against descendant animations bubbling.
- **Collect-flight id uniqueness (Edge #5):** id embeds `gameState.trickNumber` to avoid `Date.now()` collisions between back-to-back tricks.
- **Reduced-motion glow timing (Edge #10):** snapshot held for `MOTION.TRICK_RESOLVE_PAUSE` instead of `MOTION.REVEAL_PANEL`.
- **Collect fade tail (Blind #3):** new `endOpacity` field on the descriptor; collect flights set `endOpacity: 0` so cards fade as they reach the winner's pile (no 1-frame empty-slot flicker before unmount).
- **rAF-deferred measurements (Acceptance #3):** opponent-throw and auto-play flights schedule rect reads via `requestAnimationFrame` so the slot anchor is laid out before measurement.

Rejected findings (not real defects):
- Edge Case Hunter #2 / Blind #4 — `prevTrickLenRef` reset on hand transitions: ref is unconditionally written to `trick.length` at the end of every effect run.
- Edge Case Hunter #4 — out-of-order events on a single WebSocket connection: not a real risk.
- Edge Case Hunter #8 — synchronous measurement during initial render: existing zero-size guard handles it.
- Edge Case Hunter #9 — opponent-throw double-fire on self-play: already filtered by the seat check.
- Blind Hunter #7 — snapshot vs. `payload.cards`: intentional; local store is authoritative for the four-card composition.
- Blind Hunter #8 — out-of-range compass: theoretical; server validation enforces `seat ∈ [0, 3]`.

KEEP for future iterations:
- The `pendingResolvedTrick` snapshot pattern is load-bearing for the same-tick `event:card_played` + `event:trick_resolved` race — do not collapse it into a derived value.
- Native `addEventListener` for `animationend` in `FlightCard` works around React 19 + jsdom synthetic-event flake — do not switch back to `onAnimationEnd` (test suite will silently break).
- Slot anchors must always be in the DOM (visibility-hidden when occupied) for the overlay's measurement to work.

## Design Notes

**Why a single overlay (not per-component animations):** Today's split between `HandCards.handThrow` and `TrickArea.trickLand` runs two separate elements with overlapping timelines. The handoff at the bottom of the screen is implicit — there's no shared element, so any timing or geometry drift becomes visible as a "stutter" or "second card." A single viewport-fixed flight element guarantees continuity by construction.

**Why measure rects (not configure offsets):** Seat positions today are CSS-class-driven (`bottom-44`, `right-16`) and the table is responsive. Hard-coded `approachFrom: { x: 420, y: 0 }` etc. only happens to look "kinda right" at one viewport size; on smaller screens the gap widens. Reading rects from `data-testid` anchors keeps the geometry truthful at any size.

**Why snapshot via `pendingResolvedTrick`:** The existing `if (trick.length === 0 && prevLen === 4)` collect branch in `TrickArea` only fires when displayTrick had observed all 4 cards. When `event:card_played #4` and `event:trick_resolved` are processed in the same JS tick, React batches both store updates into one render — `currentTrick` jumps 3→0 and the existing "force-sync" branch wipes the display silently. The snapshot decouples animation input from `currentTrick`'s authoritative-but-transient nature.

**Rect measurement timing:** Schedule with `requestAnimationFrame` after the state change that triggers the flight (so the source/dest elements are laid out). Cache the measured rects on the flight object — don't re-read mid-flight.

## Verification

**Commands:**
- `make lint` — expected: clean (no new ESLint warnings; Prettier formatted).
- `make test` — expected: all existing + new tests pass (`vitest` + `go test ./...`).
- `npm run dev` (frontend) and exercise: play through one full hand against bots/test peers; observe each of the four flight scenarios.

**Manual checks:**
- Throw a card: a single card visibly travels from hand → bottom-center → south slot, with size morph; no double-paint glitch.
- Each opponent throws: card visibly originates from their `CardBackStack` (next to their avatar), grows, lands in their compass slot.
- Win a trick as self: 4 cards converge toward the hand area before disappearing; the collect is visible for ~600 ms even when the server is fast.
- Win a trick as each opponent: 4 cards converge toward that opponent's deck stack before disappearing.
- Toggle OS reduced-motion on; repeat — no flights, instant transitions, parity with the pre-change reduced-motion behaviour.
- Resize the browser between flights; re-measure on next play; no off-screen / wildly-misaligned flights.
