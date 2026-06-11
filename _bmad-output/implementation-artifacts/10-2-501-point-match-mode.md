---
baseline_commit: b84d4a5a078b734b12f411722da07fbd654b4c92
---

# Story 10.2: 501-Point Match Mode

Status: done

<!-- Moved from Epic 12 (was Story 12.2) on 2026-06-11 via bmad-correct-course — see
     _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-11.md. ACs unchanged. -->

## Story

As a player,
I want to play shorter 501-point matches,
so that I can enjoy a quicker game when I don't have time for a full 1001 match.

## Acceptance Criteria

1. **Given** a room is configured with 501 match mode, **when** a game starts, **then** the match-end threshold is set to 501 points instead of 1001.
2. **Given** a team's score reaches or exceeds 501, **when** the match-end check runs, **then** the match ends with the same resolution rules as 1001 (higher score wins; tie goes to the taker's team).
3. **Given** a room is created, **when** the mode dropdown is configured, **then** both 1001 and 501 are available as options.
4. **Given** the ScorePanel displays during a 501 match, **when** the player views the HUD, **then** the target score context reflects 501 (not 1001).

## Tasks / Subtasks

- [x] Task 1 — Server: accept `"501"` at room creation (AC: 1, 3)
  - [x] 1.1 In `server/internal/room/handler.go:25`, change `validMatchModes = map[string]bool{"1001": true}` to also accept `"501"`. This is the ONLY server source change in the story.
  - [x] 1.2 In `server/internal/room/handler_test.go`, add a create-room test with `"matchMode":"501"` asserting the response and persisted room carry `MatchMode == "501"` (mirror the existing happy-path test at handler_test.go:386-401). Verify `TestCreateRoom_InvalidMatchMode` (line 607, mode `"999"`) still passes unchanged.
- [x] Task 2 — Server: 501-threshold tiebreaker tests in the rules engine (AC: 1, 2)
  - [x] 2.1 NO engine source changes. `scoring.go:137` already resolves the threshold via `matchTarget(state.MatchMode)` and `matchTarget` (scoring.go:274-279) already returns 501 for `"501"`. Two 501 tests already exist: `TestHandScoring_501MatchMode` (scoring_test.go:364) and `TestMatchEnd_501Mode` (scoring_test.go:467).
  - [x] 2.2 Add the two missing 501 analogs of the 1001 tiebreaker tests in `server/internal/game/scoring_test.go`, using the established pattern `gs := testfixtures.NewGameNearEnd(a, b); gs.MatchMode = "501"`:
    - Both teams exceed 501, higher score wins — mirror `TestMatchEnd_BothTeamsExceed1001_HigherScoreWins` (line 404). Fixture math: trick 8 awards +70 to team A, +92 to team B (team B is the taker, seat 1). `NewGameNearEnd(450, 420)` → A=520, B=512, both ≥ 501 → Team A wins.
    - Both teams exceed 501 with TIED score, taker's team wins — mirror `TestMatchEnd_BothTeamsExceed1001_TiedScore_ContractingTeamWins` (line 421). Need `a + 70 == b + 92`, i.e. `a − b = 22`, both finals ≥ 501: `NewGameNearEnd(500, 478)` → both 570 → Team B (taker) wins.
  - [x] 2.3 Add a "match continues below 501" guard test: `NewGameNearEnd(300, 200)` + `MatchMode = "501"` → finals 370/292 → expect `PhaseHandComplete` and `WinnerTeam == nil`.
- [x] Task 3 — Client: enable the 501 option in CreateRoomModal (AC: 3)
  - [x] 3.1 In `client/src/features/room/CreateRoomModal.tsx:116-123`, remove `disabled: true` from the 501 entry in `matchModeOptions`. Keep the option order (501 first, 1001 second) and keep `"1001"` as the default state (line 46). The summary panel (line 345) already labels both values — no change there.
  - [x] 3.2 Rewrite `lobby.createRoomModal.matchModeHint` in ALL FOUR locale files — the current value ("Only 1001 pts for now" / "Zasad samo 1001 poen" / "Засега само 1001 поен" / "Zasad samo 1001 bod") becomes false the moment the option is enabled. Suggested values (adjust for idiom, see i18n rules in Dev Notes): en "501 for a quicker match, 1001 for the full race", sr "501 za kraći meč, 1001 za punu trku", mk "501 за пократок меч, 1001 за целосна трка", hr "501 za kraći meč, 1001 za punu utrku".
  - [x] 3.3 In `client/src/features/room/CreateRoomModal.test.tsx`, add a test: select the 501 segment, submit, assert the create request body carries `matchMode: "501"` (existing tests at lines 82/103/119 only exercise `"1001"`).
