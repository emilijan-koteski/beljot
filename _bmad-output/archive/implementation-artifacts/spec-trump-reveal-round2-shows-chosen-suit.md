---
title: "Trump-reveal dialog shows chosen suit (round 2 free pick)"
type: "bugfix"
created: "2026-05-09"
status: "done"
baseline_commit: "d6fe68f3fa362b90bfa4bd98653139b7f2f5908a"
context:
  - "{project-root}/_bmad-output/project-context.md"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When a player picks trump in **round 2** of Bitola bidding (free-suit choice), the `TrumpReveal` dialog labels the wrong suit and shows the wrong visual. The component derives the displayed suit name from `cardId` (the originally face-up candidate), and always renders the candidate `PlayingCard`. So when the candidate is `9♠` but the picker chose Diamonds, the dialog reads `… took trump · Spades` and shows the 9 of spades, contradicting the persistent `TrumpIndicator` (which correctly reads "Diamonds"). The payload already carries `trumpSuit` separately from `cardId` — the bug is purely in `TrumpReveal.tsx` reading the wrong field.

**Approach:** In `TrumpReveal`, use `trumpSuit` (passed through from the existing `TrumpSelectedPayload`) as the source of truth for the captioned suit name. Detect "free pick" rounds by comparing `parseCardId(cardId).suit !== trumpSuit`. Branch the visual: round 1 (suits match) keeps the current `PlayingCard` of the candidate; round 2 (suits differ) replaces the playing-card visual with a chosen-suit chip (large parchment suit-orb in the suit's color) and adds a small "from candidate **Ten of Spades**" caption — full English rank + suit names, never glyphs or single-letter codes — so other seats still see what was on the table without misreading a `T` as a "T". No backend or wire-protocol changes — `trumpSuit` and `cardId` are already on `TrumpSelectedPayload`.

## Boundaries & Constraints

**Always:**

- The captioned suit name (in the title line) is derived from `trumpSuit`, not from `cardId`, in both rounds.
- Round 1 vs round 2 is detected at render time by `parseCardId(cardId).suit !== trumpSuit`. Do not introduce a new `round` field on the payload.
- The chosen-suit chip in round 2 mirrors the suit-orb pattern already established in `TrumpIndicator` (parchment circle + radial halo, red for H/D, near-black for S/C). Reuse the same color tokens (`--suit-red`, `--suit-black`, `--brass`) for visual consistency. Do not extract a shared component as part of this fix — duplication is acceptable for one screen.
- The original face-up candidate must still be visible in round 2 so other seats see what was on the table — render it as a small **localized full-name** caption beneath/beside the chip ("candidate: Ten of Spades", "candidate: Nine of Hearts"). Never use single-letter rank codes (`T`, `J`, `Q`, `K`, `A`) or suit glyphs (`♠♥♦♣`) in this text — `T` is ambiguous (could read as "T" the letter), and glyphs do not localize. The numeric ranks 7/8/9 also use their localized word forms ("Seven", "Eight", "Nine") for consistency. Do not render the full `PlayingCard` here.
- Round 1 visual (full `PlayingCard` of the candidate) and round 1 copy (eyebrow `Trump taken`, title `… took trump · {{Suit}}`) stay exactly as today, since in round 1 the candidate **is** the chosen trump.
- Add new i18n keys to **all four** translation files (`en.json`, `sr.json`, `mk.json`, `hr.json`) — `i18n.test.ts` enforces parity and will fail otherwise.
- Auto-dismiss timing (8 s normal, 1.5 s reduced-motion), the X-with-countdown-ring control, and the viewer-relative team glow stay unchanged.

**Ask First:**

- None — this is a purely client-side correctness fix, additive props/i18n only.

**Never:**

- Do not change `TrumpSelectedPayload`, `events.go`, or `manager.go`. The wire payload is already correct.
- Do not modify `TrumpIndicator`. The persistent top-right indicator is already correct in both rounds.
- Do not extend `GameState` with a "round taken in" flag or persist any round-2 metadata — derive it from the two fields already on the payload.
- Do not remove the candidate-card information entirely in round 2. Other players need to see what was on the table; collapse it to a glyph, do not delete it.
- Do not change the auto-dismiss durations or the `key={playerSeat}-{cardId}` remount strategy.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|----------------------------|----------------|
| Round 1 pick (candidate suit = chosen suit) | `cardId="7S"`, `trumpSuit="S"` | Eyebrow `Trump taken`; title contains picker name and `Spades`; full `PlayingCard` for `7S` rendered (`data-testid="playing-card-7S"` present) | N/A |
| Round 2 free pick (suits differ) | `cardId="9S"`, `trumpSuit="D"` | Eyebrow `Trump taken · free pick`; title contains picker name and `Diamonds` (not Spades); a suit-chip with the diamond glyph is rendered (`data-testid="trump-reveal-suit-chip"` present, `data-suit="D"`); no `data-testid="playing-card-9S"`; small candidate caption reads `candidate: Nine of Spades` (full words, no `9♠`) | N/A |
| Round 2, hearts chosen, ten-of-clubs candidate | `cardId="TC"`, `trumpSuit="H"` | Suit chip uses suit-red color tokens; title says `Hearts`; candidate caption reads `candidate: Ten of Clubs` (never `T♣` — `T` would be misread as the letter "T") | N/A |
| Malformed payload (cardId length < 2) | `cardId=""`, `trumpSuit="D"` | Component returns `null` — existing guard preserved | Silent (drop) |
| Locale `sr` round-2 | `cardId="9S"`, `trumpSuit="D"`, locale Serbian Latin | Eyebrow uses Serbian "free pick" string; suit name "Karo"; candidate caption uses Serbian rank + suit words (e.g. "Devetka pik"), never glyphs or `T/J/Q/K/A` codes | N/A |
| Unknown picker seat | `playerSeat=5`, no matching player | Falls back to `unknownPlayer` title; round-1 vs round-2 visual still applied per suit comparison | N/A |

</frozen-after-approval>

## Code Map

- `client/src/features/game/components/TrumpReveal.tsx` -- accept new `trumpSuit: Suit` prop; derive `suitName` from `trumpSuit`; compute `isFreePick = parseCardId(cardId).suit !== trumpSuit`; branch visual between full `PlayingCard` (round 1) and a suit-chip + full-word candidate caption like "candidate: Ten of Spades" (round 2); use a different eyebrow key when `isFreePick`. Build the caption as `t('game.trumpReveal.candidateLabel', { rank: t('game.ranks.<word>'), suit: t('game.suits.<word>') })` — the existing `game.suits.*` keys cover suits, new `game.ranks.*` keys cover all 8 ranks. Never substitute single-letter codes or glyphs.
- `client/src/features/game/GamePage.tsx` -- pass `trumpSuit={trumpReveal.trumpSuit as Suit}` into `<TrumpReveal>` (the value is already in store; just thread it through).
- `client/src/features/game/components/TrumpReveal.test.tsx` -- update existing renders to pass `trumpSuit`; add a round-2 test asserting the suit chip is rendered, the suit name reads from `trumpSuit`, and the `PlayingCard` is **not** rendered.
- `client/src/shared/i18n/en.json` -- add (1) `game.trumpReveal.eyebrowFreeChoice` ("Trump taken · free pick"), (2) `game.trumpReveal.candidateLabel` ("candidate: {{rank}} of {{suit}}"), and (3) the full `game.ranks` block — `seven`, `eight`, `nine`, `ten`, `jack`, `queen`, `king`, `ace`. The locale's word forms must be capitalized to match the title-case suit names already in `game.suits.*`.
- `client/src/shared/i18n/sr.json` -- mirror keys; rank words in Serbian Latin (e.g. "Sedmica", "Osmica", "Devetka", "Desetka", "Žandar", "Dama", "Kralj", "As") and a `candidateLabel` template that reads naturally in Serbian (the suit-name grammatical form may need to follow the existing `game.suits.*` form).
- `client/src/shared/i18n/mk.json` -- mirror keys in Macedonian Cyrillic.
- `client/src/shared/i18n/hr.json` -- mirror keys in Croatian.

## Tasks & Acceptance

**Execution:**

- [x] `client/src/features/game/components/TrumpReveal.tsx` -- add `trumpSuit: Suit` to `TrumpRevealProps`; replace `card.suit` with `trumpSuit` in `suitName` lookup; compute `isFreePick`; when `isFreePick`, render a suit-orb (mirroring `TrumpIndicator`'s parchment+halo for `trumpSuit`) instead of `<PlayingCard>` and a candidate caption built as `t('game.trumpReveal.candidateLabel', { rank: t(rankNameKey(card.rank)), suit: t(suitNameKey(card.suit)) })` — full localized words, never `T♠` or `9C` style codes. When not free-pick, render `<PlayingCard>` exactly as today. Use `game.trumpReveal.eyebrowFreeChoice` for the eyebrow when `isFreePick`. Tag the chip with `data-testid="trump-reveal-suit-chip"` + `data-suit={trumpSuit}`. Add a local `rankNameKey(rank: Rank)` helper that returns the matching `game.ranks.<word>` key (mirrors the existing `suitNameKey`).
- [x] `client/src/features/game/GamePage.tsx` -- thread `trumpSuit={trumpReveal.trumpSuit as Suit}` (or with a runtime guard if the type is `string`) into `<TrumpReveal>` alongside the existing props.
- [x] `client/src/features/game/components/TrumpReveal.test.tsx` -- add `trumpSuit` to every `<TrumpReveal>` render (`trumpSuit="S"` for the existing `cardId="7S"` cases, `trumpSuit="D"` for the existing `cardId="9D"` case); add a new test case with `cardId="9S"` + `trumpSuit="D"` asserting (a) `getByTestId("trump-reveal-suit-chip")` with `data-suit="D"`, (b) the title text contains "Diamonds" not "Spades", (c) `queryByTestId("playing-card-9S")` is null, (d) the candidate caption text contains the full words "Nine of Spades" and does **not** contain `9S`, `T`, `J`, `Q`, `K`, `A` as standalone codes nor any of the glyph characters `♠♥♦♣`.
- [x] `client/src/shared/i18n/en.json` -- add `game.trumpReveal.eyebrowFreeChoice`, `game.trumpReveal.candidateLabel` ("candidate: {{rank}} of {{suit}}"), and a new `game.ranks` block with all eight ranks spelled out (Seven, Eight, Nine, Ten, Jack, Queen, King, Ace).
- [x] `client/src/shared/i18n/sr.json`, `mk.json`, `hr.json` -- mirror the same key shape with locale-appropriate rank words and a `candidateLabel` template that reads naturally in each language; never include glyphs or single-letter rank codes in any locale's value.

**Acceptance Criteria:**

- Given a Bitola hand where the candidate is the 9 of Spades and the picker chooses Diamonds in round 2, when `event:trump_selected` fires, then the dialog title reads "… took trump · Diamonds" and a diamond-colored suit chip is rendered in place of the playing card, with a small "candidate: Nine of Spades" caption visible (full English words, no `9♠` glyph, no `9S` code).
- Given the same scenario, when the dialog closes (timer or X), then the persistent `TrumpIndicator` and the dialog's caption agreed on the same suit (Diamonds) at all times — no contradiction frame.
- Given a round-1 pick where the candidate suit equals the chosen suit, when the dialog renders, then the visual and copy are identical to today (full `PlayingCard` of the candidate, eyebrow "Trump taken").
- Given the active locale is Serbian, Macedonian, or Croatian, when a round-2 pick happens, then the eyebrow and candidate-label texts render in that locale without falling back to English defaults.

## Spec Change Log

### 2026-05-09 — Iteration 1 patches (no spec amendment)

One review finding classified as **patch** (auto-fix, no re-derivation):

1. **T-rank IO Matrix coverage** — Edge Case Hunter flagged that the negative regex `\b[TJQKA]\b` and the "no `T♣`" claim from IO Matrix Row 3 (`cardId="TC"`, `trumpSuit="H"`) had no direct test case — only the round-2 `cardId="9S"` case ran through the assertions. Added a dedicated round-2 test for the `T`-rank candidate that asserts the full-word caption ("Ten of Clubs"), absence of `TC`, and the chosen-suit title ("Hearts" not "Clubs"). Strengthens regression protection at the highest-risk rank-letter that was previously untested.

Findings classified as **reject** (out of scope or non-issues):

- Defensive guards for `trumpSuit`/`cardId` malformation: out-of-contract per the wire schema; spec explicitly stayed additive.
- Round-2 same-suit pick mis-classification: impossible in Bitola (the candidate suit is disabled in round 2 by the existing rules — see `spec-bitola-round2-disable-candidate-suit`).
- Slavic locale grammar polish ("Devetka Pik"): spec accepted the `"{{rank}} {{suit}}"` template; user approved without grammar pushback. Punted to future i18n polish.
- Chip glyph `aria-hidden`: mirrors existing `TrumpIndicator` pattern; not regression introduced by this story.
- React key not including `trumpSuit`: spec explicitly forbids changing the key strategy; scenario (same `playerSeat`+`cardId` with different `trumpSuit`) is contrived.
- Locale-leak test flake: vitest worker isolation; not observed.

KEEP instructions for any future re-derivation:

- The `card.suit !== trumpSuit` round-detector is correct **specifically because Bitola disallows the candidate suit in round 2**. If this component is ever shared with the Croatian variant (deferred), the detector must be re-evaluated.
- `as Suit` cast at the GamePage boundary is intentional — the wire field is typed `string` per the contract while the component contract is `Suit`. Do not "fix" this with runtime validation; the validation belongs in `useWsDispatch` if anywhere.

## Verification

**Commands:**

- `cd client && npx vitest run src/features/game/components/TrumpReveal.test.tsx` -- expected: existing tests + new round-2 test all pass.
- `cd client && npx vitest run src/shared/i18n/i18n.test.ts` -- expected: parity test stays green after adding the two new keys to all four locales.
- `cd client && npx prettier --write .` -- expected: formats touched files.
- `make lint` -- expected: ESLint + Prettier + golangci-lint all green.

**Manual checks:**

- `make dev`, force a Bitola round-2 path (4 passes in round 1, then in round 2 pick a suit different from the candidate's). Confirm the centered dialog reads the chosen suit, shows the suit chip, and shows the candidate as a full-name caption like "candidate: Ten of Spades" (never `T♠` or `9C`-style codes) — and matches the top-right `TrumpIndicator`.
- Repeat with a round-1 pick. Confirm the dialog is visually identical to before this fix (full `PlayingCard`, eyebrow "Trump taken").
- Toggle each locale (en / sr / mk / hr) and re-trigger a round-2 reveal. Confirm the eyebrow + candidate caption translate.

## Suggested Review Order

**Round-detection and visual branching**

- Entry point: the new prop, the round-2 detector, and the eyebrow/caption derivations all live in one block.
  [`TrumpReveal.tsx:131-159`](../../client/src/features/game/components/TrumpReveal.tsx#L131-L159)

- Visual branch — chip vs PlayingCard — keyed by `isFreePick`. Mirrors `TrumpIndicator`'s parchment+halo for the chosen suit.
  [`TrumpReveal.tsx:193-242`](../../client/src/features/game/components/TrumpReveal.tsx#L193-L242)

- Eyebrow now driven by the precomputed `eyebrow` variable (round-1 vs round-2 copy).
  [`TrumpReveal.tsx:250`](../../client/src/features/game/components/TrumpReveal.tsx#L250)

- Candidate caption renders only on free pick, anchored under the title with the localized full-name template.
  [`TrumpReveal.tsx:262-270`](../../client/src/features/game/components/TrumpReveal.tsx#L262-L270)

**Localization helpers and constants**

- New `rankNameKey` mirrors the existing `suitNameKey`; locked to the canonical 8-rank union.
  [`TrumpReveal.tsx:68-89`](../../client/src/features/game/components/TrumpReveal.tsx#L68-L89)

- `RANK_NAME` / `SUIT_GLYPH` provide pure-English defaults so missing-key fallbacks never expose `T♣`-style codes.
  [`TrumpReveal.tsx:33-49`](../../client/src/features/game/components/TrumpReveal.tsx#L33-L49)

**Wiring and i18n**

- GamePage threads `trumpSuit` through; the `as Suit` cast at the wire boundary is intentional.
  [`GamePage.tsx:1007`](../../client/src/features/game/GamePage.tsx#L1007)

- New `game.ranks` block + `eyebrowFreeChoice` + `candidateLabel`. Parity test enforces all four locales.
  [`en.json:327-342`](../../../client/src/shared/i18n/en.json#L327-L342)

- Serbian Latin counterparts.
  [`sr.json:327-342`](../../../client/src/shared/i18n/sr.json#L327-L342)

- Macedonian counterparts.
  [`mk.json:327-342`](../../../client/src/shared/i18n/mk.json#L327-L342)

- Croatian counterparts.
  [`hr.json:327-342`](../../../client/src/shared/i18n/hr.json#L327-L342)

**Tests**

- Round-2 free pick — primary regression: title, chip, no playing card, full-word caption, no glyphs/codes.
  [`TrumpReveal.test.tsx:177`](../../client/src/features/game/components/TrumpReveal.test.tsx#L177)

- T-rank candidate (added in iteration 1 patch) — guards specifically against `T` leaking into user-facing text.
  [`TrumpReveal.test.tsx:216`](../../client/src/features/game/components/TrumpReveal.test.tsx#L216)

- Round-1 path: confirms zero behavioral drift when suits match.
  [`TrumpReveal.test.tsx:244`](../../client/src/features/game/components/TrumpReveal.test.tsx#L244)
