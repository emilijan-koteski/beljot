---
title: "Bitola round-2 trump prompt locks out the original candidate suit"
type: "feature"
created: "2026-04-30"
status: "in-review"
baseline_commit: "2fc3c7c"
context:
  - _bmad-output/archive/implementation-artifacts/spec-bitola-round2-trump-prompt-shows-candidate.md
  - _bmad-output/archive/implementation-artifacts/3-2-trump-bidding-bitola-variant.md
  - _bmad-output/project-context.md
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** In Bitola round-2 bidding the active bidder currently sees four enabled suit buttons (S/H/D/C) and may legally pick the same suit that was the round-1 face-up candidate. House rules say that suit is already "spent" — round 2 is meant for choosing a _different_ trump than the one that nobody wanted in round 1. Today the UI offers it and the server happily accepts it.

**Approach:** In round 2, render the candidate-suit button visibly but in a disabled state alongside the other three suit buttons (no layout shift, same grid). On the server, reject any `pick_trump` action in round 2 whose `Suit` equals `TrumpCandidate.Suit` with `ErrInvalidBid`. Round 1, the reshuffle path, and the candidate-card render above the grid are unchanged.

## Boundaries & Constraints

**Always:**

- The disabled suit button stays in its grid slot, keeps its suit symbol, and uses standard shadcn/ui `Button disabled` styling — no extra copy, badge, or tooltip.
- The disabled button must be inert: `disabled` attribute set so click does not fire `onPick`, with `aria-disabled="true"` for assistive tech.
- Backend stays the source of truth: even if the disabled button were bypassed, `handlePickTrump` rejects `Suit == TrumpCandidate.Suit` in round 2 with `ErrInvalidBid` before any state mutation.
- The four-button grid order (S, H, D, C) is preserved — only the matching suit's `disabled` flag flips.
- Existing `data-testid="trump-prompt-suit-{S}"` selectors remain on every button, including the disabled one, so test selectors still resolve.

**Ask First:**

