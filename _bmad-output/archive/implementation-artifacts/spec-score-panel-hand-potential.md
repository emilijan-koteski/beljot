---
title: "Live in-hand (potential) scoring in ScorePanel"
type: "feature"
created: "2026-04-17"
status: "done"
context: []
baseline_commit: "efd7ef5"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The always-visible ScorePanel (top-left) only renders `teamScores`, which update once per hand (after all 8 tricks). Players win a trick, announce belot, or watch declarations resolve — and nothing in the HUD reflects it until hand-end. The user experiences this as "points not added after a trick is taken" and wants the HUD to distinguish _potential_ (in-progress) vs _confirmed_ (match-total) points.

**Approach:** Treat the ScorePanel as two tiers per team. Keep the large `teamScores` number as the primary (confirmed) score. Add a smaller secondary line underneath showing the current hand's combined potential total = `handPoints[team] + declarationPoints[team]`. Hide the secondary line when that combined total is 0 (start of a fresh hand, before any trick resolves). When the hand ends and handPoints/declarationPoints reset server-side, the secondary line disappears and the primary score jumps.

## Boundaries & Constraints

**Always:**

- Match-level `teamScores` remains the dominant visual — larger font, full contrast, same position as today.
- Potential total is the single combined sum of `handPoints[team] + declarationPoints[team]`. No separate breakdown into card/declaration sub-rows in the HUD.
- Secondary line is visually subordinate (smaller font, muted color) and uses a unambiguously non-final framing (i18n key `game.score.thisHand`, shown as `+{total} this hand`).
- `aria-live="polite"` on the panel preserved; the secondary line is part of the same live region.
- Existing bonus float-up animation on last-trick/capot keeps working unchanged.
- `prefers-reduced-motion` users get no new motion.

**Ask First:**

- Any change that alters the match-score number source (still `teamScores`), trick-count display, or the trick-bonus float-up animation.
- Any redesign of the confirmed-score typography, color, or layout position.

**Never:**

- Don't summate potential into the match score (don't display `teamScores + handPoints` as one number). The two must read as distinct tiers.
- Don't break out card points vs declaration points in the HUD (user asked for "potential vs real", not full breakdown — the ScoreReveal modal at hand-end already shows breakdown).
- Don't touch the server scoring pipeline. All signals already exist on `GameState`.
- Don't change `DeclarationReveal` or `ScoreReveal` components.

## I/O & Edge-Case Matrix

| Scenario                                       | Input / State                                                          | Expected Output / Behavior                                                                   |
| ---------------------------------------------- | ---------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Fresh hand, no tricks resolved                 | `handPoints=[0,0]`, `declarationPoints=[0,0]`                          | Secondary "this hand" line is hidden for both teams; only match score + trick count visible. |
| First trick won by team A                      | `handPoints=[12,0]`, `declarationPoints=[0,0]`                         | Team A shows `+12 this hand` under match score; team B secondary line hidden.                |
| Trick 1 resolves, declarations award team B 50 | `handPoints=[12,14]`, `declarationPoints=[0,50]`                       | Team A shows `+12 this hand`; team B shows `+64 this hand`.                                  |
| Player announces belot during trick            | `handPoints[team] += 20` (server-side)                                 | Matching team's "this hand" number bumps by 20 on next state push.                           |
| Hand ends, server resets hand/decl points      | `handPoints=[0,0]`, `declarationPoints=[0,0]`, `teamScores` increments | Secondary lines hide; primary numbers bump (existing CSS transition).                        |
| Match ends (phase `match_end`)                 | ScorePanel still mounted but no new tricks                             | Behavior identical to fresh-hand row: potentials 0, secondary hidden.                        |

</frozen-after-approval>

## Code Map

- `client/src/features/game/components/ScorePanel.tsx` -- add two new props and render secondary "this hand" row per team
- `client/src/features/game/components/ScorePanel.test.tsx` -- cover new rendering rules (hidden when 0, visible otherwise, correct sum)
- `client/src/features/game/GamePage.tsx` -- wire `gameState.handPoints` and `gameState.declarationPoints` into the new props
- `client/src/shared/i18n/en.json` -- add `game.score.thisHand` key: `"this hand"`
- `client/src/shared/i18n/sr.json` -- add `game.score.thisHand` key: `"ova deljenja"` (or best Serbian equivalent)
- `client/src/shared/types/gameTypes.ts` -- read-only reference for existing `handPoints` / `declarationPoints` / `teamScores` fields
- `server/internal/game/playing.go` -- read-only reference confirming handPoints updates per-trick
- `server/internal/game/declarations.go` -- read-only reference confirming declarationPoints awarded after trick 1 and belot adds immediately

## Tasks & Acceptance

**Execution:**

