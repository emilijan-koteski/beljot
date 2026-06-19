---
title: 'Match-stake (pot) HUD + top-corner stacking fix'
type: 'feature'
created: '2026-06-19'
status: 'done'
context: ['{project-root}/_bmad-output/project-context.md']
baseline_commit: '7ae72d5066f71dd167d82c4bc49f0966db45e906'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The match table never shows the total stake (the pot = what the winning team takes), so players can't see what they're competing for. Separately, on phones the top-left score panel is painted *over* by the partner's (north) avatar — it renders below the seats in the stacking order.

**Approach:** Add a compact coin "stake" pill to the table HUD — mobile: top-right, left of the hamburger; desktop: top-right, in a column with the trump indicator — when no trump has been taken yet (dealing/bidding) the stake sits at the top, and once the trump indicator appears it drops directly beneath it (a top-anchored flex column that grows downward). Derive the pot on the client as `(# human players) × room buy-in`, matching the server's settlement formula exactly (no backend/wire change). Introduce one new z-tier above the seats and apply it to the score panel and the new stake pill so both sit above the partner avatar.

## Boundaries & Constraints

**Always:**
- Pot = `matchState.players.filter(p => !p.isBot).length × room.coinBuyIn` — mirror server `computeSettlement` (`pot = numHumans × buyIn`); bots never contribute. Hide the pill when pot is `0` (explicit `> 0`, never truthiness).
- New z-tier sits **above** `Z.SEATS` (20) and **below** `Z.CARD_FLIGHT` (30) — apply via `style={{ zIndex: Z.X }}` (the `zLayers.ts` pattern), never a Tailwind `z-*` class.
- Reuse existing HUD chrome (panel bg + brass border + blur), the `Coins` icon, `COIN_GOLD`, `formatCoins()`. No new colors/tokens. i18n added to all four locales (en/hr/sr/mk); mk all-Cyrillic.

**Ask First:**
- Any need to add a field to the server `GameState`/WS contract (the client-derived approach is the chosen path — only revisit if it proves insufficient).

**Never:**
- No backend, `events.go`, or `wsEvents.ts` changes — the pot is derived client-side.
- Don't change seat (`Z.SEATS`) z-index or any dialog-band tier.
- Don't alter the desktop trump-indicator's visibility gating or the hamburger behavior.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Paid match, 4 humans | coinBuyIn 500, 4 non-bot players | Pill shows `2,000` (`Coins` icon + formatted) | N/A |
| Paid match w/ bots | coinBuyIn 500, 2 humans + 2 bots | Pill shows `1,000` (bots excluded) | N/A |
| Free room | coinBuyIn 0 | No stake pill rendered (both breakpoints) | N/A |
| Buy-in not yet fetched | roomBuyIn null on first paint | No pill until getRoom resolves, then it appears | N/A |
| Mobile overlap | narrow viewport, north seat at top-center | Score panel + stake render above the partner avatar | N/A |
| Desktop, no trump yet | phase dealing/bidding | Stake sits at the top-right (trump indicator absent) | N/A |
| Desktop, trump taken | phase playing | Trump indicator on top, stake directly beneath it | N/A |

</frozen-after-approval>

## Code Map

- `client/src/shared/lib/zLayers.ts` -- `Z` tiers; add `HUD` between `SEATS` (20) and `CARD_FLIGHT` (30).
- `client/src/features/match/components/StakePill.tsx` -- NEW presentational pill (coin icon + `formatCoins(amount)`).
- `client/src/features/match/MatchPage.tsx` -- lift room buy-in to state; derive pot; render the pill (mobile near hamburger ~L1636, desktop under trump indicator ~L1438); apply `Z.HUD`.
- `client/src/features/match/components/ScorePanel.tsx` -- mobile panel (~L225) + desktop panel (~L274) currently `z-10`; switch to `Z.HUD`.
- `client/src/shared/i18n/{en,hr,sr,mk}.json` -- add `match.stake.label`.
- `*.test.tsx` co-located -- see Execution.

## Tasks & Acceptance

