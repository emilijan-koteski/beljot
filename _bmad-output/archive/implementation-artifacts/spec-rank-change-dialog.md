---
title: 'Season rank-change dialog after a match'
type: 'feature'
created: '2026-09-26'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: ['{project-root}/_bmad-output/implementation-artifacts/spec-13-5-divisions-demotion-and-sp-feedback.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 13.5 announces a season rank change with toasts: `toast.success` for a promotion, `toast.info` for a demotion. The owner wants a post-match dialog instead, "same as lvl up", for both rank up and rank down (request 2026-09-26). This supersedes 13.5's toast rule.

**Approach:**
- On `event:season_points_awarded` with `rankChange` `promoted` or `demoted`, store a pending rank change in a new store outside `matchStore`, and fire no toast.
- A `RankChangeGate` mounted in `AppLayout` next to `LevelUpGate` shows a forced `RankChangeDialog` once the player is back in the lobby or room. It follows `LevelUpDialog`'s pattern: no outside or Escape dismiss, no close button, and a Continue CTA that clears the store.
- The dialog shows a large tier badge, the rank label ("Gold 2"), the match's signed SP change, and, when the current-season query has loaded, the bar toward the next rank step.
- **Rank up** is celebratory like level-up: brass hairline, gold halo, title "Rank Up!".
- **Rank down** uses the same shell with no halo and a neutral accent, title "Rank Down". A demotion still never shows the rank-up celebration.
- If a level-up is also pending, the rank dialog waits until the level-up dialog is dismissed. The latest pending rank change wins.
- The two toast strings are removed.
- New strings, following the terminology reference (mk in Cyrillic, СП, no em-dash):

| key | en | mk | hr | sr |
|---|---|---|---|---|
| `season.rankUpDialog.title` | Rank Up! | Нов ранг! | Novi rang! | Novi rang! |
| `season.rankDownDialog.title` | Rank Down | Пад на рангот | Pad ranga | Pad ranga |
| `season.rankDialog.sp` | {{change}} SP this match | {{change}} СП во овој меч | {{change}} SP u ovom meču | {{change}} SP u ovom meču |
| `season.rankDialog.continue` | Continue | Продолжи | Nastavi | Nastavi |

</frozen-after-approval>

## Implementation Notes

- Files: new `client/src/shared/stores/rankChangeStore.ts` (store + `rankChangeFromPayload`), `shared/components/RankChangeDialog.tsx`, `shared/components/RankChangeGate.tsx` (+ tests for all three); `useWsDispatch.ts` (toasts replaced by `setPending`; season-tier imports dropped); `AppLayout.tsx` (gate mounted after `LevelUpGate`); `TierBadge.tsx` (`lg` size, `glow` prop, `data-glow`); the four locale files (`season.tierUp` and `season.demoted` removed, the frozen keys added).
- The step bar reads `useCurrentSeasonQuery` (enabled only while a change is pending) and renders only when that read matches the pending change's SP total, tier and division, so a refetch still in flight never shows the previous match's bar.
- Level-up precedence is `open = pending && !levelUpPending`; the rank dialog opens as soon as the level-up one is dismissed.
- Verification: full client suite 150 files / 2214 tests; `make lint` exit 0.
- Baseline: this change starts from the uncommitted 13.4 + 13.5 tree; for review, diff against git tree `b8344f26638fb451dac0516699e0c8871232b930` (server/ + client/, written via a temporary index; no commit, real index untouched).
- Review fixes: the gate keeps the last shown change through the close animation (derived state); the title's `aria-label` carries the rank; the bar row is reserved by a silent placeholder; step values clamp like RankBanner; `authStore.logout` clears `rankChangeStore`; two stale "toast" comments corrected. The close-animation contract is tested in `RankChangeGate.closing.test.tsx` (mutation-checked: fails with the fix reverted), because jsdom unmounts the popup without its exit animation.
- Final verification: client 151 files / 2219 tests; `make lint` exit 0.

## Review Triage Log

Blind hunter, 12 findings:

| # | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|
| 1 | Closing demotion repaints as "Rank Up! · Iron" (null fallbacks during the exit animation) | medium | Confirmed: `clear()` nulls `pending` while the popup animates out; props fell back to `promoted`/Iron | patch |
| 2 | A change kept through a later "none" match keeps stale SP and never shows its bar | low | True only if the next match starts while the forced dialog is open; the fix needs merge logic | reject (rare; copy and bar degrade, rank stays right) |
| 2b | Inconsistent test fixture (Gold 2 then a "none" at Silver 2) | low | Confirmed | patch |
| 3 | "Latest wins" can announce a demotion back to the rank last seen | low | True, needs two unseen changes; the forced dialog sits between matches | reject |
| 4 | `rankChangeStore` survives logout | medium | Confirmed against `authStore.logout` | patch (levelUpStore's same gap deferred, pre-existing) |
| 5 | Screen readers never hear the new rank | medium | Confirmed: rank line is a plain div, badge aria-hidden | patch (title `aria-label`) |
| 6 | The step bar pops in and the centred card jumps | low | Confirmed: stale cache on remount, then refetch | patch (reserved row) |
| 7 | Stacking with DailyRewardDialog / level-up's exit animation | low | Daily reward stacks at most once a day; fix needs its state lifted to a store | reject |
| 8 | No test that the store outlives match end | low | Confirmed | patch |
| 9 | Unknown-tier coverage deleted | low | Confirmed | patch (moved to the gate test) |
| 10 | Step bar duplicated from RankBanner; values not clamped | low | Duplication true; negative values unreachable | patch (clamp only); extraction rejected |
| 11 | Two comments still describe the tier-up toast | low | Confirmed (`seasonTier.ts`, `useSeasonWindowWatch.ts`) | patch |
| 12 | Planning docs still require the toasts | medium | Confirmed (epics 13.5 ACs, UX spec, epic context) | defer (spec edits are the owner's call) |