- [x] `client/src/features/game/components/ScorePanel.tsx` -- extend prop type with `teamAHandPotential: number` and `teamBHandPotential: number`; render a small secondary line under each team's match score showing `+N {t('game.score.thisHand')}` when `>0`, hidden otherwise
- [x] `client/src/features/game/GamePage.tsx` -- pass `teamAHandPotential={gameState.handPoints[0] + gameState.declarationPoints[0]}` and `teamBHandPotential={gameState.handPoints[1] + gameState.declarationPoints[1]}`
- [x] `client/src/shared/i18n/en.json` -- add `game.score.thisHand: "this hand"`
- [x] `client/src/shared/i18n/sr.json` -- add `game.score.thisHand: "ovo deljenje"` (Serbian, reuse existing locale style)
- [x] `client/src/features/game/components/ScorePanel.test.tsx` -- add three tests: secondary line hidden at 0, visible with correct sum at non-zero, independent per team
- [x] Confirm no regression in the bonus float-up test and existing score label/tricks tests

**Acceptance Criteria:**

- Given it is mid-hand and Team A has won one trick worth 12 points, when the game state updates, then the ScorePanel shows Team A match score unchanged and a smaller `+12 this hand` underneath the Team A row.
- Given belot is announced by a Team B player, when the game state updates, then Team B's "this hand" number increases by 20 without any change to match score.
- Given the hand ends (all 8 tricks resolved and server has reset `handPoints`/`declarationPoints`), when state arrives, then the secondary lines disappear and the match score bumps with the existing CSS transition.
- Given `handPoints` and `declarationPoints` are both zero for both teams (start of hand), when the panel renders, then neither secondary line is shown and the panel visual height is roughly today's height.
- Given accessibility, the secondary line is inside the existing `aria-live="polite"` region so screen readers announce changes.

## Design Notes

Visual hierarchy target:

```
┌──────────────────┐
│ Team A     120   │  <- unchanged: team-a color, 3xl bold, tabular-nums
│          +12 this hand  <- new: text-text-secondary, text-xs, tabular-nums
│                  │
│ Team B      95   │
│          +64 this hand
├──────────────────┤
│ Tricks: 2 - 1    │
└──────────────────┘
```

Hide the secondary line via conditional JSX (not visibility: hidden) so the panel shrinks at 0 and doesn't carve out vertical space when potentials are zero. Use the existing `motion-safe:transition-all` on the primary number so when the hand finishes the primary bumps smoothly while the secondary disappears.

Keep the secondary line's color muted (`text-text-secondary`) rather than team-colored: the team color carries match-score weight; the muted gray signals "not yet final".

## Verification

**Commands:**

- `cd client && npx vitest run src/features/game/components/ScorePanel.test.tsx src/features/game/GamePage.test.tsx` -- expected: all tests pass, including new ScorePanel rendering rules
- `cd client && npx tsc --noEmit` -- expected: no errors (new props are typed)
- `cd client && npx vitest run` -- expected: all 304 existing tests still pass

**Manual checks:**

- Start a game, watch a trick resolve. Team A or Team B should gain a visible `+N this hand` line immediately without waiting for hand-end.
- Announce belot; matching team's `this hand` increments by 20 on the next state tick.
- Let a full hand complete; secondary lines should disappear and the primary scores should bump via the existing transition.

## Suggested Review Order

**Rendering — the secondary tier**

- Entry point: new props + per-team block structure, secondary line gated on `> 0`.
  [`ScorePanel.tsx:9`](../../client/src/features/game/components/ScorePanel.tsx#L9)

- Secondary render block for Team A; muted text, tabular-nums, right-aligned for number stability.
  [`ScorePanel.tsx:67`](../../client/src/features/game/components/ScorePanel.tsx#L67)

- Same for Team B — independent condition, so one team can show while the other hides.
  [`ScorePanel.tsx:93`](../../client/src/features/game/components/ScorePanel.tsx#L93)

**Data wiring**

- Sum `handPoints + declarationPoints` per team and pass down; no new state.
  [`GamePage.tsx:344`](../../client/src/features/game/GamePage.tsx#L344)

**Stale-state cleanup (review-surfaced patch)**

- Hand-scored handler now zeros per-hand arrays so the secondary line disappears as soon as the match score bumps, without waiting for the follow-up `event:game_state`.
  [`useWsDispatch.ts:146`](../../../client/src/shared/hooks/useWsDispatch.ts#L146)

**i18n**

- English label.
  [`en.json:206`](../../../client/src/shared/i18n/en.json#L206)

- Serbian label ("ovo deljenje").
  [`sr.json:206`](../../../client/src/shared/i18n/sr.json#L206)

**Tests**

- Four ScorePanel tests covering hide-at-zero, Team-A-only, both-teams independent, default-props.
  [`ScorePanel.test.tsx:81`](../../client/src/features/game/components/ScorePanel.test.tsx#L81)

- Dispatch test proving hand_scored clears hand/declaration potentials.
  [`useWsDispatch.test.ts:331`](../../../client/src/shared/hooks/useWsDispatch.test.ts#L331)
