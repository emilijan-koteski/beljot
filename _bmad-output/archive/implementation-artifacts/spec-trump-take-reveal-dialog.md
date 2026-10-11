---
title: "Trump-take reveal dialog"
type: "feature"
created: "2026-04-26"
status: "done"
baseline_commit: "4b17cc7e59e76a590beb9510fb9ad32f3b708506"
context:
  - "{project-root}/_bmad-output/project-context.md"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When a player takes trump (round 1 or round 2 in Bitola), nothing announces it: the bidding UI just disappears and the persistent `TrumpIndicator` lights up showing only the suit + caller team/name. Other seats can't see the originally face-up `trumpCandidate` card the picker absorbed, so they have to remember it from the deal animation.

**Approach:** Add a brief, table-wide reveal dialog that fires on `event:trump_selected`, showing the picker's username and the actual face-up card they took (the `trumpCandidate` that was just folded into their hand). Auto-dismiss after a few seconds. The persistent `TrumpIndicator` stays unchanged — it already shows only the suit + caller name — so the post-dismiss state already meets the requirement.

## Boundaries & Constraints

**Always:**

- Send the `trumpCandidate` card on the `event:trump_selected` payload itself (read it from `oldState.TrumpCandidate.String()` in `manager.go` _before_ the candidate is cleared by `ApplyAction`). The follow-up `event:game_state` no longer carries the candidate — clients cannot derive it after the fact.
- Update **both** WS contract files in the same change: `server/internal/ws/events.go` (introduce `TrumpSelectedPayload` typed struct) and `client/src/shared/types/wsEvents.ts` (extend `TrumpSelectedPayload` with `cardId: string`).
- The dialog is informational and non-blocking: `pointer-events-none`, no full-screen backdrop, must not interfere with the immediately-following `playing`-phase UI (active player can play their card while the reveal is on screen).
- Auto-dismiss timing matches existing reveal components in spirit: ~3.5 s normal, 1.5 s `prefers-reduced-motion`. State is held in `gameStore` (`trumpReveal: TrumpSelectedPayload | null`) and cleared by the component's `onComplete` — same pattern as `declarationReveal` / `belotReveal`.
- Payload guard in `useWsDispatch`: drop the reveal silently if `cardId` is missing/malformed (`length < 2`), mirroring the `EVENT_BELOT_ANNOUNCED` guard.
- `TrumpIndicator` keeps its current behavior — suit + team chip + caller name only, no card. Do not extend it.
- Add i18n keys to **both** `en.json` and `sr.json` (Serbian Latin); `i18n.test.ts` parity is enforced.

**Ask First:**

- None — payload extension is additive (existing `playerSeat` + `trumpSuit` keys preserved), so no breaking changes for any consumer.

**Never:**

- Do not extend `GameState` or persist the candidate after pick — the candidate is intentionally cleared by `handlePickTrump` so it can't leak into post-pick logic. Carry it on the event only.
- Do not use the seat-anchored `PANEL_POSITIONS` map from `BelotReveal`/`DeclarationReveal`. The dialog is centered on the table, not pinned to the picker's seat.
- Do not block the underlying game with a modal backdrop or `disabled` overlay — the next player must be able to play immediately.
- Do not show this for `action:pass_trump`. Reveal fires only when a pick succeeds (`event:trump_selected` is already gated on `newState.TrumpSuit != nil`).
- Do not change rules-engine logic, dealer rotation, or round-2 free-suit choice.

## I/O & Edge-Case Matrix

