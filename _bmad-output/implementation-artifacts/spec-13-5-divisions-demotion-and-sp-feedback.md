---
title: 'Story 13.5: Divisions, Demotion & SP Feedback'
type: 'feature'
created: '2026-09-26'
status: 'done'
baseline_commit: 'a290a7c8294ce081443d77a490c4bde50902e766'
route: 'dispatch'
review_loop_iteration: 0
context: ['{project-root}/_bmad-output/project-context.md', '{project-root}/_bmad-output/implementation-artifacts/epic-13-context.md', '{project-root}/_bmad-output/implementation-artifacts/spec-13-4-competitive-sp-formula.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** After 13.4, SP rises and falls, but players see only a flat tier, a celebratory toast that can only mean "up", and no SP feedback at match end. Ended seasons also re-derive their tier from SP on the new floors, so Q3 players read as Grandmaster.

**Approach:** Put divisions (Iron–Diamond 1–3, Master/Grandmaster single) on every tier shape, stored with the tier snapshot. Make ended seasons read the stored snapshot. Extend `event:season_points_awarded` with the signed change, division, rank change and reason. Show a subdued demotion notice and an SP line on the match-end screens, in en/mk/hr/sr.

## Boundaries & Constraints

**Always:**
- `tier.go` stays the single source: divisions are thirds of a tier's band (`Ladder.Rank`); the next rank step is the next division, or the next tier from Diamond 3 and Master; Grandmaster is terminal. `seasonTier.ts` mirrors it display-only.
- Migration 000028 adds nullable `player_seasons.rank_division` (1–3); every award writes it with `rank_tier` (NULL for Master/Grandmaster). Pre-Q4 rows stay NULL.
- ACTIVE season reads derive tier and division from SP (live table). ENDED season reads (`seasons.ends_at <= now`: archive, and leaderboard rows plus viewer for `?season=<id>`) use the stored `rank_tier` and `rank_division`, never `TierForSP`.
- Division on the wire is a number 1–3 or `null`: `rankDivision` on the current season and the event, `division` wherever the sibling field is `tier`.
- Event payload becomes `{spChange, newSeasonSp, rankTier, rankDivision, rankChange, reason, seasonName}`: `spChange` is the signed applied change; `rankChange` ∈ `promoted|demoted|none`, comparing (tier, division) before and after; `reason` ∈ `normal|abandoned|partner_abandoned` (abandoned seat / its teammate / everyone else). `spEarned` and `tieredUp` are removed. The Go payload, golden, TS type and strict zod schema change together.
- Promotion (division or tier) keeps the existing success toast, with the rank label. Demotion shows `toast.info`, never the success toast.
- SP line: on `MatchResult` (natural end) and on `ReconnectOverlay` (abandonment), from a `seasonSettlement` held in `matchStore` (reset on match_end and match_abandoned, like coins and honor). A gain uses the accent colour, a loss a muted one, never red. A signed change is never passed through `seasonSpOrZero`, `seasonBarFill` or `finiteOrZero`; totals still are.
- Rank label everywhere (header chip, RankBanner, SeasonSection, leaderboard rows and viewer, archive rows, toasts): the `season.rank` key "{{tier}} {{division}}" when a division exists, else the tier name alone. The banner bar fills toward the next rank step.
- New strings (mk Cyrillic, no em-dash outside en; hr/sr use noun phrases where "you" would be gendered):

| key | en | mk | hr | sr |
|---|---|---|---|---|
| `season.rank` | {{tier}} {{division}} | {{tier}} {{division}} | {{tier}} {{division}} | {{tier}} {{division}} |
| `season.demoted.toast` | Dropped to {{rank}} | Пад на {{rank}} | Pad na {{rank}} | Pad na {{rank}} |
| `season.result.line` | {{change}} SP · {{rank}} | {{change}} СП · {{rank}} | {{change}} SP · {{rank}} | {{change}} SP · {{rank}} |
| `season.result.partnerAbandoned` | Partner abandoned: {{change}} SP (half loss) | Партнерот го напушти мечот: {{change}} СП (половина од загубата) | Partner je napustio meč: {{change}} SP (pola gubitka) | Partner je napustio meč: {{change}} SP (pola gubitka) |
| `season.result.abandoned` | You abandoned: {{change}} SP | Го напушти мечот: {{change}} СП | Napuštanje meča: {{change}} SP | Napuštanje meča: {{change}} SP |

  `{{change}}` is "+24", "−13" (U+2212) or "0". en `season.banner.progressLabel` "into the next tier" becomes "to the next rank" (the others already say rank).

**Never:** no change to the 13.4 formula, constants or floors; no division shields; no new REST endpoint; no leaderboard WS push. `rankChange` compares ranks, never SP alone.

## I/O & Edge-Case Matrix

Floors 0/150/300/600/800/1000/1200/1400; Gold 1/2/3 start at 600/667/734.

| Scenario | Input / State | Expected |
|---|---|---|
| Division up | 650 → 690 | rankTier gold, rankDivision 2, rankChange promoted; success toast "Gold 2" |
| Tier down | 610 → 590 | silver 3, demoted; info toast "Dropped to Silver 3"; no success toast |
| Same division | 700 → 690 | gold 2, none; no toast |
| Master / Grandmaster | 1250; 1500 | division null; next step Grandmaster floor; Grandmaster terminal (bar full) |
| Partner abandoned | teammate of the abandoned seat, −12 | reason partner_abandoned; overlay "Partner abandoned: −12 SP (half loss)" |
| Loss at 0 SP | 0 → 0 | spChange 0, none; line "0 SP · Iron 1" |
| Ended Q3 row | stored rank_tier gold, rank_division NULL, sp 3500 | archive and `?season=Q3` show "Gold", no division, never "Grandmaster" |
| Active season | stored stale tier, sp 700 | derived gold 2 |

</frozen-after-approval>

## Code Map

- `server/internal/season/tier.go` -- `Ladder.Rank`/`RankForSP`, `Progress`, `HasDivisions`, `TierClimbed` (replace its use with a rank-change helper); add next-step progress.
- `server/migrations/000024:64` -- `rank_tier VARCHAR(16)`; its "never read the stored tier" comments (and `model.go:36-39,87-90,111-112`, `gorm_repo.go:121-123`, `service.go:150,231,339,396`, `handler.go:91-94,276-277`) must be rewritten for the ended-season rule.
- `server/internal/season/gorm_repo.go` -- `ApplySeasonPoints` INSERT/UPDATE (add `rank_division`); selects in `LeaderboardPage:303`, `FindLeaderboardEntry:334`, `PlayerSeasonArchive:406` (add `rank_tier`, `rank_division`). Scan structs `LeaderboardEntry`, `ArchiveEntry`, `PlayerSeason` in `model.go`.
- `server/internal/season/service.go` -- `ApplySeasonPoints` maps snapshots (walk `outcome.Seats` for the reason); `CurrentSeasonView:151` (`TierProgress`); `LeaderboardView` has `window` (:197), `viewerPosition` needs it; `ArchiveView:358`; `CurrentSeasonRank:397`.
- `server/internal/season/handler.go` -- views `CurrentSeasonView:26`, `LeaderboardRowView:95`, `LeaderboardViewerView:115`, `ArchiveRowView:279`, `SeasonRankView:304` (flows to both profile DTOs via `user/handler.go:32,103,142`).
- `server/internal/match/live_match.go:141-150` `PlayerSeasonSnapshot`; `sp_award.go:181-187` builds the payload; `ws/events.go:145-173` payload; golden `ws/testdata/events/season_points_awarded.json` (regenerate `UPDATE_GOLDENS=1 go test ./internal/ws/ -run Contract`).
- Tests pinning shapes: `season/handler_test.go` wire keys `:356,:656,:1408`, `TestGetLeaderboard_EndedSeasonByIdRendersItsStandings:1295`, `TestApplySeasonPoints_PrecomputesTheSnapshot:569`, mock `:180,:202,:290`; `user/handler_test.go:1136,1171,1191,1216`; `match/sp_wiring_test.go` stub `:33-60`; `gorm_repo_test.go` archive `:947-1086`, raw INSERTs `:762,:971`.
- Client: `shared/lib/seasonTier.ts` (floors, `normalizeSeasonTier`, `seasonBarFill`, `seasonSpOrZero` clamps); `shared/types/apiTypes.ts` (`CurrentSeasonResponse:299`, `LeaderboardRow:350`, `LeaderboardViewer:377`, `SeasonArchiveEntry`, `SeasonRank:463`); `wsEvents.ts:296`, `wsEvents.schemas.ts:362` (`z.strictObject`), `wsEvents.contract.test.ts` (reads the Go golden); `hooks/useWsDispatch.ts:443-492` (toast via sonner; `toast.info` + `MOTION.TOAST_INFO` precedent `:230`); `stores/matchStore.ts:72-197`; `features/match/components/MatchResult.tsx:220-280` (coin/honor pill pattern), `ReconnectOverlay.tsx:152-245` (`abandon-result-line:232`), `MatchPage.tsx:255-258,2069-2095,2311-2320`.
- Client rank renderers: `shared/components/season/HeaderRankChip.tsx:37-42`, `features/profile/components/RankBanner.tsx:63-136`, `SeasonSection.tsx:77-114`, `shared/components/season/LeaderboardRow.tsx:21,82-95`, `SeasonArchiveRow.tsx:15,40-44`, `features/leaderboard/LeaderboardPage.tsx:208,227`. i18n: `season` block line ~1390 in all four files, keys `season.tier.<token>`, `season.tierUp.toast`, `season.banner.*`; parity `i18n.test.ts:124`, `i18n.parity.test.ts`.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/season/tier.go` (+test) -- `RankProgress(sp) (tier, division, spIntoStep, spForNextStep)` and `RankChange(prevSP, sp) string`.
- [x] `server/migrations/000028_add_rank_division_to_player_seasons.{up,down}.sql` -- nullable SMALLINT with CHECK 1–3; down drops it.
- [x] `server/internal/season/{model,repository,gorm_repo,service,handler}.go` (+tests) -- write the division; select the stored snapshot; active-derive / ended-stored split; views gain the division; current season exposes `spIntoDivision`/`spForNextDivision` in place of the tier-band pair; snapshot gains division, rank change, reason.
- [x] `server/internal/match/{live_match,sp_award}.go`, `server/internal/ws/events.go` + golden (+tests) -- snapshot and payload per the frozen shape; `ws` constants for rank change and reason.
- [x] `server/internal/user/handler_test.go` -- `seasonRank` gains `division`.
- [x] `client/src/shared/lib/seasonTier.ts` (+test) -- division mirror, next-step progress, rank-label and signed-change helpers.
- [x] `client/src/shared/types/{apiTypes,wsEvents,wsEvents.schemas}.ts` (+contract test) -- new fields and payload.
- [x] `client/src/shared/stores/matchStore.ts`, `hooks/useWsDispatch.ts` (+tests) -- `seasonSettlement`; promoted → success toast, demoted → info toast; resets.
- [x] `client/src/features/match/components/{MatchResult,ReconnectOverlay}.tsx`, `MatchPage.tsx` (+tests) -- SP line with the reason variants.
- [x] Rank renderers above (+tests) -- rank label, bar toward the next step, archive from stored tier/division.
- [x] `client/src/shared/i18n/{en,mk,hr,sr}.json` -- the frozen strings; parity green.

**Acceptance Criteria:**
- Given a mix of Q3 and Q4 rows, when the archive and both seasons' leaderboards load, then Q3 shows its stored tier with no division and Q4 shows tier plus division.
- Given a demotion event, when it is dispatched, then an info toast shows and `toast.success` is never called.
- Given `make lint` and `make test` (integration on `beljot_e13`), when they run, then both pass, including the ws contract and i18n parity tests.

## Implementation Notes

- Environment: same as 13.4 (URLs in `/tmp/claude-1000/-home-emilijan-projects-beljot/b97fb269-6cd9-4087-9c90-7b36e2b67ee1/scratchpad/dburls.env`; apply 000028 to `beljot_e13` with `mise which migrate`; never touch `beljot`). Edit locale JSON with file-editing tools, not shell pipes.
- Resolves the 13.4 deferred item "ended seasons re-derive their tier" (deferred-work.md).
- **Review baseline.** 13.4 is uncommitted in the same tree, so `baseline_commit` (HEAD) includes it. The tree right before 13.5 is git tree `45ef3e0f67fc2667cf4bdd0ab6d301f940e79e70` (server/ + client/, untracked included, written via a temporary index; no commit, real index untouched). Diff 13.5 alone against that tree.

## Spec Change Log

## Review Triage Log

Pass 1 (2026-09-26): blind-hunter (B), edge-case-hunter (E), verification-gap (V).

| # | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|
| B1 | An early deploy (before 2026-10-01) makes every later Q3 award overwrite Q3's stored `rank_tier`/`rank_division` with new-floor values, which ended-season reads then serve (e.g. 3500 SP → grandmaster) | high | Confirmed: `ApplySeasonPoints` writes `RankForSP(next)` into the season covering `now`; ended reads return that snapshot | defer → owner decision (extends 13.4's E2; a runtime guard "no SP for seasons that started before 2026-10-01" is proposed to the owner; not reverted, root cause is release timing) |
| B7 | The abandoner never sees their penalty (no socket); the teammate's line shows only inside the 3 s overlay window | medium | Confirmed; Design Notes already state it; no durable per-match SP record exists | defer (pre-existing socket model; a durable carrier belongs with the deferred per-match SP audit trail) |
| B8 | mk `season.result.abandoned` „Го напушти мечот" reads as 2sg or 3sg („he left") | low | Confirmed: the aorist is identical for ти/тој; hr/sr use the noun phrase | owner decision (frozen string); proposed „Напуштање на мечот: {{change}} СП" |
| E1 | `partner_abandoned` with an applied change ≥ 0 (0 floor, or Capot +5 over a small half loss) reads "Partner abandoned: +3 SP (half loss)" in accent colour | medium | Confirmed: partner change = round(lossRaw/2) + capot can be > 0; `SeasonSpLine` picks the copy on reason alone | patch |
| V1 | SeasonSection's sr-only current-rank summary is not pinned; reverting it to the bare tier passes every test | medium | Pre-verified mutation (157 tests green) | patch |
| B10 | `TestHandleMatchEnd_SingleTiersCarryANullDivision` asserts inside an unchecked branch | low | Confirmed: no `found`/`require` | patch |
| B12 | SP line mounts after the result screen with no live region | low | Confirmed; `MatchResult` has no status role | patch (`role="status"` on the line) |
| B5 / V-other | Client `seasonRankProgress` + `divisionStart` have no production caller; a second copy of the step arithmetic | low | Confirmed (only `seasonTier.test.ts`) | patch (delete + fix header) |
| B6 / E5 | `TierForSP` / `TierProgress` have no production caller; `tier.go` header claims paths that no longer use them | low | Confirmed by grep | patch (delete if test-only; correct the header) |
| B2c | `ws/events.go` says a stale tab loses "only the toast"; it also skips the `season.current` invalidation | low | Confirmed: dispatcher returns before invalidating on a rejected frame | patch (comment) |
| B3n | Now that ended seasons show stored tokens, a future tier-token rename needs a `rank_tier` data migration; nothing says so | low | Confirmed: `tier.go`/000028 silent | patch (comment in `tier.go`) |
| B3 / B4 / E2 / E4 | Unknown stored tier token falls back to an SP bucket (old totals → Grandmaster) and keeps a division from another tier | low | Unreachable today: production Q3 rows hold only the eight live tokens (`beljot_q3`); fix touches seven call sites | reject |
| B2 / E3 | A stale bundle reads the renamed `spForNextTier` as 0 and shows "Top of the ladder" until refresh | low | True, transient, release-boundary degradation the epic accepts | reject (release note: ask players to refresh) |
| B9 | DB CHECK does not tie division to tier | low | Only `RankForSP` writes it | reject |
| B11 | No-division is `int` 0 in the season snapshot, `*int` elsewhere; `RankForSP(0)` in the loop | low | True; refactor only | reject |
| V-other2 | Default test DSN points at the shared `beljot` DB (migration 26), so an env-less run fails | false | Pre-existing (13.3) and fails loudly; CI migrates first | reject |

## Design Notes

The abandoner has no socket when the event is sent (that is why their window expired), so the `abandoned` line is supported but in practice only teammates and opponents see a line. Both land in `ReconnectOverlay`'s 3 s window, which reads the store live, so the line can arrive after the overlay mounts.

A rank step is the unit the bar fills: Gold 2 (667–733) fills 0→67 SP toward Gold 3; Diamond 3 fills toward the Master floor; Master fills toward Grandmaster; Grandmaster is full.

## Verification

**Commands:**
- `cd server && BELJOT_DB_URL=$BELJOT_E13_DB_URL go test -count=1 ./...` -- expected: pass; season integration tests RUN.
- `cd client && npx vitest run` -- expected: pass, including `wsEvents.contract.test.ts` and both i18n tests.
- `make lint` -- expected: exit 0.