- [x] Task 4 — Client: localized 501 labels on room surfaces (AC: 3)
  - [x] 4.1 Add `lobby.card.matchMode501` to ALL FOUR locale files, matching the style of the sibling `lobby.card.matchMode1001` ("1001 pts" / "1001 bod" / "1001 поен" / "1001 bod"): en "501 pts", sr "501 bod", mk "501 поен", hr "501 bod".
  - [x] 4.2 Consistency fix while in sr.json: `lobby.createRoomModal.matchMode501` is currently "501 poen" while its sibling `matchMode1001` is "1001 bod" — align to "501 bod" (501 ends in 1, singular "bod" is correct).
  - [x] 4.3 In `client/src/features/lobby/components/RoomCard.tsx:17-20`, add a `"501"` branch to `modeLabel` returning `t("lobby.card.matchMode501")` (today a 501 room would fall through to the unlocalized `"501 pts"` fallback).
  - [x] 4.4 In `client/src/features/room/RoomPage.tsx:52-54`, add `"501": "lobby.card.matchMode501"` to `matchModeKeys` (today line 599 would render the raw string `"501"`).
  - [x] 4.5 Tests: RoomCard test rendering a `matchMode: "501"` room asserts the localized label (not the fallback); optionally extend `RoomPage.locale.test.tsx` the same way.
- [x] Task 5 — Client: pass the real target to ScorePanel and ScoreReveal (AC: 4)
  - [x] 5.1 In `client/src/features/match/MatchPage.tsx`, derive `const matchTarget = matchState.matchMode === "501" ? 501 : 1001;` — explicit `===` comparison (project rule: no truthiness on Go-sourced fields), and the not-"501"-means-1001 fallback mirrors the server's `matchTarget()` semantics. `matchState.matchMode` is already delivered in every snapshot (`matchTypes.ts:101`, `wsEvents.schemas.ts:86`) — NO WebSocket contract change.
  - [x] 5.2 Pass `matchTarget={matchTarget}` to `<ScorePanel …>` (MatchPage.tsx:1302). The prop already exists with default 1001 (`ScorePanel.tsx:155`) and is already rendered by both the compact and expanded variants (lines 98, 266) — ScorePanel source needs NO change.
  - [x] 5.3 In `client/src/features/match/components/ScoreReveal.tsx`, add `matchTarget?: number` (default `1001`) to `ScoreRevealProps` (line 16) and replace the hardcoded `/ 1001` in the match-score brass strip (line 360) with the prop. Pass `matchTarget={matchTarget}` from MatchPage (line 1913).
  - [x] 5.4 Tests (co-located, `data-testid` selection, present-tense descriptions): ScorePanel renders "/ 501" when `matchTarget={501}`; ScoreReveal brass strip (`row-match-total`) shows "/ 501" when passed and keeps "/ 1001" by default; a MatchPage-level test that a snapshot with `matchMode: "501"` propagates to the panel is a plus.