**Execution:**
- [x] `client/src/shared/lib/zLayers.ts` -- add `HUD: 25` with a doc comment (corner HUD: scoreboard, trump indicator, stake — above avatars, below flights/dialogs).
- [x] `client/src/features/match/components/StakePill.tsx` -- new component `StakePill({ amount }: { amount: number })`: inline-flex pill, `Coins` icon (`COIN_GOLD`) + `formatCoins(amount)`, HUD chrome style, `aria-label`=`` `${t("match.stake.label")}: ${formatCoins(amount)}` ``, `data-testid="match-stake"`. Positionless (caller positions it).
- [x] `client/src/shared/i18n/en.json` (+ hr/sr/mk) -- add `match.stake.label`: en `Stake` / hr `Ulog` / sr `Ulog` / mk `Влог`.
- [x] `client/src/features/match/MatchPage.tsx` -- (a) add `roomBuyIn` state, set it in the existing `getRoom().then` beside `roomBuyInRef`; (b) after the matchState guard, derive `matchStake = roomBuyIn ? humanCount × roomBuyIn : 0`; (c) wrap the desktop trump-indicator block in a `absolute top-4 right-4 hidden flex-col items-end gap-2 md:flex` container at `Z.HUD`, render `<StakePill>` beneath the (still-gated) `TrumpIndicator` when `matchStake > 0`; (d) add a `md:hidden` block rendering `<StakePill>` at `absolute top-3 right-14` (left of the hamburger) at `Z.HUD` when `matchStake > 0`.
- [x] `client/src/features/match/components/ScorePanel.tsx` -- replace `z-10` with `style={{ zIndex: Z.HUD }}` on both the mobile (`score-panel-mobile`) and desktop (`score-panel`) containers.
- [x] `client/src/features/match/components/StakePill.test.tsx` -- new: renders formatted amount, coin icon, aria-label, testid.
- [x] `client/src/features/match/components/ScorePanel.test.tsx` -- assert `score-panel-mobile` `style.zIndex === String(Z.HUD)` and `Z.HUD > Z.SEATS`.
- [x] `client/src/features/match/MatchPage.test.tsx` -- assert stake pill shows `2,000` for coinBuyIn 500 × 4 humans (model on the existing coinBuyIn:500 test ~L478); assert no pill for coinBuyIn 0 (model on ~L414).

**Acceptance Criteria:**
- Given a paid match, when the table renders, then a coin pill shows `humans × buyIn` top-right (mobile: beside the hamburger; desktop: under the trump indicator).
- Given two bots fill seats, when the pill renders, then bots are excluded from the pot.
- Given a free room (buy-in 0), when the table renders, then no stake pill appears.
- Given a phone viewport, when the north partner avatar would overlap the top-left score panel, then the score panel and the stake pill paint above the avatar.

## Verification

**Commands:**
- `cd client && npx vitest run src/features/match/components/StakePill.test.tsx src/features/match/components/ScorePanel.test.tsx src/features/match/MatchPage.test.tsx` -- expected: all pass.
- `make lint` -- expected: ESLint + Prettier clean.

**Manual checks:**
- Phone (<768px): stake pill left of the hamburger; score panel + stake render above the partner avatar (no clipping).
- Desktop: stake alone at top-right during dealing/bidding; once trump is taken the indicator appears above it and the stake drops beneath — same anchor, no jump.

## Suggested Review Order

**Stake derivation (the design core)**

- Entry point — the whole feature in one line: pot = non-bot players × buy-in, hidden until loaded.
  [`MatchPage.tsx:1295`](../../client/src/features/match/MatchPage.tsx#L1295)

- Buy-in lifted from the mount-time getRoom fetch into state (the ref stays for insolvency).
  [`MatchPage.tsx:784`](../../client/src/features/match/MatchPage.tsx#L784)

- The state declaration paired with the existing synchronous ref.
  [`MatchPage.tsx:290`](../../client/src/features/match/MatchPage.tsx#L290)

**Stake HUD rendering**

- Presentational pill — reuses HUD chrome, coin glyph + formatCoins, aria-label only.
  [`StakePill.tsx:25`](../../client/src/features/match/components/StakePill.tsx#L25)

- Desktop top-right column: trump indicator (gated) with the stake beneath; stake-only sits at top.
  [`MatchPage.tsx:1448`](../../client/src/features/match/MatchPage.tsx#L1448)

- Mobile pill anchored left of the hamburger, its own block so it persists like the score panel.
  [`MatchPage.tsx:1652`](../../client/src/features/match/MatchPage.tsx#L1652)

**Z-index stacking fix**

- New HUD tier between SEATS (20) and CARD_FLIGHT (30) — the root-cause fix.
  [`zLayers.ts:38`](../../client/src/shared/lib/zLayers.ts#L38)

- Score panel mobile + desktop migrated off Tailwind `z-10` to inline `Z.HUD`.
  [`ScorePanel.tsx:228`](../../client/src/features/match/components/ScorePanel.tsx#L228)

**Localization & tests**

- New `match.stake.label` key (en/hr/sr/mk; mk all-Cyrillic).
  [`en.json:742`](../../client/src/shared/i18n/en.json#L742)

- Pot value, bot exclusion, and free-room hiding.
  [`MatchPage.test.tsx:199`](../../client/src/features/match/MatchPage.test.tsx#L199)

- Mobile score panel outranks the seat avatars.
  [`ScorePanel.test.tsx:64`](../../client/src/features/match/components/ScorePanel.test.tsx#L64)

- StakePill rendering, aria-label, coin glyph.
  [`StakePill.test.tsx:13`](../../client/src/features/match/components/StakePill.test.tsx#L13)