| Scenario                                         | Input / State                                                                                             | Expected Output / Behavior                                                                                                                       | Error Handling                 |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------ |
| Round-1 pick (candidate suit becomes trump)      | `oldState.TrumpCandidate = {7,S}`, `action.PlayerSeat=2` ("Ana")                                          | `event:trump_selected` payload `{playerSeat:2, trumpSuit:"S", cardId:"7S"}`; all 4 clients show centered dialog "Ana took trump" + the `7♠` card | N/A                            |
| Round-2 pick (free-suit; candidate suit ≠ trump) | `oldState.TrumpCandidate = {9,D}`, picker chooses `H`                                                     | Payload `{playerSeat, trumpSuit:"H", cardId:"9D"}`; dialog shows the `9♦` card, persistent `TrumpIndicator` afterward shows `♥`                  | N/A                            |
| Reveal dismiss timing                            | Normal motion                                                                                             | Dialog visible ~3500 ms, then unmounts; `gameStore.trumpReveal` set back to `null`                                                               | N/A                            |
| Reduced-motion user                              | `prefers-reduced-motion: reduce`                                                                          | Dialog visible 1500 ms (mirrors `BelotReveal` / `DeclarationReveal`)                                                                             | N/A                            |
| Malformed payload                                | `cardId` empty or length < 2                                                                              | Dispatch drops the reveal; persistent indicator still updates from follow-up `event:game_state`                                                  | Silent ignore + `console.warn` |
| Pass action                                      | `action:pass_trump`                                                                                       | No `event:trump_selected` emitted (existing behavior); no reveal                                                                                 | N/A                            |
| Reconnect after reveal window                    | Reconnecting client receives only the next `event:game_state` snapshot, not the historic `trump_selected` | No reveal shown for the reconnecter; persistent `TrumpIndicator` already shows correct suit + caller                                             | Acceptable miss                |
| Locale = Serbian Latin                           | `sr` active                                                                                               | Dialog renders Serbian translations                                                                                                              | N/A                            |

</frozen-after-approval>

## Code Map

