---
title: "Dealer indicator and trump-caller player name"
type: "feature"
created: "2026-04-26"
status: "done"
baseline_commit: "f54f6ee97e917d5d717d51f7b15359436da8f937"
context:
  - "{project-root}/_bmad-output/project-context.md"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** During a hand, the UI does not surface who is dealing. The trump-caller badge in `TrumpIndicator` shows only the suit and the calling team's color/name, but not the actual player who picked the trump — players have to remember bidding order to know.

**Approach:** Add a small persistent `DealerIndicator` badge alongside the existing `TrumpIndicator` (top-right corner of the game table) that shows the dealer's username. Extend `TrumpIndicator` to also render the trump-caller's username next to the team chip. Both pieces of data are already in the broadcast `GameState` (`players[dealerSeat].username`, `players[trumpCallerSeat].username`), so this is a frontend-only change — no WS event contract or backend changes.

## Boundaries & Constraints

**Always:**

- Resolve usernames by looking up `gameState.players` by seat — never trust a separate payload field. The `players` array is the single source of truth for username/seat mapping.
- Hide each indicator gracefully when its source data is missing: `DealerIndicator` hides if no player exists at `dealerSeat`; trump-caller name hides if `trumpCallerSeat` is `null` or no matching player is found.
- Match the existing `TrumpIndicator` styling/anchoring conventions (rounded-full pill, team colors via `team-a`/`team-b` Tailwind tokens, `aria-live="polite"`, `data-testid` attributes).
- Add translations to BOTH `en.json` and `sr.json` (Serbian Latin) — i18n parity is enforced by `i18n.test.ts`.
- Keep `TrumpIndicator`'s existing visibility rule (hidden during `dealing` and `bidding` phases) unchanged. Dealer indicator should be visible from the moment a hand starts (i.e., as soon as `gameState.players` and `dealerSeat` are populated), including during dealing and bidding.

**Ask First:**

- None — scope is locked to the two indicators and their i18n. No backend changes.

**Never:**

- Do not modify `event:trump_selected` payload, `wsEvents.ts` `TrumpSelectedPayload`, `events.go`, or any Go code. The full `GameState` already carries everything needed.
- Do not change turn order, dealer rotation, or any rules-engine logic.
- Do not introduce a new Zustand slice or store accessor — read directly from the existing `gameState`.
- Do not add a transient/banner notification (per user choice: option 2 — persistent label, not a toast).

## I/O & Edge-Case Matrix

| Scenario                                              | Input / State                                      | Expected Output / Behavior                                        | Error Handling                       |
| ----------------------------------------------------- | -------------------------------------------------- | ----------------------------------------------------------------- | ------------------------------------ |
| Trump picked by team-A player                         | `trumpCallerSeat=0`, `players[0].username="Marko"` | TrumpIndicator shows suit, team-A chip, AND "Marko"               | N/A                                  |
| Trump not yet picked                                  | `trumpCallerSeat=null`                             | TrumpIndicator hides team chip and player name (current behavior) | N/A                                  |
| Trump picked but caller player missing from `players` | `trumpCallerSeat=2`, no `players[2]`               | Render team chip, omit player name; no crash                      | Silent fallback                      |
| Dealer present, hand active                           | `dealerSeat=1`, `players[1].username="Ana"`        | DealerIndicator shows label "Dealer: Ana" (i18n-driven)           | N/A                                  |
| Dealer seat without matching player                   | `dealerSeat=5`, no `players[5]`                    | DealerIndicator does not render                                   | Silent hide                          |
| Bitola dealer rotation (no one picked round 2)        | New hand, new `dealerSeat`                         | Indicator updates to new dealer's name automatically              | N/A (driven by reactive `gameState`) |
| Locale = Serbian Latin                                | i18n active language `sr`                          | Both indicators render Serbian translations                       | N/A                                  |

</frozen-after-approval>

## Code Map

