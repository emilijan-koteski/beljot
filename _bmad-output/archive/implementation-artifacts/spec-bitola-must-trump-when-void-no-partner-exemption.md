---
title: "Bitola: must trump when void in led suit (remove partner exemption)"
type: "bugfix"
created: "2026-04-26"
status: "done"
context: ["spec-bitola-must-overplay-led-suit.md"]
baseline_commit: "4aa94d2"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The legal-move filter currently grants a "partner exemption" — when the active player is void in the led suit and their partner is currently winning the trick, the filter returns the entire hand. The user reports this is wrong for the Bitola variant: a void player must always cut with trump if they hold any trump, regardless of who currently wins. Three screenshots demonstrate the bug: (1) void + opponent already trumped → user sees full hand instead of only over-trumps, (2) void with trump in hand + partner winning a non-trump → user sees trumps + non-trumps instead of trumps only, (3) void + no trump on table yet + partner winning the led non-trump → user sees full hand instead of trumps only.

**Approach:** Remove the partner-exemption branch from `legalCards` in both server and client. When the player is void in the led suit and holds at least one trump, they must play a trump — over-trump if a trump is already on the table and any of their trumps beats it; otherwise any trump in hand. When the player is void and holds no trump, any card is legal. The existing `applyOverTrump` helper already implements the within-trump filtering correctly; the fix is purely the removal of the `isOpponentWinning(state, seat)` precondition. Keep `isOpponentWinning` and `currentTrickWinnerSeat` themselves — they are used by other features (auto-play sort tests reference winning state, `currentTrickWinnerSeat` is used by trick resolution).

## Boundaries & Constraints

**Always:**

- Server `legalCards` is the single source of truth; client `legalCards.ts` mirrors it byte-for-byte in semantics.
- Server `isCardLegal` (used by `handlePlayCard`) and `AutoPlay` automatically inherit the new rule by reusing `legalCards`.
- Use `TrumpRankOrder` consistently when comparing trumps; never compare across rank orderings.
- Bitola is currently the only variant; do not introduce variant-conditional branching for this rule.
- Update existing tests whose hardcoded plays relied on partner exemption (they are not exercising the rule under test — they used non-trump cards as filler that the old engine accepted).

**Ask First:**

- Re-introducing partner exemption for any future variant — this fix removes it unconditionally because the codebase has only Bitola.
- Removing `isOpponentWinning` itself or `currentTrickWinnerSeat` — both have other call sites.

**Never:**

- Do not change `currentTrickWinnerSeat`, declaration logic, scoring, trick resolution, or auto-play sort order.
- Do not relax the existing `applyOverTrump` filter — over-trump obligation within the trump set still applies.
- Do not silently weaken the led-suit overplay behavior fixed by `spec-bitola-must-overplay-led-suit.md` (must overplay highest led-suit card on table).
- Do not touch fixtures (`NewGameMidPlay`, `NewGameFirstTrick`, etc.) for this fix; only adjust card choices in tests so they remain legal under the new rule.

## I/O & Edge-Case Matrix

Hand reference: H = hearts (trump in scenarios 1 + 3, also a trump suit in scenario 2 mirror), S = spades, D = diamonds, C = clubs. Trick sequence is `[(seat, card)]`.