- `server/internal/ws/events.go` -- add `TrumpSelectedPayload` struct (`PlayerSeat`, `TrumpSuit`, `CardID`) co-located with the other typed payloads.
- `server/internal/session/manager.go` -- `case game.ActionPickTrump`: replace inline `map[string]interface{}` with the new struct, source `CardID` from `oldState.TrumpCandidate.String()` (guarded against nil — should never be nil here, but keep the broadcast safe).
- `server/internal/ws/events_test.go` -- NEW; JSON-contract test for `TrumpSelectedPayload` (locks camelCase field names). Replaces the originally-planned manager_test broadcast assertion since the session-test layer has no broadcast-capture infra.
- `client/src/shared/types/wsEvents.ts` -- extend `TrumpSelectedPayload` with `cardId: string`.
- `client/src/shared/stores/gameStore.ts` -- add `trumpReveal: TrumpSelectedPayload | null` + `setTrumpReveal` setter (mirror `belotReveal`); reset in `initialState`.
- `client/src/shared/hooks/useWsDispatch.ts` -- in `EVENT_TRUMP_SELECTED` branch, validate `payload.cardId.length >= 2` then `store.setTrumpReveal(payload)`. Replace the existing "informational only" no-op.
- `client/src/shared/hooks/useWsDispatch.test.ts` -- add a test: dispatching a well-formed `event:trump_selected` populates `gameStore.trumpReveal`; malformed payload drops it.
- `client/src/features/game/components/TrumpReveal.tsx` -- NEW; centered, non-blocking auto-dismissing panel showing picker's username + the candidate `PlayingCard`. Props `{ playerSeat, players, cardId, onComplete }`.
- `client/src/features/game/components/TrumpReveal.test.tsx` -- NEW; render test, auto-dismiss timer, reduced-motion path, missing-player fallback.
- `client/src/features/game/GamePage.tsx` -- read `trumpReveal` + `setTrumpReveal` from store; render `<TrumpReveal>` when present, key by `${playerSeat}-${cardId}` for clean remount on back-to-back picks (e.g. reshuffle then pick).
- `client/src/shared/i18n/en.json` -- add `game.trumpReveal.title` ("{{name}} took trump") and `game.trumpReveal.unknownPlayer` ("Trump taken").
- `client/src/shared/i18n/sr.json` -- mirror keys (`"{{name}} je uzeo aduta"` / `"Adut uzet"`).

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/ws/events.go` -- introduce typed `TrumpSelectedPayload{ PlayerSeat int, TrumpSuit string, CardID string }` with camelCase JSON tags.
- [x] `server/internal/session/manager.go` -- in the `ActionPickTrump` branch, build `ws.TrumpSelectedPayload{...}` with `CardID: oldState.TrumpCandidate.String()` (only if non-nil; otherwise leave empty and continue). Keep the follow-up `EventGameState` broadcast unchanged.
- [x] `server/internal/ws/events_test.go` -- NEW; JSON-contract test for `TrumpSelectedPayload` locking the `playerSeat` / `trumpSuit` / `cardId` camelCase field names. (Originally planned in `manager_test.go`, but the session-test layer has no broadcast-capture infra; the wire-contract assertion delivers the same regression value with far less surface.)
- [x] `client/src/shared/types/wsEvents.ts` -- add `cardId: string` to `TrumpSelectedPayload`.
- [x] `client/src/shared/stores/gameStore.ts` -- add `trumpReveal` field, `setTrumpReveal` setter, include in `initialState`, reset in `clearGame`/`reset`.
- [x] `client/src/shared/hooks/useWsDispatch.ts` -- replace `EVENT_TRUMP_SELECTED` no-op with `setTrumpReveal(payload)` after `payload.cardId.length >= 2` guard.
- [x] `client/src/shared/hooks/useWsDispatch.test.ts` -- add cases: well-formed payload populates `trumpReveal`; malformed payload leaves it `null`.
- [x] `client/src/features/game/components/TrumpReveal.tsx` -- NEW: centered panel (`absolute top-1/3 left-1/2 -translate-x-1/2 -translate-y-1/2 z-30 pointer-events-none`), `bg-surface-elevated/95 border border-border rounded-lg shadow-lg px-4 py-3`. Renders `t("game.trumpReveal.title", { name })` (or fallback key when player not found in `players`) and a `<PlayingCard size="md">` for the parsed `cardId`. `useEffect` timer: 3500 ms / 1500 ms reduced-motion → `setVisible(false)` + `onComplete()`. `data-testid="trump-reveal"`.
- [x] `client/src/features/game/components/TrumpReveal.test.tsx` -- NEW: (1) renders picker name + card; (2) calls `onComplete` after duration (use fake timers); (3) reduced-motion shortens duration; (4) unknown seat falls back to generic title without crashing.
- [x] `client/src/features/game/GamePage.tsx` -- add `trumpReveal` / `setTrumpReveal` reads, `handleTrumpRevealComplete` callback, render `<TrumpReveal key={...} ...players=... onComplete={handleTrumpRevealComplete} />` alongside the other reveals.
- [x] `client/src/shared/i18n/en.json` -- add `game.trumpReveal.title` = "{{name}} took trump" and `game.trumpReveal.unknownPlayer` = "Trump taken".
- [x] `client/src/shared/i18n/sr.json` -- add `game.trumpReveal.title` = "{{name}} je uzeo aduta" and `game.trumpReveal.unknownPlayer` = "Adut uzet".

**Acceptance Criteria:**

- Given a Bitola hand in `bidding` round 1 with `trumpCandidate = 7♠` and seat 2 ("Ana") picks, when `event:trump_selected` is broadcast, then all four clients render `[data-testid="trump-reveal"]` containing "Ana" and a `7♠` card.
- Given the dialog is shown, when ~3500 ms elapse (or 1500 ms under `prefers-reduced-motion`), then the dialog unmounts and `gameStore.trumpReveal === null`.
- Given the dialog is dismissed, when the player inspects the top-right corner, then `[data-testid="trump-indicator"]` shows the suit (and caller team/name) but no card image — the existing `TrumpIndicator` is unchanged.
- Given a round-2 free-suit pick where the candidate suit differs from the chosen trump suit, when the reveal renders, then it shows the originally face-up `trumpCandidate` card (not a card matching the chosen trump suit).
- Given the user's locale is Serbian, when the dialog renders, then the title uses the Serbian translation.
- Given a malformed `event:trump_selected` payload (`cardId` shorter than 2), when dispatched, then `gameStore.trumpReveal` stays `null` and a `console.warn` is emitted; the persistent `TrumpIndicator` still updates from the follow-up `event:game_state`.

## Spec Change Log

### 2026-04-26 — Iteration 1 patches (no spec amendment)

Three review findings classified as **patch** (auto-fix without re-deriving the spec):

1. **Server defensive logging** — `manager.go` `ActionPickTrump` branch: when `oldState.TrumpCandidate` is unexpectedly nil but `newState.TrumpSuit` is set, suppress the broadcast and `slog.Warn` instead of emitting an empty `cardId` the client would silently drop. Avoids a debugging blind spot.
2. **`parseCardId` boundary guard** — `TrumpReveal.tsx`: add `cardId.length < 2` short-circuit before render. Defence in depth, since the WS-dispatch guard lives in a different module and could be bypassed in tests or future call sites.
3. **z-index reduced from `z-30` to `z-20`** — `TrumpReveal.tsx`: `ScoreReveal` is `fixed inset-0 z-30`; if a user is slow to dismiss the score-reveal and the next hand's trump pick lands while it is still up, both reveals would compete at the same z-level. Lowering matches `BelotReveal`/`DeclarationReveal` and keeps `ScoreReveal` on top.

KEEP instructions for any future re-derivation:

- Auto-dismiss durations 3500 ms / 1500 ms (reduced motion) — do not change.
- `key={playerSeat}-{cardId}` on `<TrumpReveal>` — required for back-to-back-pick remount.
- `cardId` sourcing from `oldState.TrumpCandidate.String()` (pre-`ApplyAction`) is correct — `newState` clears it.

## Design Notes

The reveal piggy-backs the existing post-pick broadcast order — `event:trump_selected` lands before `event:game_state`, so the dialog can mount while the client is still on the bidding screen. By the time the timer fires, the client is already in `playing` phase with the persistent indicator visible. No phase-coupled gating is needed beyond the presence of `trumpReveal`.

Reading the `cardId` off the **event payload** (not store) is deliberate: `handlePickTrump` clears `state.TrumpCandidate` to nil immediately after the pick (it's no longer face-up — the picker has it). Trying to recover it from `gameState` after the follow-up `game_state` is a dead end; the snapshot does not carry it. This is why the wire field is the one source of truth and the spec mandates updating both contract files.

Sketch of the centered panel (analogous to `BelotReveal`'s container, but center-stage):

```tsx
<div
  className="absolute top-1/3 left-1/2 -translate-x-1/2 -translate-y-1/2 z-30 pointer-events-none"
  data-testid="trump-reveal"