- `client/src/features/game/components/TrumpIndicator.tsx` -- existing trump pill; extend props with `trumpCallerName?: string | null` and render it after team chip.
- `client/src/features/game/components/TrumpIndicator.test.tsx` -- co-located tests; add cases for player-name rendering and omission.
- `client/src/features/game/components/DealerIndicator.tsx` -- NEW; small pill mirroring `TrumpIndicator` styling, props `{ dealerName: string }`.
- `client/src/features/game/components/DealerIndicator.test.tsx` -- NEW; verify rendering, `data-testid`, `aria-live`, hide-when-missing.
- `client/src/features/game/GamePage.tsx` -- lookup `players.find(p => p.seat === dealerSeat/trumpCallerSeat)` once near the existing trump block; render `DealerIndicator` alongside `TrumpIndicator` in the top-right corner stack.
- `client/src/shared/i18n/en.json` -- add `game.dealerIndicator.label` ("Dealer: {{name}}") and `game.trumpIndicator.labelWithCaller` ("Trump suit: {{suit}}, called by {{team}} team ({{name}})"); also add a `caller` test-id-friendly key.
- `client/src/shared/i18n/sr.json` -- Serbian Latin counterparts.
- `client/src/shared/i18n/i18n.test.ts` -- existing parity test; should pass once both locale files are updated symmetrically.
- `client/src/shared/types/gameTypes.ts` -- read-only reference; `dealerSeat: number`, `trumpCallerSeat: number | null`, `players: PlayerState[]` with `.username`.

## Tasks & Acceptance

**Execution:**

- [x] `client/src/features/game/components/TrumpIndicator.tsx` -- add optional `trumpCallerName?: string | null` prop; when both team and name are present, render an additional inline span (`data-testid="trump-caller-name"`) showing the username; update `aria-label` to use a new i18n key `labelWithCaller` when name is provided, falling back to existing `labelWithTeam` / `label`.
- [x] `client/src/features/game/components/TrumpIndicator.test.tsx` -- add: (1) renders player name when `trumpCallerName` provided alongside seat, (2) omits name span when only seat provided without name, (3) `aria-label` includes player name when both team and name are set.
- [x] `client/src/features/game/components/DealerIndicator.tsx` -- NEW component matching `TrumpIndicator`'s pill style (neutral border, no team color). Props: `dealerName: string`. Renders label-icon + name. `data-testid="dealer-indicator"`, `aria-live="polite"`, `aria-label` from `game.dealerIndicator.label`.
- [x] `client/src/features/game/components/DealerIndicator.test.tsx` -- NEW; verify renders name, has `data-testid` and `aria-live`, label uses i18n key.
- [x] `client/src/features/game/GamePage.tsx` -- in the existing `top-4 right-16` block, derive `dealerName` and `trumpCallerName` via `players.find(p => p.seat === ...)?.username`; render `<DealerIndicator dealerName={...} />` (when name resolves) above or beside the existing `<TrumpIndicator ... trumpCallerName={trumpCallerName} />`; stack vertically with a small gap (`flex flex-col items-end gap-2`).
- [x] `client/src/shared/i18n/en.json` -- add `game.dealerIndicator.label` = "Dealer: {{name}}", `game.dealerIndicator.dealer` = "Dealer", and `game.trumpIndicator.labelWithCaller` = "Trump suit: {{suit}}, called by {{team}} team ({{name}})".
- [x] `client/src/shared/i18n/sr.json` -- mirror keys: `game.dealerIndicator.label` = "Delilac: {{name}}", `game.dealerIndicator.dealer` = "Delilac", `game.trumpIndicator.labelWithCaller` = "Adut: {{suit}}, zvao {{team}} tim ({{name}})".

**Acceptance Criteria:**

- Given a game in `playing` phase with trump picked by seat 0 named "Marko" (Team A), when GamePage renders, then both the team chip AND the text "Marko" appear inside `[data-testid="trump-indicator"]`.
- Given a game with `dealerSeat=2` and `players[2].username="Ana"`, when GamePage renders any phase that has populated players, then `[data-testid="dealer-indicator"]` is present and contains "Ana".
- Given the user switches the i18n language to Serbian, when GamePage re-renders, then both indicators show Serbian text ("Delilac:", "Adut:", suit names, team names).
- Given `gameState.trumpCallerSeat` is `null`, when GamePage renders, then `[data-testid="trump-caller-name"]` is not present and `[data-testid="trump-caller-team"]` is also absent (existing behavior preserved).
- Given the dealer rotates between hands (e.g., Bitola round 2 reshuffle path), when the new hand starts with a different `dealerSeat`, then `DealerIndicator` shows the new dealer's username without manual refresh.

