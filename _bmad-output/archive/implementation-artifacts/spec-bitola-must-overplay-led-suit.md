---
title: "Bitola: must overplay highest led non-trump suit card"
type: "bugfix"
created: "2026-04-25"
status: "done"
context: []
baseline_commit: "bdacfb6da008b9602c5f432a0c80c98d84f1cad1"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Bitola variant's "must overplay" rule is only enforced when the led suit is trump (existing over-trump / _iber_ logic). When the led suit is non-trump, players who can follow suit may currently play any card of that suit — including a card lower than the highest led-suit card already on the table. Per the Bitola variant's full following-suit rule, that lower card must not be a legal play.

**Approach:** Extend the existing must-overplay logic so it applies to the led suit regardless of whether it is trump. When a player has cards of the led suit, they must play one strictly higher than the highest led-suit card currently in the trick (using the led suit's rank ordering — `NonTrumpRankOrder` when the led suit is non-trump, `TrumpRankOrder` when it is trump). If no such card exists in hand, any same-suit card is legal (rule 2). Void-in-led-suit behavior (rule 3 / trump obligation) is unchanged.

## Boundaries & Constraints

**Always:**

- Server `legalCards` is the single source of truth; client `legalCards.ts` mirrors it byte-for-byte in semantics.
- Server `isCardLegal` (used by `handlePlayCard`) and `AutoPlay` automatically inherit the new rule by reusing `legalCards`.
- Use `NonTrumpRankOrder` for non-trump led suits and `TrumpRankOrder` for trump-led; never compare ranks across orderings.
- Rule 3 unchanged: when void in the led suit, partner-winning exemption and trump obligation behave exactly as today.
- Variant gating: Bitola is currently the only variant; do not introduce conditional branching on `state.Variant` for this rule. If other variants are added later, that gating is a separate concern.

**Ask First:**

- Refactoring scope beyond this rule (e.g. unifying `applyOverTrump` into a generalized helper) — propose, don't enact, if the scope creeps.

**Never:**

- Do not change `currentTrickWinnerSeat`, declaration logic, scoring, or trick resolution.
- Do not introduce a `state.Variant`-based code path; keep the rule unconditional under Bitola.
- Do not alter how the client derives `ledSuit` (still from `currentTrick[0]`, not `state.leadSuit`) — that race-mitigation pattern stays.
- Do not silently relax existing trump over-trump behavior.

## I/O & Edge-Case Matrix

Hand reference: H = hearts (led, non-trump), D = diamonds (trump). Trick sequence is `[(seat, card)]`.

| Scenario                                      | Input / State                                                | Expected `legalCards` | Notes                                                                             |
| --------------------------------------------- | ------------------------------------------------------------ | --------------------- | --------------------------------------------------------------------------------- |
| Lower follow-suit blocked                     | hand=`[8H, AH, KC, AC]`, trick=`[(prev, KH)]`, trump=D       | `[AH]`                | Core fix — 8H excluded                                                            |
| All led-suit lower than highest on table      | hand=`[7H, 8H]`, trick=`[(prev, KH)]`, trump=D               | `[7H, 8H]`            | Rule 2: no higher → all same-suit legal                                           |
| Multiple higher led-suit cards                | hand=`[QH, KH, AH]`, trick=`[(prev, JH)]`, trump=D           | `[QH, KH, AH]`        | All strictly higher are legal                                                     |
| Highest led-suit on table is from partner     | hand=`[8H, AH]`, trick=`[(partner, KH), (opp, 7H)]`, trump=D | `[AH]`                | Rule still applies — partner exemption is only for void case (rule 3)             |
| Trump played over led non-trump               | hand=`[8H, AH]`, trick=`[(prev, KH), (opp, 7D)]`, trump=D    | `[AH]`                | Highest _led-suit_ card is KH; trump in trick doesn't relax the led-suit overplay |
| Void in led suit, opponent winning, has trump | hand=`[KD, 7C]`, trick=`[(opp, AH)]`, trump=D                | `[KD]`                | Unchanged — rule 3 / trump obligation                                             |
| Void in led suit, partner winning             | hand=`[KD, 7C]`, trick=`[(partner, AH)]`, trump=D            | `[KD, 7C]`            | Unchanged — partner exemption                                                     |
| Trump-led over-trump (regression)             | hand=`[7D, KD]`, trick=`[(prev, QD)]`, trump=D               | `[KD]`                | Existing trump path must keep working                                             |
| Trump-led, no over-trump available            | hand=`[7D, 8D]`, trick=`[(prev, JD)]`, trump=D               | `[7D, 8D]`            | Existing path: lower trumps legal when no over-trump                              |
| Leading the trick                             | hand=`[8H, AH, ...]`, trick=`[]`                             | full hand             | Unchanged                                                                         |

</frozen-after-approval>

## Code Map

- `server/internal/game/validation.go` — `legalCards`, `applyOverTrump`, `highestTrumpInTrick`. Add new helper `applyMustOverplayLedSuit` (or refactor existing) and call it on the follow-suit branch. Single source of truth for legality.
- `server/internal/game/validation_test.go` — Server-side legality tests. Add cases from the I/O matrix.
- `server/internal/game/playing.go` — Calls `isCardLegal` on play; no edit needed (inherits via `legalCards`).
- `server/internal/game/auto_play.go` — `AutoPlay` consumes `legalCards`; no edit needed.
- `client/src/features/game/lib/legalCards.ts` — Client mirror. Add the matching helper, update the follow-suit branch.
- `client/src/features/game/lib/legalCards.test.ts` — Client legality tests. Mirror added cases.
- `client/src/features/game/components/HandCards.tsx` — Consumer of `legalCardIds`; no edit needed (already styles unplayable cards).

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/validation.go` -- Add `applyMustOverplayLedSuit(suitCards, trick, ledSuit, trumpSuit) []Card` returning the led-suit cards in hand strictly higher than the highest led-suit card in the trick (using `TrumpRankOrder` if `ledSuit == trumpSuit`, else `NonTrumpRankOrder`); in `legalCards`, when `len(suitCards) > 0`, call it and return its result if non-empty, else return `suitCards`. The existing trump-led `applyOverTrump` branch is replaced by this unified call. -- One rule for both led-suit cases; matches user-described semantics.
- [x] `server/internal/game/validation_test.go` -- Add tests covering each row of the I/O matrix that touches non-trump-led overplay, plus a regression test for trump-led over-trump going through the new path. -- Lock in new rule and prevent regression of trump path.
- [x] `client/src/features/game/lib/legalCards.ts` -- Mirror the server change: add `applyMustOverplayLedSuit` (using `NON_TRUMP_RANK_ORDER` / `TRUMP_RANK_ORDER` parallel to the server) and update the follow-suit branch. Preserve `currentTrick[0]`-derived `ledSuit`. -- Keeps client/server semantics identical so UI highlighting matches server validation.
- [x] `client/src/features/game/lib/legalCards.test.ts` -- Mirror server tests: add the same I/O matrix cases. -- Symmetric regression coverage on the client.

**Acceptance Criteria:**

- Given the scenario in the attached image (kiro holds `[8H, AH, KC, AC]`, trick is `[(prev, KH)]`, trump is diamonds), when computing legal cards for kiro, then only `AH` is returned (server and client agree).
- Given a player holds `[7H, 8H]` and the trick is `[(prev, KH)]` with trump diamonds, when computing legal cards, then both `7H` and `8H` are legal (rule 2 — no higher led-suit available).
- Given the same trump-led over-trump scenarios that pass today, when running the existing test suite, then all current trump tests continue to pass without modification.
- Given the server rejects an illegal card play via `handlePlayCard`, when a client attempts to play `8H` in the matrix's first scenario, then the server returns the existing illegal-play error and the card is not played.
- Given the client highlights playable cards, when the same scenario is on screen, then `8H` is rendered as unplayable and `AH` as playable.

## Spec Change Log

### 2026-04-25 — review iteration 1 (patches only, no loopback)

- **Acceptance auditor flagged** two I/O matrix rows lacking dedicated test coverage: "multiple higher led-suit cards" (server + client) and "highest led-suit on table is from partner" (server only — client was covered).
- **Patches applied** without changing the spec or the implementation: added two server subtests under `TestValidationMustOverplayLedSuit` (`multiple higher led-suit cards — all strictly higher are legal` and `must overplay led-suit when highest card on table came from partner`) and one client test (`multiple higher led-suit cards — all strictly higher are legal`).
- **Other reviewer findings classified reject:** Go-`nil`-vs-TS-`[]` style nit (both safe via `len()`/`length` guards); missing-rank-in-map defensive validation (violates project guidance against validation for unreachable cases — `Rank` is an enumerated type); `applyOverTrump` nil-branch concern (verified equivalent under the unified helper since `currentTrick[0].suit == ledSuit` always holds).

## Verification

**Commands:**

- `cd server && go test ./internal/game/...` — expected: all tests pass, including new I/O matrix cases.
- `cd client && npm run test -- legalCards` — expected: all `legalCards` tests pass, including new cases.
- `make lint` (from repo root) — expected: clean.
- `cd client && npx prettier --write .` — expected: format the changed TS files before commit (per CLAUDE memory).

**Manual checks:**

- Reproduce the screenshot scenario in a local dev game (or via fixtures): confirm `8H` is shown as unplayable and clicking it does nothing; confirm `AH` plays normally.

## Suggested Review Order

**Server rule (source of truth)**

- Entry point: the unified follow-suit branch now calls `applyMustOverplayLedSuit` for both trump-led and non-trump-led tricks.
  [`validation.go:34`](../../../server/internal/game/validation.go#L34)

- New helper picks the rank ordering by whether the led suit is trump and filters strictly higher cards.
  [`validation.go:61`](../../../server/internal/game/validation.go#L61)

**Client mirror (UI legality highlighting)**

- The client `legalCards` follows the same shape, retaining the `currentTrick[0]`-derived `ledSuit` race fix.
  [`legalCards.ts:121`](../../client/src/features/game/lib/legalCards.ts#L121)

- TypeScript twin of the server helper — same rank-order selection.
  [`legalCards.ts:59`](../../client/src/features/game/lib/legalCards.ts#L59)

**Tests**

- New server table covers the screenshot scenario, rule-2 fallback, trump-played-over-led-suit, multi-higher, partner-played-highest, and a trump-led regression.
  [`validation_test.go:202`](../../../server/internal/game/validation_test.go#L202)

- Client mirrors of the new I/O matrix rows; existing trump-led tests preserved.
  [`legalCards.test.ts:148`](../../client/src/features/game/lib/legalCards.test.ts#L148)
