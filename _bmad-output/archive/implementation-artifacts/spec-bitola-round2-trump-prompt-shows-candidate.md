---
title: "Phase 3 trump-suit dialog displays the originally face-up candidate card"
type: "feature"
created: "2026-04-26"
status: "done"
baseline_commit: "fb89d64"
context:
  - _bmad-output/archive/implementation-artifacts/spec-bitola-deal-flip-bid-flow.md
  - _bmad-output/project-context.md
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** In the Bitola "Choose Trump Suit" round (bidding round 2), the active bidder picks any of the four suits freely, but the originally face-up candidate card is still given to that player as their 8th card — same as round 1. The current `TrumpPrompt` dialog only renders the candidate in round 1, so a round-2 bidder sees four bare suit buttons with no reminder of which physical card they will receive when they pick. This is misleading: the suit choice and the inherited card are two separate facts the player needs to weigh together.

**Approach:** In `TrumpPrompt`, render the existing `gameState.trumpCandidate` above the suit buttons in round 2 as well, reusing the same `PlayingCard` rendering used by round 1. The card stays visible for the entire active-bidder round-2 view (until the player picks or passes). No backend change — `TrumpCandidate` is already kept on `GameState` until the pick resolves, and the candidate is already appended to the picker's hand in stage-2 distribution for both rounds (see `spec-bitola-deal-flip-bid-flow.md`).

## Boundaries & Constraints

**Always:**

- In round 2, when the active bidder views the prompt and `trumpCandidate` is non-null, render it as a `PlayingCard` above the suit-button grid using the same size and layout as the round-1 candidate render.
- Card stays visible for the entire round-2 active-bidder view; it is not hidden, dimmed, or replaced as the player hovers or pre-selects a suit.
- The four suit buttons remain unchanged in look, position, and `data-testid`s — the candidate is added above them, not woven into them.
- Non-active bidders in round 2 keep the existing waiting indicator unchanged.
- Backend `TrumpCandidate` lifecycle is already correct (cleared only inside `handlePickTrump` after stage-2). No server change.

**Ask First:**

- If the round-2 candidate would visually break the existing `max-w-[480px]` dialog frame on small viewports, flag before tweaking the dialog's max-width or padding.

**Never:**

- Don't change the suit-button layout, colors, symbols, or test IDs.
- Don't alter the `TrumpCandidate` lifecycle in `bidding.go` / `state.go` — round 2 already gets the candidate via the unconditional append in stage-2.
- Don't add a separate "what card will I receive?" tooltip or secondary affordance — the inline candidate card itself is the affordance.
- Don't change i18n keys (`titleRound2` already reads "Choose Trump Suit" / "Izaberi adut"), and don't add new copy.
- Don't touch round 1, the dealing animation, the trump-take reveal dialog, or any other bidding/declaration prompt.

## I/O & Edge-Case Matrix

| Scenario                                          | Input / State                                                                 | Expected Output / Behavior                                                                                                                   | Error Handling |
| ------------------------------------------------- | ----------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | -------------- |
| Round 2 active bidder, candidate present          | `biddingRound=2`, `isActiveBidder=true`, `trumpCandidate={rank:"A",suit:"H"}` | Dialog renders the candidate `PlayingCard` above the four suit buttons; PASS button visible; PICK button absent (round 2 has no PICK button) | n/a            |
| Round 2 active bidder picks a suit                | user clicks `trump-prompt-suit-S`                                             | `onPick("S")` fires; candidate render is unaffected by the click                                                                             | n/a            |
| Round 2 non-active bidder                         | `biddingRound=2`, `isActiveBidder=false`                                      | Existing waiting indicator unchanged; candidate not shown                                                                                    | n/a            |
| Round 1 active bidder                             | `biddingRound=1`, `isActiveBidder=true`, candidate present                    | Existing round-1 layout unchanged: candidate + PICK + PASS                                                                                   | n/a            |
| Round 2 active bidder, candidate null (defensive) | `trumpCandidate=null`                                                         | Suit buttons render; candidate slot is omitted gracefully (no error, no broken layout)                                                       | n/a            |

</frozen-after-approval>

## Code Map

- `client/src/features/game/components/TrumpPrompt.tsx` — Replace the round-1-gated candidate render with a round-agnostic block: render `<PlayingCard>` whenever `trumpCandidate` is non-null AND the dialog is in active-bidder mode (covers both rounds 1 and 2). Keep the round-2 suit-button grid below it.
- `client/src/features/game/components/TrumpPrompt.test.tsx` — Update existing round-2 tests to pass a non-null `trumpCandidate`. Add a focused test asserting the candidate card renders in round 2 (e.g., probe by `data-testid` on the rendered `PlayingCard` once it has one, or by `aria-label`/visible card text). Keep the round-1 assertions and the `null` defensive case intact (add a round-2 `null` case if not already covered).
- `server/internal/game/bidding_test.go` — Strengthen `TestPickTrumpStage2Rotation`'s round-2 cases with an explicit assertion that the originally captured candidate card is now in the picker's hand. Conservation already implies it; the explicit assertion locks the contract against future regression.

## Tasks & Acceptance

**Execution:**

