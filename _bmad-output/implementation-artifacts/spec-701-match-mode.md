---
title: '701-point match mode'
type: 'feature'
created: '2026-10-25'
status: 'done'
baseline_commit: '2d704566df19ae411ef3250c11408d0dac384682'
route: 'dispatch'
review_loop_iteration: 0
context: ['{project-root}/.claude/rules/game-rules.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Rooms can only race to 501 or 1001. Players want a middle-length match.

**Approach:** Add `"701"` as a third match mode with a 701-point target, offered in the Create Room form between 501 and 1001. Quick Play stays 501 and the Create Room default stays 1001.

## Boundaries & Constraints

**Always:** 701 uses the same match-end, stop-at-target, tie and SP rules as the other modes; only the target differs. The client mirrors the server's mode-to-target mapping. New UI strings go into en, sr, hr and mk.

**Never:** Change Quick Play's mode or the create-room default. Add a migration (`match_mode` is free `VARCHAR(10)`). Add mode-specific reward scaling (coins, XP and SP already scale through buy-in, score and target). Touch landing copy or the rules "Race to 1001" headline.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Create 701 room | create room `matchMode: "701"` | Room created with mode 701 | N/A |
| Omitted mode | create room, no `matchMode` | Mode `"1001"` | N/A |
| Unknown mode | `matchMode: "999"` | Rejected | `ErrInvalidMatchMode` (unchanged) |
| Reaching 701 | 701 match, a team ends a hand at ≥701 | Match ends, same resolution as other modes | N/A |
| Season replay | finished 701 match | `OutcomeFor` returns `Target 701` | N/A |
| Quick Play | join/create | Mode `"501"` | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/room/handler.go:28` -- `validMatchModes` allowlist (read only at :362). Default :360 and Quick Play :3037 stay.
- `server/internal/game/scoring.go:264-270` -- `MatchTarget`: "501"→501, else 1001, so "701" would silently play to 1001. Feeds match end (:100), stop-at-target (:134) and SP (`internal/match/sp_award.go:59`).
- `server/internal/season/replay.go:60` -- `OutcomeFor` rejects targets other than 501/1001. `cmd/sptune` reuses it; leave sptune's 1001 modelling alone.
- `client/src/shared/types/matchTypes.ts:20` -- `MatchMode` union; API and match-state types derive from it.
- `client/src/features/room/CreateRoomModal.tsx` -- state :86, `matchModeOptions` :241, `PreviewCard` prop :813, match-mode `<Segmented>` :403-415 in a half-width `sm:grid-cols-2` cell.
- `client/src/shared/components/ui/segmented.tsx` -- `flex-1` buttons with a `dense` prop. Do not change it.
- `client/src/shared/lib/roomLabels.ts:14-22` -- per-mode branches in `modeLabel` / `modeOptionLabel`.
- `client/src/features/match/MatchPage.tsx:1126` -- `matchMode === "501" ? 501 : 1001`, feeding ScorePanel/ScoreReveal.
- `client/src/shared/i18n/*.json` -- `lobby.createRoomModal.matchMode{501,1001}`, `lobby.createRoomModal.matchModeHint`, `lobby.card.matchMode{501,1001}`.
- `client/src/features/rules/content/*.ts` (~:296-297) -- the "race to 501 instead" sentence.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/game/scoring.go` -- `MatchTarget` returns 701 for "701"; update the doc comment; add a table test for 501/701/1001/unknown.
- [x] `server/internal/room/handler.go` -- add `"701"` to `validMatchModes`; add a 701 create case next to the 501 case in `handler_test.go`.
- [x] `server/internal/season/replay.go` -- accept 701; add a 701 case to `replay_test.go`.
- [x] `server/internal/game/stop_at_target_test.go` -- add a 701 counterpart of `TestStopAtTargetRespectsThe501Target`.
- [x] `client/src/shared/types/matchTypes.ts` -- add `"701"`, plus an exported `matchTargetFor(mode: string): number` that mirrors `MatchTarget` (unknown → 1001).
- [x] `client/src/features/match/MatchPage.tsx` -- use `matchTargetFor`; add a 701 case near the 501 case in `MatchPage.test.tsx`.
- [x] `client/src/shared/lib/roomLabels.ts` -- add "701" branches; extend `roomLabels.test.ts`.
- [x] `client/src/features/room/CreateRoomModal.tsx` -- options `["501","701","1001"]`; type state and prop as `MatchMode`; fit three options (Design Notes); add a 701 test in `CreateRoomModal.test.tsx` and keep the 1001 default assertion.
- [x] `client/src/shared/i18n/{en,sr,hr,mk}.json` -- add `matchMode701` under both `lobby.createRoomModal` and `lobby.card`; rewrite `matchModeHint` to cover all three modes.
- [x] `client/src/features/rules/content/{en,sr,hr,mk}.ts` -- mention 701 next to 501 in the shorter-match sentence.

**Acceptance Criteria:**
- Given the Create Room form, when the player views Match mode, then 501, 701 and 1001 appear in that order with 1001 selected, with no wrapping or overflow at desktop and 375px widths.
- Given a 701 match in progress, when the score HUD or hand reveal shows, then the target reads 701.
- Given a 701 room in the lobby, when its card and room badge render, then they show the localized "701 pts".

## Implementation Notes

- Layout: `dense` alone still wrapped "501 points" in the half-width cell at desktop. The `sm:grid-cols-2` wrapper was removed, so Variant and Match mode are each full-width rows, like Timer style. At 375px "1001 points" still overflowed by about 0.4px, so the match-mode Segmented also gets `className="[&>button]:px-1.5"` (the child-selector idiom is already used in `RoomPage.tsx`). Screenshots are in the session scratchpad `screens/`.
- `OutcomeFor` checks `Atoi(mode) == game.MatchTarget(mode)` instead of keeping a second list of valid targets.
- Client tests need the pinned Node 22. Under Node 24, `MatchPage.test.tsx` fails with an undici AbortSignal error that is unrelated to this change.

## Spec Change Log

## Review Triage Log

Pass 1 (blind B1–B11, edge E1–E2, verification-gap V1):

| # | Finding | Verdict | Evidence | Route |
|---|---------|---------|----------|-------|
| B1 | `validMatchModes` and `MatchTarget` are separate lists; a mode in the allowlist but not in `MatchTarget` silently plays to 1001 | low | Real for a future mode added to only one list. This change updates both. Rare developer path, and the fix adds a cross-package consistency test or refactor | reject |
| B2 | `OutcomeFor` relies on the 1001 fallback and accepts "+1001"/"01001" | false | Accepting iff `Atoi(mode)==MatchTarget(mode)` means replay's `Target` always equals the target the engine raced to, which is the invariant replay needs. Non-canonical strings never reach storage because the create-room allowlist rejects them, and the pre-change check accepted them too | reject |
| B3 | Client has no single `MATCH_MODES` list; the options literal is unchecked against the union | low | Pre-existing structure; harm only when someone adds a fourth mode. The fix is a refactor, not a direct correction | reject |
| B4 | New client mirror of `MatchTarget` is not in CLAUDE.md's hand-mirrored constants list | low | `matchTargetFor` mirrors `game.MatchTarget` by hand; a server edit gets no prompt to update the client. The fix edits CLAUDE.md | defer |
| B5 | Per-locale roomLabels test passes on English fallback when a key is missing | false | `i18n.parity.test.ts:49` fails whenever a key is missing from any locale, so a missing `matchMode701` cannot ship green | reject |
| B6 | Match-mode Segmented keeps `dense` on a full-width row, so it is ~4px shorter than the Variant and Timer rows | low | `CreateRoomModal.tsx` passes `dense` (py-1.5), while sibling Segmenteds use py-2. Visible on every desktop open. Restoring py-2 through the existing child-selector className is a direct fix; dropping `dense` entirely risks wrapping near 360px | patch |
| B7 | Three labels wrap at a 320px viewport | low | At 320px each segment has ~60px for ~70px of text. UX spec minimum is 1280px desktop; 320px devices are rare, and the fix needs responsive variants | reject |
| B8 | No 701 test for the live SP path or for replay surrender/instant-win below 701 | false | `sp_award.go:59` passes the mode straight to `MatchTarget`, which `TestMatchTarget` pins for "701". Replay classifies with a generic `< target` (`replay.go:88-90`), already exercised at 501/1001 | reject |
| B9 | AC2/AC3 not covered at component level (room badge/card at 701, ScoreReveal target) | low | AC3: no test pins a localized 701 badge (same root as V1). ScoreReveal: refuted, because MatchPage passes one `matchTarget` const to both ScorePanel and ScoreReveal and the new MatchPage test pins it at 701 | patch (with V1) |
| B10 | sr labels say "bod" while sr generic and rules text say "poena" | low | Pre-existing: sr `matchMode501`/`1001` already use "bod"; 701 follows that pattern by design | defer |
| B11 | Match history does not show the match mode | low | Pre-existing; 501 and 1001 already made scores ambiguous. Adding UI is a new feature, and few users would notice | reject |
| E1 | Replay accepts "+1001"/"01001" | false | Same refutation as B2: the replay target equals the engine target, and the allowlist prevents storage | reject |
| E2 | roomLabels per-locale test passes on English fallback | false | Same refutation as B5 (parity test) | reject |
| V1 | No test asserts the exact localized 701 compact label; deleting the `modeLabel` "701" branch leaves CI green while sr/hr/mk show "701 bodova/poena/поени" | low | Pre-verified gap: `roomLabels.test.ts` checks only that the label contains "701" and not "matchMode" | patch |

## Design Notes

Copy: en "701 points" / "701 pts"; sr and hr "701 bod"; mk "701 поен". Like 501, 701 ends in 1, so the singular form is correct. Example hint (en): "501 for a quick match, 701 for a medium one, 1001 for the full race".

Fitting the options: keep the two-column layout and first add `dense` to the match-mode Segmented. If a screenshot still shows wrapping or clipping, move Match mode to its own full-width row. Do not switch to the compact `pts` labels.

## Verification

**Commands:**
- `make lint` -- expected: client and server clean.
- `make test` -- expected: all pass, including the i18n parity test.

**Manual checks:**
- Screenshot the Create Room modal at desktop and 375px widths: three options fit and 1001 is selected by default.