- If the candidate-suit lockout would leave a player with no legal move in some yet-unobserved state (e.g. `TrumpCandidate` somehow nil mid-round-2), pause and ask before adding a fallback — round 2 always has a non-nil candidate by construction (`handlePickTrump`'s existing guard at `bidding.go:70`).

**Never:**

- Don't change round 1 behavior, the PASS button, the candidate card render above the grid, the dialog frame, the title copy, or any i18n keys.
- Don't add a tooltip, sub-label, or new i18n string explaining _why_ the suit is disabled — the visible candidate card above the grid already communicates it.
- Don't introduce a new error code or new WS event — reuse `apperr.ErrInvalidBid`.
- Don't touch the reshuffle path, stage-2 distribution, instant-win check, or `cloneGameState`.
- Don't gate the disabled state on `TrumpCandidate` being non-null only — in round 2 it is guaranteed non-null; if it ever is null defensively render all four enabled (matches today's behavior, since the server's `bidding.go:70` guard already rejects pick_trump when the candidate is missing).

## I/O & Edge-Case Matrix

| Scenario                                        | Input / State                                                   | Expected Output / Behavior                                                         | Error Handling  |
| ----------------------------------------------- | --------------------------------------------------------------- | ---------------------------------------------------------------------------------- | --------------- |
| Frontend, round 2 active bidder, candidate = KH | `biddingRound=2`, `isActiveBidder=true`, `trumpCandidate={K,H}` | Four buttons render; H is `disabled` and `aria-disabled="true"`; S/D/C are enabled | n/a             |
| Frontend, click disabled candidate-suit button  | round 2, candidate = KH, user clicks `trump-prompt-suit-H`      | `onPick` is not invoked                                                            | n/a             |
| Frontend, click another suit                    | round 2, candidate = KH, user clicks `trump-prompt-suit-S`      | `onPick("S")` fires once                                                           | n/a             |
| Frontend, round 1 active bidder                 | `biddingRound=1`, candidate present                             | No suit-button grid (regression guard) — round 1 layout unchanged                  | n/a             |
| Backend, round 2 pick of candidate suit         | `BiddingRound=2`, `TrumpCandidate.Suit=H`, action `Suit=&H`     | Returns `(nil, ErrInvalidBid)`; original state untouched                           | `ErrInvalidBid` |
| Backend, round 2 pick of other suit             | `BiddingRound=2`, `TrumpCandidate.Suit=H`, action `Suit=&S`     | Stage-2 distribution proceeds; `TrumpSuit=&S`; phase → `playing`                   | n/a             |
| Backend, round 1 pick (regression)              | `BiddingRound=1`                                                | Trump locked to candidate suit; `action.Suit` ignored                              | n/a             |

</frozen-after-approval>

## Code Map

- `client/src/features/game/components/TrumpPrompt.tsx` — In the round-2 grid, derive `const lockedSuit = trumpCandidate?.suit ?? null` and pass `disabled={suit === lockedSuit}` plus `aria-disabled={suit === lockedSuit}` to each `Button`. No other prop changes.
- `client/src/features/game/components/TrumpPrompt.test.tsx` — Add tests: locked suit button is `disabled` and clicking it does not fire `onPick`; other suit buttons remain enabled and fire `onPick`; round-1 grid is unaffected (regression).
- `server/internal/game/bidding.go` — In `handlePickTrump` round-2 branch, after the existing `validSuits` check, add an equality check: `if state.TrumpCandidate != nil && *action.Suit == state.TrumpCandidate.Suit { return nil, apperr.ErrInvalidBid }`. Keep the existing nil-candidate guard at the top of `handlePickTrump` untouched.
- `server/internal/game/bidding_test.go` — (1) Update `TestPickTrumpRound2` "pick hearts (same as candidate)" case to expect `ErrInvalidBid` (or relocate to a new dedicated test). (2) Drop the "round 2, picker = seat 1, suit = hearts (same as candidate)" sub-case from `TestPickTrumpStage2Rotation`. (3) Add a focused test `TestRound2PickCandidateSuitRejected` that asserts `ErrInvalidBid` for the candidate suit across multiple `passCount`s and verifies the original state is not mutated.
- `server/internal/game/testfixtures/fixtures_test.go` — In `TestNewGameJustDealt_NoInstantWinOnAnyPick`, skip the candidate suit (`gs.TrumpCandidate.Suit`) in the round-2 inner loop so the property test continues to pass under the new rule.

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/bidding.go` — Added the `*action.Suit == state.TrumpCandidate.Suit` rejection in the round-2 branch of `handlePickTrump`, returning `apperr.ErrInvalidBid` (after the existing nil/`validSuits` checks, before any state mutation).
- [x] `server/internal/game/bidding_test.go` — Removed the now-invalid "pick hearts (same as candidate) in round 2" case from `TestPickTrumpRound2`, dropped the analogous sub-case from `TestPickTrumpStage2Rotation`, and added `TestRound2PickCandidateSuitRejected` covering 4 seats × 4 passCounts with state-immutability assertions on phase, round, pass count, active seat, trump suit, and candidate.
- [x] `server/internal/game/testfixtures/fixtures_test.go` — Updated `TestNewGameJustDealt_NoInstantWinOnAnyPick` to skip `gs.TrumpCandidate.Suit` in the round-2 inner loop so the no-instant-win property holds under the lockout.
- [x] `client/src/features/game/components/TrumpPrompt.tsx` — Compute `isLocked = trumpCandidate?.suit === suit` per button in round 2 and pass `disabled` + `aria-disabled` accordingly. Layout, classes, and `data-testid`s unchanged.
- [x] `client/src/features/game/components/TrumpPrompt.test.tsx` — Switched the existing "calls onPick with suit when suit button clicked in round 2" test from clicking H to clicking S (S is unlocked); added `disables the candidate-suit button in round 2 and leaves the others enabled` and `does not call onPick when the disabled candidate-suit button is clicked`.

**Acceptance Criteria:**

- Given a Bitola game in bidding round 2 with `trumpCandidate.suit === "H"` and the local player is the active bidder, when the `TrumpPrompt` renders, then the `trump-prompt-suit-H` button is present in the grid and is `disabled` with `aria-disabled="true"`, while `trump-prompt-suit-S`, `trump-prompt-suit-D`, and `trump-prompt-suit-C` are enabled.
- Given the same state, when the user clicks the disabled `trump-prompt-suit-H`, then `onPick` is not invoked.
- Given the same state, when the user clicks `trump-prompt-suit-S`, then `onPick("S")` fires exactly once.
- Given a Bitola game in bidding round 2 with `TrumpCandidate.Suit == SuitHearts`, when `ApplyAction` processes a `pick_trump` action with `Suit=&SuitHearts` from the active bidder, then it returns `(nil, apperr.ErrInvalidBid)` and the input state is not mutated.
- Given the same backend state, when the active bidder submits `pick_trump` with any suit other than hearts, then stage-2 distribution proceeds and `TrumpSuit` is locked to that chosen suit (regression guard for the unaffected path).
- Given a Bitola game in bidding round 1 (regression guard), when the prompt renders for the active bidder, then no suit-button grid is shown and the round-1 PICK + PASS layout is unchanged; on the server, round-1 `pick_trump` continues to lock trump to the candidate suit and ignores `action.Suit`.

## Spec Change Log

## Verification

**Commands:**

- `cd server && go test ./internal/game/...` — expected: all bidding and fixture tests pass, including the new `TestRound2PickCandidateSuitRejected` and the updated `TestPickTrumpRound2` / `TestPickTrumpStage2Rotation` cases.
- `cd client && npx vitest run src/features/game/components/TrumpPrompt.test.tsx` — expected: all TrumpPrompt tests pass, including the new disabled-button cases.
- `cd client && npx vitest run` — expected: full frontend suite green.
- `cd client && npx prettier --write .` — expected: formatting normalized (per memory rule before any client commit).
- `make lint` — expected: no errors.

**Manual checks:**

- Run `make dev`, take all four seats through round 1 passes, then as the active bidder in round 2 confirm the candidate-suit button is visibly disabled and unclickable while the other three pick a suit and transition to playing phase.