>
  <div className="bg-surface-elevated/95 border border-border rounded-lg shadow-lg px-4 py-3 flex flex-col items-center gap-2">
    <p className="text-text-primary font-display text-base font-semibold">
      {title}
    </p>
    <PlayingCard
      card={parseCardId(cardId)}
      state="default"
      size="md"
      withTransition={false}
    />
  </div>
</div>
```

## Verification

**Commands:**

- `cd client && npx prettier --write .` -- expected: formats touched files.
- `cd client && npx vitest run` -- expected: existing suites + new `TrumpReveal.test.tsx`, updated `useWsDispatch.test.ts`, `i18n.test.ts` parity all pass.
- `cd server && go test ./...` -- expected: rules + session tests pass; new manager assertion green.
- `make lint` -- expected: ESLint + Prettier + golangci-lint all green.
- `make test` -- expected: full Go + frontend suite green.

**Manual checks:**

- `make dev`, open four clients in a Bitola room, deal a hand: when one player picks trump (round 1), confirm a centered dialog with the picker's username and the face-up card appears on every client and auto-dismisses after a few seconds.
- Force a round-2 pass-pass-pass-pick path with a free-suit choice that differs from the candidate's suit; confirm the reveal still shows the originally face-up card (not the chosen suit).
- Toggle `prefers-reduced-motion` (DevTools → Rendering → Emulate CSS media feature); confirm the dismiss is faster.
- Switch locale to Serbian; confirm Serbian title.

## Suggested Review Order

**Wire-protocol contract (server → client)**

- The originally face-up `TrumpCandidate` is read off `oldState` _before_ `ApplyAction` clears it, then carried on the event with a defensive `slog.Warn` if the candidate is unexpectedly nil.
  [`manager.go:467`](../../server/internal/session/manager.go#L467)

- New typed broadcast payload replaces the previous inline `map[string]interface{}` and locks `playerSeat` / `trumpSuit` / `cardId` camelCase tags.
  [`events.go:49`](../../../server/internal/ws/events.go#L49)

- TS-side payload type extends with `cardId: string` to mirror the Go struct exactly — no other consumer reads this event today.
  [`wsEvents.ts:116`](../../../client/src/shared/types/wsEvents.ts#L116)

**Client dispatch and store**

- Promotes `EVENT_TRUMP_SELECTED` from a no-op to setting `trumpReveal`, with a `cardId.length < 2` guard mirroring the `EVENT_BELOT_ANNOUNCED` pattern.
  [`useWsDispatch.ts:181`](../../../client/src/shared/hooks/useWsDispatch.ts#L181)

- New `trumpReveal` slot lives next to `belotReveal` / `declarationReveal`; cleared by `clearGame` / `reset` via `initialState` spread.
  [`gameStore.ts:21`](../../client/src/shared/stores/gameStore.ts#L21)

**Reveal component**

- Defence-in-depth `cardId.length < 2` short-circuit before rendering — `parseCardId` is unsafe for short strings.
  [`TrumpReveal.tsx:37`](../../client/src/features/game/components/TrumpReveal.tsx#L37)

- z-index `z-20` (not `z-30`) so `ScoreReveal`'s `fixed inset-0 z-30` always sits on top during hand-end overlap.
  [`TrumpReveal.tsx:48`](../../client/src/features/game/components/TrumpReveal.tsx#L48)

- Auto-dismiss timer matches existing reveal pattern (3.5 s normal, 1.5 s reduced motion) and clears on unmount.
  [`TrumpReveal.tsx:28`](../../client/src/features/game/components/TrumpReveal.tsx#L28)

**Page wiring**

- Reveal is keyed by `${playerSeat}-${cardId}` so back-to-back picks (e.g. reshuffle then pick) remount cleanly.
  [`GamePage.tsx:562`](../../client/src/features/game/GamePage.tsx#L562)

**i18n**

- Two new keys parity-mirrored across English and Serbian Latin; `i18n.test.ts` enforces the symmetric shape.
  [`en.json:218`](../../../client/src/shared/i18n/en.json#L218)

- Serbian-Latin counterparts.
  [`sr.json:218`](../../../client/src/shared/i18n/sr.json#L218)

**Tests**

- Component test covers rendering, fallback for unknown seat, normal auto-dismiss, and reduced-motion auto-dismiss.
  [`TrumpReveal.test.tsx:63`](../../client/src/features/game/components/TrumpReveal.test.tsx#L63)

- Dispatch tests cover happy path and the malformed-cardId guard with `console.warn`.
  [`useWsDispatch.test.ts:387`](../../../client/src/shared/hooks/useWsDispatch.test.ts#L387)

- Wire-format JSON contract test locks the camelCase field names so a future TS/Go drift surfaces immediately.
  [`events_test.go:14`](../../../server/internal/ws/events_test.go#L14)