- [x] Task 6 — Rules pages: stop asserting 1001 is the only target (flagged by sprint-change-proposal-2026-06-11.md §Technical Impact)
  - [x] 6.1 In each of `client/src/features/rules/content/{en,sr,mk,hr}.ts`, amend the match-end `note` (en.ts:208-210 and the same block ±1 line in the other three) with one added sentence saying casual rooms can shorten the race to 501 with identical rules. Keep 1001 as the headline number everywhere else (chapter 1 title/lede and the quick-facts "Race to 1001" stay as they are — 1001 remains the canonical mode).
  - [x] 6.2 Localization rules for the new sentence: mk all-Cyrillic and idiomatic (no Serbian carryover); never mix hr/sr forms (hr "utrka/bodova" vs sr "trka/poena" per each file's existing usage); em-dashes allowed ONLY in en; „quotes" in mk/hr/sr.
- [x] Task 7 — Verification (Definition of Done)
  - [x] 7.1 `make lint` clean; `make test` green (Go + Vitest). The four-locale parity test (`i18n.parity.test.ts`) fails the build if any new key is missing or empty in any locale — new keys land in all four files in the same commit.
  - [x] 7.2 Manual flow: create a 501 room → label shows on lobby RoomCard and the room-lobby header → start with 4 players → ScorePanel shows `… / 501` → finish a hand → ScoreReveal brass strip shows `/ 501` → a team crossing 501 ends the match with the normal match-end flow.

### Review Findings

<!-- Code review 2026-06-11 (adversarial: Blind Hunter, Edge Case Hunter, Acceptance Auditor). 16 raw findings → 5 patch, 5 defer (D137–D141 in deferred-work.md), 8 dismissed. -->

- [x] [Review][Patch] No test pins the exact score == 501 boundary (`>=` at scoring.go:138 — a `>` regression would pass the suite; existing tests land at 520/570/370) [server/internal/game/scoring_test.go]
- [x] [Review][Patch] Modal preview label namespace mismatch on the newly-reachable 501 branch — 1001 renders `lobby.card.matchMode1001` ("1001 pts") but 501 renders `lobby.createRoomModal.matchMode501` ("501 points"); align to `lobby.card.matchMode501` [client/src/features/room/CreateRoomModal.tsx:341]
- [x] [Review][Patch] `sprint-status.yaml` edited (backlog → review) but missing from the Dev Agent Record File List [_bmad-output/implementation-artifacts/10-2-501-point-match-mode.md]
- [x] [Review][Patch] No page-level test of the default-1001 target derivation in MatchPage (only the "501" path is asserted at page level; the `?: 1001` fallback is covered only at ScoreReveal unit level) [client/src/features/match/MatchPage.test.tsx]
- [x] [Review][Patch] No automated test that a room created with "501" propagates through `StartMatch` to `GameState.MatchMode` — the seam is pinned only with "1001" (manager_test.go:142, reconnect_test.go:567); 501 verified manually only [server/internal/match/manager_test.go]
- [x] [Review][Defer] Rules pages don't document the both-teams-cross-tied → taker-wins match tiebreaker that the engine implements [client/src/features/rules/content/{en,mk,hr,sr}.ts] — deferred, pre-existing (D137)
- [x] [Review][Defer] Mode-label mapping duplicated across three client sites with divergent fallbacks (`RoomCard.modeLabel` → "`${m} pts`", `RoomPage.matchModeKeys` → raw value, MatchPage ternary → 1001) [client/src/features/lobby/components/RoomCard.tsx:17; client/src/features/room/RoomPage.tsx:52; client/src/features/match/MatchPage.tsx:1180] — deferred, pre-existing (D138)
- [x] [Review][Defer] `matchMode` is stringly-typed end-to-end — no `"1001" | "501"` union on Room/MatchState types; typos compile cleanly into the divergent fallbacks [client/src/shared/types] — deferred, pre-existing (D139)
- [x] [Review][Defer] `profile.matchHistory.mode` i18n map lacks a "501" entry in all four locales — latent (no consumer yet); spec mandates leaving match history alone this story [client/src/shared/i18n/{en,mk,hr,sr}.json] — deferred, pre-existing (D140)
- [x] [Review][Defer] sr landing mock `r2meta` says "501 poen" while sr lobby labels standardized on "bod"; spec sanctioned only the analogous mk fix [client/src/shared/i18n/sr.json:812] — deferred, pre-existing (D141)

## Dev Notes

### Why this story is small — the plumbing already exists

The mode was built end-to-end during Epics 2–4 and only the last-mile gates are closed. The full chain, verified in code on 2026-06-11:

- DB/room: `room/model.go:24` persists `MatchMode` (`size:10, not null, default:1001`). No migration needed.
- Start: `room/handler.go:799,1542` pass `room.MatchMode` into `match.Manager.StartMatch` (`live_match.go:92`) → `game.NewGame(…, matchMode, …)` (`state.go:183-187`).
- Engine: `scoring.go:137` checks `state.TeamScores[…] >= matchTarget(state.MatchMode)`; `matchTarget` returns 501 for `"501"`, 1001 otherwise. The tiebreaker (`determineMatchWinner`) is mode-agnostic.
- Wire: `GameState.MatchMode` has `json:"matchMode"` (`state.go:64`); the strict zod snapshot schema already includes it (`wsEvents.schemas.ts:86`); `matchTypes.ts:101` types it; `matchStore.matchState.matchMode` carries it. Reconnect snapshots include it too (`reconnect.go:555-594`) — the target survives reconnection for free.
- Errors: `apperr.ErrInvalidMatchMode` already exists (`errors.go:62`). Do NOT add new error definitions.
- i18n: `matchMode501` option labels already translated in all four locales.

What is actually missing: the `validMatchModes` gate (one line), the `disabled: true` flag on the modal option, the `matchTarget` prop hand-off in MatchPage, the hardcoded `/ 1001` in ScoreReveal, two lobby label lookups, hint/label i18n values, and tests.

### Hard scope boundaries — do NOT touch

- **Quick Play stays 1001.** `room/handler.go:1618` hardcodes `MatchMode: "1001"` for auto-created Quick Play rooms, and `MatchmakingDiagram.tsx:85` shows a hardcoded 1001 badge. Both are by design (PRD: 1001 for competitive play; 501 is a casual-room option) — leave them.
- **No engine changes, no migrations, no WS contract changes, no new apperr entries.** Everything is in place (see chain above). If you think you need one of these, re-read the chain — something else is wrong.
- **No room-settings edit endpoint exists.** Match mode is set only at creation; do not invent a PATCH for it.
- **Match history does not display the mode** and that is fine — the server already returns `matchMode` in history rows (`user/handler.go:74`) but the client type doesn't consume it. The i18n block `profile.matchHistory.mode.1001` is currently orphaned (no consumer). Leave both alone; surfacing mode in history is not part of this story's ACs.
- **Landing-page copy already references 501 example rooms** (`landing.room.r2title` "Quick 501 — no timer") — no changes there. (Pre-existing nit, optional one-word fix only if already editing mk.json: mk `landing.room.r2meta` says "501 поени"; numbers ending in 1 take singular "поен".)
- One story = one branch = one PR. Suggested branch: `feat/E10-S2-501-match-mode`. If you find unrelated bugs, file them in deferred-work.md, don't fix them here.

### Architecture & project-context compliance (the rules that bite here)

- `"1001"`/`"501"` are string enum values everywhere (DB default `1001`, JSON `camelCase` field `matchMode`). Never convert to numbers on the wire; convert to a number only at the display/threshold boundary (`matchTarget` server-side, the derived `matchTarget` const client-side).
- Never use JS truthiness on Go-sourced fields — explicit `matchState.matchMode === "501"` comparison.
- Game components are purely presentational: the client derives the displayed target from server state; the match-end decision itself is exclusively server-side (`scoring.go`). Do not add any client-side "is the match over" logic.
- Engine tests go through `ApplyAction` effects only and use `testfixtures` factories — `NewGameNearEnd(a, b)` then set `gs.MatchMode = "501"` on the returned state (the established pattern at scoring_test.go:364-366; no raw `GameState{}` literals).
- Tie-at-501 inside a hand (taker fails on equal hand points) is already handled by `scoreHand` and is variant-specific interim behavior — out of scope here (deferred to Epic 12; see project-context "Tied-hand rule"). This story's "tie" AC is about the MATCH-END tiebreaker (both teams cross the target with equal totals → taker's team wins), which is mode-agnostic and already implemented.
- Middleware order, fetchClient, store partitioning: untouched by this story.

