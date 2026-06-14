---
title: "Bitola: led-suit overplay only while the led suit can still win"
type: "bugfix"
created: "2026-06-14"
status: "done"
context:
  [
    "{project-root}/_bmad-output/project-context.md",
    "{project-root}/_bmad-output/implementation-artifacts/spec-bitola-must-overplay-led-suit.md",
    "{project-root}/_bmad-output/implementation-artifacts/spec-bitola-must-trump-when-void-no-partner-exemption.md",
  ]
baseline_commit: "55a433a"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `legalCards` enforces the led-suit overplay (_iber_) obligation **unconditionally** on the follow-suit branch, so when a non-trump-led trick has already been cut by a trump, a player holding the led suit is still forced to play a higher led-suit card. That is wrong: the trump is already winning, the led suit can no longer take the trick. `spec-bitola-must-overplay-led-suit.md` over-applied the rule (its "Trump played over led non-trump → [AH]" row is the exact bug).

**Approach:** Gate the led-suit overplay to apply **only while the led suit can still win** — when `ledSuit == trumpSuit` (trump led) **or** no trump is present in the trick. Once a trump cuts a non-trump-led trick, the player must still **follow the led suit** but may play **any** card of it. Void-in-led-suit (over-trump else any trump, no partner exemption), void-in-both (any card), and trump-led over-trump are all unchanged. One predicate in the engine, mirrored in the client; the rest is tests, stale comments, and rule docs.

## Boundaries & Constraints

**Always:**

- Server `legalCards` stays the single source of truth; client `legalCards.ts` mirrors its semantics. `isCardLegal`, `AutoPlay`, and bots inherit by reusing `legalCards` / `game.LegalCards`.
- The gate is exactly: apply `applyMustOverplayLedSuit` iff `ledSuit == trumpSuit || highestTrumpInTrick(trick, trumpSuit) == nil` (reuse the existing helper on both sides).
- Following suit stays mandatory after a cut — off-suit is illegal whenever the player holds the led suit.
- The change is strictly more permissive on the follow-suit branch (only ever adds legal cards); do not narrow any other branch. Bitola is the only variant — no `state.Variant` branching.

**Ask First:**

- Any change to the void-in-led-suit branch, over-trump filter, `currentTrickWinnerSeat`, trick resolution, declarations, or scoring — all out of scope here.
- Rewording rule docs beyond Rule 3 (the "Cut by a trump?" block) in any language.

**Never:**

- Do not touch `applyOverTrump`, `highestTrumpInTrick`, `currentTrickWinnerSeat`, or `applyMustOverplayLedSuit`'s internals — only its **call site** is gated.
- Do not weaken Rule 1 (overplay still required when no trump has cut) or trump-led over-trump.
- Do not change rule-doc structure (block order/kinds/titles) — only Rule 3's `text` per language. Keep the parity test green.

## I/O & Edge-Case Matrix

Led non-trump suit = Spades (S), trump = Hearts (H). Trick = `[(seat, card)]`.

| Scenario                                            | Input / State                                                       | Expected `legalCards`     | Notes                                            |
| --------------------------------------------------- | ------------------------------------------------------------------- | ------------------------- | ------------------------------------------------ |
| **FIX** — non-trump led, cut by trump, holds led    | hand=`[8S, AS, KC]`, trick=`[(p, KS), (p, 7S), (opp, 7H)]`, trump=H | `[8S, AS]`                | Was `[AS]`. Any spade now legal; KC still illegal |
| Off-suit still illegal after a cut                  | as above, attempt `KC`                                              | rejected (`ErrIllegalPlay`) | Must still follow the led suit                   |
| Non-trump led, **no** trump cut, holds higher led   | hand=`[8S, AS]`, trick=`[(p, KS)]`, trump=H                         | `[AS]`                    | Rule 1 preserved — overplay still required        |
| Void in led, trump on table — over-trump            | hand=`[9H, 8D]`, trick=`[(p, KS), (opp, 7H)]`, trump=H              | `[9H]`                    | Unchanged void branch                            |
| Void in led + void in trump after a cut             | hand=`[KD, QC]`, trick=`[(p, KS), (opp, 7H)]`, trump=H              | `[KD, QC]`                | Unchanged — any card                             |
| Trump led — over-trump (regression)                 | hand=`[7H, JH]`, trick=`[(p, QH)]`, trump=H                         | `[JH]`                    | `ledSuit == trumpSuit` keeps the obligation       |
| Leading                                             | trick=`[]`                                                          | full hand                 | Unchanged                                        |

</frozen-after-approval>

## Code Map