| #   | Scenario                                                                   | Input / State                                                                                                                                   | Expected `legalCards`                                                        | Notes                                                                                                            |
| --- | -------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| 1   | Void in led non-trump, opponent already trumped, must over-trump           | hand=`[9H, TH, 8D, TD, JD, QD, AD]`, trick=`[(opp, 9C), (partner, QH), (opp, 7C)]`, trump=H, ledSuit=C, mySeat=Team A(opp partner of QH player) | `[TH]` only? → No, both `9H` and `TH` over-trump QH(2). Expected `[9H, TH]`. | **img_two** scenario. Highest trump on table = QH(rank 2). 9H(6), TH(4) both > 2 → both legal. Diamonds illegal. |
| 2   | Void in led non-trump, no trump on table, has one trump                    | hand=`[8S, JD, KD]`, trick=`[(opp, 9C), (opp, TC), (partner, AC)]`, trump=S, ledSuit=C                                                          | `[8S]`                                                                       | **img_three** scenario. No trump on table → any trump legal → only 8S. JD/KD illegal.                            |
| 3   | Void in led non-trump, no trump on table, multiple trumps                  | hand=`[7H, 8H, TH, KH, 7D, 8D, TD]`, trick=`[(opp, 7S), (partner, 9S)]`, trump=H, ledSuit=S                                                     | `[7H, 8H, TH, KH]`                                                           | **img_four** scenario. No trump on table → any trump legal. Diamonds illegal.                                    |
| 4   | Void, has trump, partner currently winning a non-trump (no trump on table) | hand=`[8C, 9C, AH]`, trick=`[(partner, TD), (opp, 7D)]`, trump=C, ledSuit=D                                                                     | `[8C, 9C]`                                                                   | Direct rebuttal of partner exemption — must trump even though partner is winning.                                |
| 5   | Void, has trump, partner currently winning with their own trump            | hand=`[QH, KH, 7C]`, trick=`[(opp, AS), (partner, 9H)]`, trump=H, ledSuit=S                                                                     | `[QH, KH]` (any trump, no over-trump available)                              | Falls back to any trump because QH(2) and KH(3) don't beat 9H(6). 7C illegal.                                    |
| 6   | Void in led trump suit, partner winning, has trumps                        | hand=`[7H, 8H, AC]`, trick=`[(opp, 9H), (partner, JH)]`, trump=H, ledSuit=H                                                                     | `[7H, 8H]`                                                                   | Trump-led path. JH(7) is highest, neither 7H nor 8H over-trumps → any trump → both legal, AC illegal.            |
| 7   | Void, no trump in hand, partner winning                                    | hand=`[KC, QC, 7D]`, trick=`[(partner, AS)]`, trump=H, ledSuit=S                                                                                | `[KC, QC, 7D]`                                                               | Unchanged — branch 2b: no trump → any card legal.                                                                |
| 8   | Void, no trump in hand, opponent winning                                   | hand=`[KC, QC, 7D]`, trick=`[(opp, AS)]`, trump=H, ledSuit=S                                                                                    | `[KC, QC, 7D]`                                                               | Unchanged — same branch 2b.                                                                                      |
| 9   | Player has led suit (regression)                                           | hand=`[8H, AH, KC]`, trick=`[(opp, KH)]`, trump=D, ledSuit=H                                                                                    | `[AH]`                                                                       | Regression on the prior overplay fix — must overplay KH(5) → only AH(7). Branch 1 unchanged.                     |
| 10  | Leading the trick (regression)                                             | hand any, trick=`[]`                                                                                                                            | full hand                                                                    | Unchanged.                                                                                                       |

</frozen-after-approval>

## Code Map

- `server/internal/game/validation.go` — `legalCards`. Remove the `isOpponentWinning(state, seat) &&` precondition on the trump-obligation branch. Update the doc comment for rule 2/3.
- `server/internal/game/validation_test.go` — Add tests for scenarios 1, 2, 3, 4, 5. Invert `TestValidationPartnerTrumped`'s "void in led suit and partner trumped winning - any card legal" sub-test to assert non-trump is illegal and over-trump is enforced.
- `server/internal/game/playing_test.go` — `TestPlayCardPartnerExemption`: invert the "any card legal" assertion to expect `ErrIllegalPlay` and add an assertion that a trump card is legal. `TestTrickResolution` "4th card triggers resolution" and `TestFull8TrickHand`: change seat 2's trick-1 play from AD to a trump card so the sequence remains legal under the new rule, and recompute downstream point assertions accordingly.
- `server/internal/game/declarations_test.go` — `completeTrick1`: change seat 0's play from `7C` to `QH` (trump, falls through to "any trump" since QH(2) cannot beat 9H(6) on table). Comment update.
- `client/src/features/game/lib/legalCards.ts` — Mirror the server change: drop the `isOpponentWinning` precondition. Remove the now-unused `isOpponentWinning` and `currentTrickWinnerSeat` helpers (no other call sites in this file).
- `client/src/features/game/lib/legalCards.test.ts` — Add tests mirroring scenarios 1–5. Invert the existing `partner exemption: void in led suit, partner winning → any card` test — under the new rule it must return only the trump.