### Files to modify (UPDATE — read each before editing)

| File | Change |
| --- | --- |
| `server/internal/room/handler.go` | line 25: add `"501": true` |
| `server/internal/room/handler_test.go` | +1 create-room test with 501 |
| `server/internal/game/scoring_test.go` | +3 tests (2 tiebreaker analogs, 1 continues-below-501) |
| `client/src/features/room/CreateRoomModal.tsx` | remove `disabled: true` (line 120) |
| `client/src/features/room/CreateRoomModal.test.tsx` | +501 submit test |
| `client/src/features/match/MatchPage.tsx` | derive `matchTarget`; pass to ScorePanel (1302) + ScoreReveal (1913) |
| `client/src/features/match/components/ScoreReveal.tsx` | new `matchTarget` prop; replace `/ 1001` (line 360) |
| `client/src/features/match/components/ScoreReveal.test.tsx` | brass-strip target assertions |
| `client/src/features/match/components/ScorePanel.test.tsx` | +`matchTarget={501}` render test (source unchanged) |
| `client/src/features/lobby/components/RoomCard.tsx` | `modeLabel` 501 branch (line 18) |
| `client/src/features/lobby/components/RoomCard.test.tsx` | localized 501 label test |
| `client/src/features/room/RoomPage.tsx` | `matchModeKeys` 501 entry (line 52) |
| `client/src/shared/i18n/{en,sr,mk,hr}.json` | `matchModeHint` rewrite; new `lobby.card.matchMode501`; sr `matchMode501` "poen"→"bod" |
| `client/src/features/rules/content/{en,sr,mk,hr}.ts` | one sentence in the match-end note |