## Design Notes

The indicators stack vertically in the top-right corner. The existing wrapper `<div className="absolute top-4 right-16 z-10">` becomes a flex column:

```tsx
<div className="absolute top-4 right-16 z-10 flex flex-col items-end gap-2">
  {dealerName && <DealerIndicator dealerName={dealerName} />}
  {gameState.trumpSuit &&
    gameState.phase !== "dealing" &&
    gameState.phase !== "bidding" && (
      <TrumpIndicator
        trumpSuit={gameState.trumpSuit}
        trumpCallerSeat={gameState.trumpCallerSeat}
        trumpCallerName={trumpCallerName}
      />
    )}
</div>
```

`DealerIndicator` styling uses a neutral pill (e.g., `border-text-secondary/30 bg-background/80`) — not a team color — because the dealer role is mechanical, not partisan.

## Verification

**Commands:**

- `cd client && npx prettier --write .` -- expected: formats touched files (per memory: prettier before every commit).
- `cd client && npx vitest run` -- expected: all tests pass, including new `DealerIndicator.test.tsx` and updated `TrumpIndicator.test.tsx`; existing `i18n.test.ts` parity check passes with both locales updated.
- `make lint` -- expected: ESLint + Prettier + golangci-lint all green.
- `make test` -- expected: full Go + frontend suite green.

**Manual checks:**

- Start `make dev`, open two clients, start a Bitola hand: confirm Dealer pill appears for both clients showing the dealer's username from hand start; after trump pick, confirm trump pill shows team color AND the picker's name.
- Toggle locale to Serbian via i18n switcher (or via dev tool): confirm "Delilac: …" and "Adut: …, zvao … tim (…)" render.

## Suggested Review Order

**Wiring and data flow**

- Stack the two indicators top-right; resolve dealer + trump-caller usernames by seat lookup against the existing `gameState.players`.
  [`GamePage.tsx:376`](../../client/src/features/game/GamePage.tsx#L376)

**Trump-caller name extension**

- New optional prop joins the existing `trumpCallerSeat` contract — name is purely additive.
  [`TrumpIndicator.tsx:5`](../../client/src/features/game/components/TrumpIndicator.tsx#L5)

- Trim-and-fallback: a single line guarantees blank/whitespace inputs resolve to `null`, mirroring `DealerIndicator`.
  [`TrumpIndicator.tsx:68`](../../client/src/features/game/components/TrumpIndicator.tsx#L68)

- Aria-label cascade: `labelWithCaller` → `labelWithTeam` → `label` keeps screen-reader output consistent with what's visually rendered.
  [`TrumpIndicator.tsx:71`](../../client/src/features/game/components/TrumpIndicator.tsx#L71)

- Caller-name span gated on `team && callerName` so it never renders without the team chip beside it.
  [`TrumpIndicator.tsx:114`](../../client/src/features/game/components/TrumpIndicator.tsx#L114)

**Dealer indicator (new)**

- Component signature mirrors `TrumpIndicator`'s pill style; trims internally and returns `null` for blank input.
  [`DealerIndicator.tsx:7`](../../client/src/features/game/components/DealerIndicator.tsx#L7)

- Neutral border (`border-text-secondary/30`) — dealer is mechanical, not partisan; intentionally avoids team colors.
  [`DealerIndicator.tsx:18`](../../client/src/features/game/components/DealerIndicator.tsx#L18)

**i18n**

- Three new English keys with placeholder interpolation; preserved alongside existing trump keys.
  [`en.json:216`](../../../client/src/shared/i18n/en.json#L216)

- Serbian Latin counterparts for parity (enforced by `i18n.test.ts`).
  [`sr.json:216`](../../../client/src/shared/i18n/sr.json#L216)

**Tests**

- New component test covers render, accessibility attributes, and blank-name short-circuit.
  [`DealerIndicator.test.tsx:6`](../../client/src/features/game/components/DealerIndicator.test.tsx#L6)

- Three new cases for the caller-name path; preserves all original assertions.
  [`TrumpIndicator.test.tsx:68`](../../client/src/features/game/components/TrumpIndicator.test.tsx#L68)