- [x] `client/src/features/game/components/TrumpPrompt.tsx` — Moved the candidate render out of the `biddingRound === 1` branch so it renders in active-bidder mode for both rounds whenever `trumpCandidate` is non-null. Round-2 suit-button grid + PASS-only footer and round-1 PICK+PASS footer left intact; outer container `role="dialog"` / `aria-modal` / focus-trap unchanged.
- [x] `client/src/features/game/components/TrumpPrompt.test.tsx` — Updated the two existing round-2 tests to pass `trumpCandidate={trumpCandidate}` instead of `null`. Added three new tests: round-2 candidate renders (`getByTestId("playing-card-KH")`), round-1 candidate renders (regression guard), and a defensive case asserting no candidate render plus suit buttons still present when `trumpCandidate=null` in round 2.
- [x] `client/src/features/game/components/PlayingCard.tsx` (read-only check) — Confirmed `PlayingCard` already emits `data-testid="playing-card-{rank}{suit}"` (line 114). Used that selector — no new test ID needed.
- [x] `server/internal/game/bidding_test.go` — In `TestPickTrumpStage2Rotation`, captured `originalCandidate := *gs.TrumpCandidate` before the action and added `assert.Contains(t, result.Players[tc.picker].Hand, originalCandidate, ...)` after the result assertions. Covers all 7 sub-cases (round-1 every seat + round-2 spades/clubs/hearts).

**Acceptance Criteria:**

- Given a Bitola game in bidding round 2 and the local player is the active bidder, when the `TrumpPrompt` renders, then the originally face-up trump candidate card is visible above the four suit buttons using the same `PlayingCard` rendering as round 1.
- Given the same state, when the active bidder clicks any suit button, then `onPick(suit)` fires with that suit and the candidate render does not flicker, disappear, or change before the dialog unmounts.
- Given bidding round 1 (regression guard), when the prompt renders for the active bidder, then the existing round-1 layout (candidate + PICK + PASS) is unchanged.
- Given bidding round 2 and the local player is NOT the active bidder, when the prompt renders, then the existing waiting indicator is shown and the candidate is not displayed.
- Given any round-2 pick (any seat, any suit including the candidate's own suit), when `ApplyAction` resolves the pick, then `result.Players[picker].Hand` contains the originally face-up candidate card.

## Spec Change Log

- **2026-04-26 — Step-04 review (iteration 1, patch-only)**
  - Tightened `TestPickTrumpStage2Rotation` candidate-in-hand assertion from `assert.Contains` to a count==1 check (Edge-case hunter E2). Defends against a future fixture regression where the picker's stage-1 hand happens to contain a duplicate of the candidate, which would silently satisfy a membership-only check.
  - Added new test `does not render the candidate card for round 2 non-active bidder` with explicit `queryByTestId(/^playing-card-/)` negative assertion (Acceptance auditor A1). Locks the non-active early-return branch so a future refactor that moves the candidate render above the early-return is caught at test time.
  - Deferred: round-2 prompt vertical overflow on short mobile viewports (D97 in deferred-work.md). Pre-existing for round 1, worsened by adding the candidate above the suit grid in round 2. Cosmetic; mobile-only; not a blocker.
  - Rejected (blind hunter B2): claim that the un-gated candidate render leaks visibility to non-active bidders. False — the non-active branch early-returns at `TrumpPrompt.tsx:44` before any candidate render is reached. Confirmed by acceptance auditor.
  - Rejected (edge-case hunter E1): server-client desync where stale `trumpCandidate` could render after the server clears it. Pre-existing concern across the entire bidding lifecycle, not caused by this story. The prompt unmounts cleanly via `isBiddingPhase` flip in `GamePage.tsx:505`.

## Verification

**Commands:**

- `cd client && npx vitest run src/features/game/components/TrumpPrompt.test.tsx` — expected: all tests pass, including the new round-2 candidate-render test.
- `cd client && npx vitest run` — expected: full frontend suite green.
- `cd client && npx prettier --write .` — expected: formatting normalized (per memory rule before any client commit).
- `cd server && go test ./internal/game/...` — expected: all rules-engine tests pass, including the strengthened `TestPickTrumpStage2Rotation` candidate-in-hand assertion.
- `make lint` — expected: no errors.

**Manual checks:**

- Run `make dev`, seat as the active bidder in round 2, observe the dialog: candidate card visible above the four suit buttons; clicking a suit closes the dialog and the next state shows that card in your hand.

## Suggested Review Order

**UI behavior change**

- Three-line core: drop the `biddingRound === 1` guard so the candidate renders in both rounds for active bidders.
  [`TrumpPrompt.tsx:83-90`](../../client/src/features/game/components/TrumpPrompt.tsx#L83-L90)

- Confirms the non-active branch early-returns before the candidate render, so the un-gated render is correctly scoped.
  [`TrumpPrompt.tsx:44-57`](../../client/src/features/game/components/TrumpPrompt.tsx#L44-L57)

- Caller passes `gameState.trumpCandidate` regardless of round; backend already keeps it set through round 2.
  [`GamePage.tsx:505-513`](../../client/src/features/game/GamePage.tsx#L505-L513)

**Backend contract lock-in**

- Tightened from `assert.Contains` to count==1 so a future fixture regression with a duplicate candidate can't silently pass.
  [`bidding_test.go:497-528`](../../../server/internal/game/bidding_test.go#L497-L528)

**New test coverage**

- Round-2 candidate render is the headline ACC test (uses existing `playing-card-{rank}{suit}` test ID from `PlayingCard.tsx:114`).
  [`TrumpPrompt.test.tsx:115-129`](../../client/src/features/game/components/TrumpPrompt.test.tsx#L115-L129)

- Round-1 regression guard — the previously-coupled R1 candidate render is now covered by an explicit assertion.
  [`TrumpPrompt.test.tsx:131-142`](../../client/src/features/game/components/TrumpPrompt.test.tsx#L131-L142)

- Defensive R2 + null candidate case — suit grid still works when the candidate prop is somehow null.
  [`TrumpPrompt.test.tsx:144-157`](../../client/src/features/game/components/TrumpPrompt.test.tsx#L144-L157)

- Locks the non-active early-return boundary against future refactors that move the candidate render above it.
  [`TrumpPrompt.test.tsx:159-173`](../../client/src/features/game/components/TrumpPrompt.test.tsx#L159-L173)