No NEW files are expected (all tests are additions to existing co-located test files).

### i18n requirements (from Story 10.1's review cycles — these WILL be reviewed hard)

- Every new/changed key must exist non-empty (trimmed) in ALL FOUR locales in the same commit — `i18n.parity.test.ts` fails the build otherwise.
- mk: all-Cyrillic, idiomatic Macedonian (10.1 went through three review rounds for Serbian carryover — don't repeat: e.g. "Испрати" not "Прати"). hr vs sr never mixed: hr ijekavica/"utrka"/"bod(ova)" per file's existing usage, sr ekavica/"trka". Em-dashes only in en; mk/hr/sr use commas/middots and „quotes".
- The localization terminology table (match = меч/meč in UI; banned word "contract" — phrase outcomes as "the taker's team"/"failed hand") governs i18n VALUES and rules prose. Code identifiers stay as they are.
- JSON files contain literal UTF-8 characters, never `\uXXXX` escapes.

### Previous story intelligence (10.1, done 2026-05-09)

- The parity test now also rejects whitespace-only values (`trim().length === 0`) — placeholder values won't survive CI.
- Translation wording is the most-reviewed part of any story touching locale files; propose natural phrasing, not literal English calques (the Task 3.2/4.1 suggestions above are starting points, not gospel).
- 10.1 fixed test/source drift caused by asserting literal UI strings — prefer `data-testid` and i18n-key-driven assertions in the new tests.
- Pre-existing lint debt noted by 10.1 (`LobbyStats.tsx` import-sort, `TimerRing.tsx` react-refresh warnings) may still exist — not yours to fix.

### Git intelligence (recent commits)

- `b84d4a5` (HEAD) applied the sprint reorg that produced this story — planning docs only.
- `0d63433` reworked `ScoreReveal.tsx` (countdown rings / `ButtonProgressBorder`) — the brass-strip block you're editing at line 360 was touched recently; rebase carefully if other match-UI work lands first, and keep the ring/`acknowledged` behavior untouched.
- Recent fix pattern: match-UI changes ship with co-located component tests in the same commit (`2d831ff`, `0d63433`) — follow that.
- Commit message scope: `feat(room): …` / `feat(match): …` / `test(rules): …` style, ≤72 chars description.

### Web research

Not applicable — no new libraries, no version-sensitive APIs. Entire story uses the existing pinned stack (Echo v4 / GORM / React 19 / Zustand / Vitest / testify).

### Testing standards summary

- Go: stdlib `testing` + testify; tests co-located (`_test.go`); engine behavior tested via `ApplyAction` effects with `testfixtures` factories; table-driven where a function gains multiple cases.
- Frontend: Vitest, co-located `.test.tsx`; `data-testid` selection (e.g. `row-match-total`, `match-mode-segmented`); present-tense descriptions (`it('renders 501 target …')`).
- Gates: `make lint` + `make test` both green before review; the four-locale parity test is part of the Vitest run.

### Project Structure Notes

- All edits land in existing domain locations (room handler, game tests, match/lobby/room features, shared i18n) — no new packages, folders, or naming decisions required.
- No conflicts with the unified project structure detected. The only cross-cutting file set is the four locale JSONs + four rules-content TS files, which this project treats as a single atomic update unit (Story 10.1 precedent).

### References

- Epics: `_bmad-output/planning-artifacts/epics.md` § "Story 10.2: 501-Point Match Mode" (lines 2009-2033); Epic 10 overview (lines 1977-1979)
- Move provenance + readiness evidence: `_bmad-output/planning-artifacts/sprint-change-proposal-2026-06-11.md` (Change 1, §1 codebase readiness, §2 Technical Impact — rules-content question delegated to this story)
- PRD: `_bmad-output/planning-artifacts/prd.md` FR15/FR16 (lines 339-344), Phase 2 bullet (line 155); Ana's 501 journey (lines 207-213)
- Engine: `server/internal/game/scoring.go:136-146` (match-end check), `scoring.go:274-279` (`matchTarget`); tiebreaker tests `server/internal/game/scoring_test.go:394-485`; fixture `server/internal/game/testfixtures/fixtures.go:508-535` (`NewGameNearEnd`). ⚠️ The fixture's doc-comment outcome paragraph is STALE — it claims seat 0 wins trick 8 with AS and the hand fails; in reality seat 3 must trump with 7H and wins (see `TestHandScoring_LastTrickBonus` comments, scoring_test.go:36-38), so the taker (team B) succeeds and scoring is A+=70 / B+=92. Trust the inline comments of the existing tiebreaker tests, not the fixture doc comment. (Optional drive-by: fix the stale fixture comment — comment-only change.)
- Project rules: `_bmad-output/project-context.md` (truthiness rule, match-target tiebreaker, testing rules, DoD checklist)
- Architecture: `_bmad-output/planning-artifacts/architecture.md` — journey-validity table note on 501 (line ~991); enum naming (line ~310). No architecture changes required (confirmed by the change proposal).

## Dev Agent Record

### Agent Model Used

Claude Fable 5 (claude-fable-5) via Claude Code

### Debug Log References

- Red-green cycle per task: each new test was run and confirmed failing before the corresponding source change (Task 1: `TestCreateRoom_501MatchMode` 400→201; Task 3: modal 501-submit test failed on disabled segment; Task 4: RoomCard/RoomPage mk-locale tests failed on fallback labels; Task 5: ScoreReveal "/ 501" test failed on hardcoded 1001).
- Task 2 tests passed immediately as designed — they are coverage of existing engine behavior (`matchTarget()` already returned 501); the story mandates no engine changes.
- Manual flow (2026-06-11, local dev stack): 501 room created through the real CreateRoomModal (501 segment enabled, new hint visible); lobby RoomCard showed "Bitola · 501 pts"; room-lobby header badge showed "501 pts"; full 4-player live match (browser as seat 0 + 3 WS bots) ran 6 hands; ScorePanel showed "x / 501" throughout; all 6 ScoreReveal brass strips showed "/ 501"; match ended exactly when team B crossed 501 (369–623, winner team B) with the normal MatchResult overlay — under 1001 rules that hand would NOT have ended the match, confirming the threshold drove the decision. Every WS snapshot carried `matchMode: "501"`.
- Harness note (not a product issue): the /tmp bot scripts listened for `event:error` but the server emits `error:*`-prefixed types, so their brute-force card retry never advanced — fixed in `/tmp/beljot-501-uibots-v3.mjs` (also adds reconnect-on-staleness self-healing around the known replaced-socket resync gap documented in score-reveal jam findings).

### Completion Notes List

- Server gate opened: `validMatchModes` now accepts `"501"` (`room/handler.go:25`) — the only server source change, exactly as scoped. New handler test asserts response + persisted room carry `"501"`; `TestCreateRoom_InvalidMatchMode` ("999") still passes unchanged.
- Engine untouched; added 3 scoring tests: both-exceed-501 higher-score-wins (`NewGameNearEnd(450, 420)` → 520/512, A wins), both-exceed-501 tied-score taker-wins (`NewGameNearEnd(500, 478)` → 570/570, B wins), and a continues-below-501 guard (370/292 → `PhaseHandComplete`, nil winner). Also fixed the stale `NewGameNearEnd` doc comment flagged in the story References (comment-only; seat 3 trumps with 7H, A+=70/B+=92).
- CreateRoomModal: 501 segment enabled (option order and "1001" default preserved); component doc comment updated (it claimed 501 was a disabled telegraph option); `matchModeHint` rewritten in all four locales since "Only 1001 pts for now" became false.
- Localized 501 labels: new `lobby.card.matchMode501` in all four locales; RoomCard `modeLabel` and RoomPage `matchModeKeys` now resolve `"501"` through i18n instead of falling back; sr `matchMode501` aligned "501 poen"→"501 bod"; sanctioned drive-by mk `landing.room.r2meta` "501 поени"→"501 поен" (singular after numbers ending in 1).
- Match HUD: `matchTarget` derived once in MatchPage via explicit `matchState.matchMode === "501"` comparison (no truthiness on Go-sourced fields), passed to ScorePanel (prop already existed) and to ScoreReveal via a new `matchTarget?: number = 1001` prop replacing the hardcoded `/ 1001` brass-strip text. No WS contract change; match-end decisions remain exclusively server-side.
- Rules pages: one sentence appended to the match-end note in all four content files — casual rooms can shorten the race to 501 with identical rules; 1001 kept as the headline number everywhere else. mk all-Cyrillic, hr "utrka/boda", sr "trka/poena", em-dash only in en.
- Out of scope honored: Quick Play stays hardcoded 1001; no migrations, no apperr additions, no room-settings PATCH, match history mode display untouched.
- Gates: `make lint` clean (ESLint + Prettier + golangci-lint); `make test` fully green — 769 Vitest tests across 75 files (includes the four-locale i18n parity test and rules-content parity test) + all Go packages.

### File List

- _bmad-output/implementation-artifacts/sprint-status.yaml — story status backlog → review
- server/internal/room/handler.go — accept "501" in validMatchModes
- server/internal/room/handler_test.go — TestCreateRoom_501MatchMode
- server/internal/game/scoring_test.go — 3 new 501 match-end tests; review patch: exact-501 boundary test
- server/internal/game/testfixtures/fixtures.go — NewGameNearEnd doc-comment fix (comment only)
- server/internal/match/manager_test.go — review patch: 501 StartMatch → GameState propagation test
- client/src/features/room/CreateRoomModal.tsx — enable 501 segment; doc-comment update; review patch: preview label namespace fix
- client/src/features/room/CreateRoomModal.test.tsx — 501 submit-payload test
- client/src/features/match/MatchPage.tsx — derive matchTarget; pass to ScorePanel + ScoreReveal
- client/src/features/match/MatchPage.test.tsx — 501 snapshot → score panel propagation test; review patch: default-1001 target test
- client/src/features/match/components/ScoreReveal.tsx — matchTarget prop; replace hardcoded / 1001
- client/src/features/match/components/ScoreReveal.test.tsx — brass-strip 501 + default-1001 tests
- client/src/features/match/components/ScorePanel.test.tsx — matchTarget={501} render test (source unchanged)
- client/src/features/lobby/components/RoomCard.tsx — modeLabel 501 branch
- client/src/features/lobby/components/RoomCard.test.tsx — mk-locale 501 label test
- client/src/features/room/RoomPage.tsx — matchModeKeys 501 entry
- client/src/features/room/RoomPage.locale.test.tsx — mk-locale 501 badge test
- client/src/shared/i18n/en.json — matchModeHint rewrite; lobby.card.matchMode501
- client/src/shared/i18n/sr.json — matchModeHint rewrite; lobby.card.matchMode501; matchMode501 poen→bod
- client/src/shared/i18n/mk.json — matchModeHint rewrite; lobby.card.matchMode501; landing r2meta singular fix
- client/src/shared/i18n/hr.json — matchModeHint rewrite; lobby.card.matchMode501
- client/src/features/rules/content/en.ts — 501 sentence in match-end note
- client/src/features/rules/content/sr.ts — 501 sentence in match-end note
- client/src/features/rules/content/mk.ts — 501 sentence in match-end note
- client/src/features/rules/content/hr.ts — 501 sentence in match-end note

## Change Log

- 2026-06-11: Implemented Story 10.2 (501-point match mode). Server accepts matchMode "501" at room creation; 501 option enabled in CreateRoomModal; localized 501 labels on lobby RoomCard and room-lobby header; ScorePanel/ScoreReveal display the real match target from the snapshot's matchMode; rules pages note the 501 casual option in all four locales; 3 engine threshold/tiebreaker tests + 7 frontend tests added. Verified with make lint + make test (all green) and a live 4-player 501 match driven to match-end (369–623 at hand 6).