- `server/internal/game/validation.go` — `legalCards` follow-suit branch: gate the `applyMustOverplayLedSuit` call on led-suit-can-still-win. Update the function doc comment (rules 1–3 header) and the inline comment at the call site. **Single source of truth.**
- `server/internal/game/validation_test.go` — invert subtest `"must overplay led non-trump even after opponent trumped"` (now: 8S legal, AS legal, off-suit illegal); add a no-cut regression and a trump-led regression assertion if not already covered.
- `client/src/features/match/lib/legalCards.ts` — mirror the gate (uses existing `highestTrumpInTrick`); update the doc comment.
- `client/src/features/match/lib/legalCards.test.ts` — invert `"must overplay led non-trump even when an opponent already trumped"` (now both `8H` and `AH` legal); add an off-suit-illegal-after-cut case.
- `server/internal/bot/bot.go` — update stale "Bitola's forced overplay" comments (~L199, L205); **no logic change** — bots consume `v.LegalCards` from the engine and adapt.
- `server/internal/game/declarations_test.go` — tidy the stale `completeTrick1` comment (L286: TD is now chosen, not forced); no behavior change.
- `client/src/features/rules/content/{en,hr,mk,sr}.ts` — rewrite **only** the Rule 3 (`"Cut by a trump?"`) `text` field per language; titles, Rules 1–2, and the closing paragraph stay.
- `_bmad-output/project-context.md` — update the "Three-layer card validation" rule (~L325) to state the overplay is gated to "while the led suit can still win".

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/validation.go` — In `legalCards`, compute `ledSuitCanWin := ledSuit == trumpSuit || highestTrumpInTrick(state.CurrentTrick, trumpSuit) == nil`; only call `applyMustOverplayLedSuit` when true, else return `suitCards`. Update doc comments. -- Core fix in the one authoritative place.
- [x] `server/internal/game/validation_test.go` — Invert the after-trump subtest and add the matrix's cut-then-any-led-suit, off-suit-illegal, and no-cut regression cases. -- Lock the new rule and prove Rule 1 / trump-led untouched.
- [x] `client/src/features/match/lib/legalCards.ts` — Mirror the gate; update doc comment. -- Keep client highlight identical to server legality.
- [x] `client/src/features/match/lib/legalCards.test.ts` — Invert the after-trump test; add off-suit-illegal-after-cut. -- Symmetric client coverage.
- [x] `server/internal/bot/bot.go` — Refresh the two "forced overplay" comments to "overplay only while led suit can win". -- Comment accuracy; no logic change.
- [x] `client/src/features/rules/content/en.ts`, `hr.ts`, `mk.ts`, `sr.ts` — Rewrite Rule 3 `text` to: follow the led suit with any card once cut; reach for trump only when void; over-trump a prior cut if able else any trump; otherwise play anything. -- Docs match engine; all four languages stay in sync.
- [x] `_bmad-output/project-context.md` — Amend the card-validation rule wording. -- Future agents inherit the corrected rule.
- [x] `server/internal/game/declarations_test.go` — Comment tidy only.

**Acceptance Criteria:**

- Given a non-trump-led trick already cut by a trump and a player who holds the led suit, when computing legal cards, then every card of the led suit is legal (server and client agree) and any off-suit card is rejected with `ErrIllegalPlay`.
- Given a non-trump-led trick with **no** trump played, when a player holds a higher led-suit card, then they are still forced to overplay (Rule 1 unchanged).
- Given the existing void-must-trump and trump-led over-trump suites, when re-run, then they pass unmodified.
- Given a bot at a seat whose trick has been cut by a trump while it holds the led suit, when it acts, then `ApplyAction` accepts its play (it now smears/ducks rather than being forced to overtake) — `go test ./internal/bot/...` stays green.
- Given the rules page in each of en/hr/mk/sr, when the play chapter is read, then Rule 3 no longer says "beat it with a higher card" and the parity + RulesPage tests pass.

## Design Notes

The whole behavior change is one predicate at the follow-suit call site (client mirror identical, using `=== null`):

```go
if len(suitCards) > 0 {
    ledSuitCanWin := ledSuit == trumpSuit || highestTrumpInTrick(state.CurrentTrick, trumpSuit) == nil
    if ledSuitCanWin {
        if higher := applyMustOverplayLedSuit(suitCards, state.CurrentTrick, ledSuit, trumpSuit); len(higher) > 0 {
            return higher
        }
    }
    return suitCards
}
```

## Verification

**Commands:**

- `cd server && go test ./internal/game/... ./internal/bot/... ./internal/match/...` -- expected: all pass, including the inverted + new validation cases.
- `cd client && npm run test -- legalCards` -- expected: all pass, including the inverted after-cut case.
- `cd client && npm run test -- rulesContent RulesPage` -- expected: parity + page tests stay green.
- `make lint` -- expected: clean.
- `cd client && npx prettier --write .` -- format changed TS before commit.

**Manual checks:**

- In a local dev game (Bitola), reproduce a non-trump-led trick that an opponent trumps: a low card of the led suit must now highlight as playable and play successfully; an off-suit card must stay unplayable.

## Suggested Review Order

**Rule logic (engine source of truth → client mirror)**

- Entry point — the one predicate that gates the led-suit overplay to "led suit can still win".
  [`validation.go:41`](../../server/internal/game/validation.go#L41)

- Byte-for-byte semantic mirror on the client (`=== null`); keeps UI highlighting in lockstep with the server.
  [`legalCards.ts:107`](../../client/src/features/match/lib/legalCards.ts#L107)

**Rule documentation**

- Rule 3 ("Cut by a trump?") rewritten — any led-suit card after a cut; hr/mk/sr mirror this verbatim.
  [`en.ts:153`](../../client/src/features/rules/content/en.ts#L153)

- Project rule corrected so future agents inherit the gated overplay, not the old unconditional one.
  [`project-context.md:325`](../project-context.md#L325)

**Bot adaptation**

- Comment-only refresh — bots consume the engine legal set and now smear/duck instead of force-overplaying.
  [`bot.go:203`](../../server/internal/bot/bot.go#L203)

**Tests**

- Inverted server case: after a trump cut, any spade is legal; off-suit and trump-in-hand stay illegal.
  [`validation_test.go:206`](../../server/internal/game/validation_test.go#L206)

- Mirrored client case: both hearts legal after the cut, off-suit and trump excluded.
  [`legalCards.test.ts:138`](../../client/src/features/match/lib/legalCards.test.ts#L138)