## Tasks & Acceptance

**Execution:**

- [ ] `server/internal/game/validation.go` — Remove `isOpponentWinning(state, seat) &&` from the trump-obligation branch in `legalCards`. Update the function's doc comment to reflect the unconditional must-trump-when-void rule.
- [ ] `server/internal/game/validation.go` — Drop now-unused `isOpponentWinning` if no other callers remain (verify via grep before removal). `currentTrickWinnerSeat` is used by trick resolution and stays.
- [ ] `server/internal/game/validation_test.go` — Add `TestValidationMustTrumpWhenVoid` table-driven test covering matrix scenarios 1–5. Update `TestValidationPartnerTrumped` second sub-test to assert non-trump rejected and over-trump (or any-trump fallback) enforced.
- [ ] `server/internal/game/playing_test.go` — Update `TestPlayCardPartnerExemption` to assert `ErrIllegalPlay` on the non-trump play and a separate legal trump play. Update `TestTrickResolution` and `TestFull8TrickHand` plays + assertions for the new legal-card constraint at seat 2 trick 1.
- [ ] `server/internal/game/declarations_test.go` — Replace `completeTrick1` seat 0 play `7C` with `QH`; the trick winner stays seat 2 (9H trump beats QH and TD).
- [ ] `client/src/features/game/lib/legalCards.ts` — Mirror server: drop `isOpponentWinning` precondition. Remove unused helpers. Update doc comment.
- [ ] `client/src/features/game/lib/legalCards.test.ts` — Add tests mirroring matrix scenarios 1–5. Invert the existing partner-exemption test to assert only the trump is legal.

**Acceptance Criteria:**

- Given the img_two scenario (kiro is seat 0, void in clubs, hand contains `[9H, TH, 8D, TD, JD, QD, AD]`, trick = `[9C, QH, 7C]` from opponents and partner, trump = hearts), when computing legal cards for seat 0, then the result is `[9H, TH]` (server and client agree). The diamonds and lower hearts are excluded.
- Given the img_three scenario (emilijan is void in clubs, hand `[8S, JD, KD]`, trump = spades, trick = `[9C, TC, AC]` with partner winning), when computing legal cards, then the result is `[8S]` only.
- Given the img_four scenario (void in led non-trump, no trump on table yet, multiple trumps in hand), when computing legal cards, then the result is exactly the set of trumps in hand.
- Given the regression scenario where the player has led-suit cards, when computing legal cards, then the existing led-suit overplay rule is preserved (the prior spec's behavior).
- Given the server rejects an illegal card via `handlePlayCard`, when a void-with-trump player attempts a non-trump, then `ErrIllegalPlay` is returned and the card is not played.
- Given the client highlights playable cards, when the same scenarios are on screen, then non-trump cards in the void+has-trump branch render as unplayable.

## Spec Change Log

(none yet)

## Verification

**Commands:**

- `cd server && go test ./internal/game/...` — expected: all tests pass, including new void-must-trump cases.
- `cd client && npm run test -- legalCards` — expected: all tests pass.
- `make lint` — expected: clean.
- `cd client && npx prettier --write .` — format changed TS files before commit.

**Manual checks:**

- Reproduce img_two locally: void player should see only over-trumps highlighted as playable.
- Reproduce img_four locally (or via fixtures): void player with only one trump should see only that trump highlighted.
