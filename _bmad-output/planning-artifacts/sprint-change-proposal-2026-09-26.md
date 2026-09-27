# Sprint Change Proposal — 2026-09-26

**Change:** Replace the Season Points ladder, where finishing a match always added SP, with a win/loss competitive ladder that has divisions and demotion.
**Trigger:** Owner observation in live play. A player lost a match and still went up in rank.
**Input:** [forged-idea.md](../forge/win-loss-rank-ladder/forged-idea.md) (forge session 2026-09-26, outcome HARDENED).
**Mode:** Batch.
**Branch / worktree:** `feat/E13-win-loss-rank-ladder` at `.claude/worktrees/E13-win-loss-rank-ladder`.
**Status:** Approved by owner 2026-09-26. Sections A–F applied to the artifacts on this branch (F included as recommended), plus two same-class wording fixes found by the post-edit sweep (`architecture.md` "progression/ELO engine" → "progression/SP engine"; `ux-design-specification.md` Progress row "Rank LP progress bar" → "Rank SP progress bar (to next division)"). Nothing committed. Implementation (`bmad-build`) runs in a separate session.

---

## 1. Issue Summary

### Problem statement

The seasonal ladder is meant to be the real list of who is best, but it measures how much someone plays, not how often they win:

- Every finished match adds SP, including a loss (+50 for finishing, plus team game points ÷ 10).
- Nobody can drop a rank.
- The game-points term is the same number XP is built from, so SP looks like a second XP bar.

### How it was found

The owner saw a player lose a match and gain rank. Tracing the code confirmed this is exactly what the current design does.

### Root cause: a misreading, plus one new requirement

- **The misreading.** sprint-change-proposal-2026-04-18 rejected **matchmaking by rank**: the Level 5 gate, the ranked queue and placement matches. Rooms were already gated by honor and coin buy-in. Story 13.1 read that proposal as also rejecting **losing rank**, and built a ladder that only goes up.
  - The code says so directly. `server/internal/season/tier.go:21-26` cites the April proposal as the reason for "FLAT LADDER, NO SUB-RANKS… no LP and no ELO".
  - The April proposal's own text lists "scaled penalties" among the retired items, which is where the over-reach came from.
  - The owner confirmed on 2026-09-26 that dropping ranks was never meant to be removed. **This proposal explicitly reverses the flat-ladder part of 2026-04-18. The rejection of matchmaking by rank stands.**
- **The new requirement.** Divisions: Gold 1 / 2 / 3.

### Evidence

- `server/internal/match/sp_award.go:11-15, 122-159`: `computeSPAwards` has no negative term and clamps at 0.
- `server/migrations/000024_create_seasons_and_player_seasons.up.sql:52`: `sp BIGINT NOT NULL DEFAULT 0 CHECK (sp >= 0)`, with the comment "SP never decreases".
- `server/internal/season/service.go:98-102`: `TieredUp` fires on any change of tier, with the comment "can only ever be a climb".
- A worked example: a loser in a 501 match gets about +80 SP, so about 6 straight losses take a player from Iron to Bronze.

---

## 2. Impact Analysis

### Epic impact

| Epic | Impact |
|---|---|
| **Epic 13: Seasonal Rank & Leaderboard** (in progress; 13.1–13.3 merged to `master`) | **Direct.** The formula and the flat tier table are replaced. The rest stays: seasons and player_seasons persistence, rollover, leaderboard and archive. Two new stories are added (13.4, 13.5). No story is rolled back. |
| Epic 9 (Coins, XP, Honor) | None. Coins, XP and honor are unchanged. Honor still records abandonment on its own terms. |
| Epic 11 (Public profiles) | Indirect. The profile shows the season rank, which will now include the division. This is covered by Story 13.5. |
| Epic 14 (Social login), Epic 16 (Phase 5) | None. |

### Story impact

- **13.1 Season Points & Tier Climb** (done, merged): its formula acceptance criteria and its flat thresholds are **superseded** by 13.4 and 13.5. The schema acceptance criteria still hold. It is annotated rather than rewritten, so the record of what was delivered stays honest.
- **13.2 Seasonal Leaderboard** (done, merged): the membership rule changes from `sp > 0` to "played at least one match this season". Players who lose back down to 0 must stay on the ladder. This moves into 13.4.
- **13.3 Season Rollover & Prior-Season Archive** (done, merged): the archive must show each season's **stored** final tier. Today it re-derives the tier from SP using the current table, which would put Q3 players in the wrong tier once the table changes. This moves into 13.5.
- **New 13.4 Competitive SP Formula**: the server engine, abandonment rules, leaderboard membership, the tuning script and the Q4 reset.
- **New 13.5 Divisions, Demotion & SP Feedback**: the division ladder, wire and API fields, promotion and demotion feedback, the SP line at match end, and i18n.

### Artifact conflicts

| Artifact | Conflict |
|---|---|
| `prd.md` | The Phase 4 "Seasonal rank system" bullet (line 168) states the old formula. **Existing drift:** the FR list (FR34–FR38) and the executive summary still describe the ranked queue, placement matches and Elo penalties that were retired in April. The PRD FR list also stops at FR52, so `epics.md` is the requirements inventory of record. |
| `epics.md` | FR37 describes a flat 8-tier ladder. The Epic 13 overview and FRs-covered line need updating. There are no FRs yet for the SP formula or for abandonment in SP. |
| `architecture.md` | This doc was never updated for seasons. It still lists an "ELO engine" and maps progression to `internal/user/`, and it lists FR38 as a pending Elo formula. The new engine needs one recorded decision: **where** SP is computed (see below). |
| `ux-design-specification.md` | The RankBanner is specified with unranked and placement states and an "LP bar". There is no demotion treatment and no SP line at match end. |
| `sprint-status.yaml` | 13.4 and 13.5 are missing. **Existing drift:** 13-1, 13-2 and 13-3 show `review`, but their story files say `done` and the code is on `master`. |
| `deferred-work.md` | Needs three owner-deferred items plus the gaps found during analysis. |
| `epic-13-context.md` | Restates the old formula. Needs a note that it has been superseded. |

### Technical impact (from the code inventory)

1. **Opponent scaling needs every seat's current SP while the award is computed.** Today `computeSPAwards` runs in `match` before the database is touched, and `ApplySeasonPoints` adds the delta to each total without reading it first (`gorm_repo.go:129-191`). The calculation therefore moves **into the season service's award transaction**:
   - `match` builds a per-match outcome and still never imports `season`.
   - `season` reads the seated humans' current-season rows in the same transaction, which already locks users in ascending ID order. It then computes the deltas and writes them.
2. **Places that assume SP never goes down:**
   - the clamp in `sp_award.go:147-152`;
   - the first-row INSERT, which writes `award.SP` straight in as the total (`gorm_repo.go:157,164`);
   - `TieredUp`, which doesn't know the direction of a change (`service.go:102`);
   - "Monotonic" comments in `model.go`, `apiTypes.ts` and migration 000024.
   The DB `CHECK (sp >= 0)` **stays**. The engine clamps totals at 0 before writing.
3. **Leaderboard membership:** `player_seasons.sp > 0` (`gorm_repo.go:229`) is shared by the page, the total and `CountAhead`.
4. **The archive tier is re-derived** with `TierForSP(e.SP)` (`service.go:349`). It has to read the stored snapshot instead.
5. **Wire contract:** `SeasonPointsAwardedPayload` (`events.go:164-170`) is validated by a strict zod schema (`wsEvents.schemas.ts:362`). A stale browser tab will drop the new event, which is harmless. The golden fixture and the client contract test change together.
6. **Nothing at match end shows SP today.** `MatchResult.tsx` shows coins only. The only SP feedback is the tier-up toast in `useWsDispatch.ts:477-490`.
7. **Client clamps:** `seasonSpOrZero`, `seasonBarFill`, and the local `finiteOrZero` copies in `LeaderboardRow.tsx` and `SeasonArchiveRow.tsx` would show a negative `spEarned` as 0.
8. **Match target:** `game.matchTarget()` is unexported (`game/scoring.go:468-473`). `MatchMode` is available at both award call sites.
9. **Capot team:** `hand_results.capot_team` is stored and `HandResult.CapotTeam` is in memory. `capotOccurred` currently throws the team away.
10. **Instant win is not stored.** `GameState.WonByInstantWin` is `json:"-"` and the `Match` model has no column for it. The Q3 replay has to recognise an instant win from a 0–0 score with no hands.
11. **Duplicated types:** `SPAward` and `PlayerSeasonSnapshot` exist in both `match` and `season`, plus two test fakes. Every signature change touches four places.
12. **Boot reconcile** (server restart mid-match) awards no SP (`reconcile.go:93-107`). This is unchanged and out of scope.
13. **Tests pinning the old behaviour:** `sp_award_test.go`, `sp_wiring_test.go` (14 tests), `tier_test.go`, `gorm_repo_test.go`, `handler_test.go` (including `ZeroSPRowsAreNotOnTheLadder`), the ws golden fixture and contract tests, plus about 15 client test files, including `seasonTier.test.ts`, `useWsDispatch.test.ts`, `RankBanner.test.tsx` and `i18n.parity.test.ts`.

---

## 3. Recommended Approach

**Selected: Option 1, Direct Adjustment.** Add Stories 13.4 and 13.5 to Epic 13, annotate 13.1–13.3, and update the planning docs.

| Option | Verdict | Why |
|---|---|---|
| **1. Direct adjustment** | **Viable. Selected.** | The season infrastructure from 13.1–13.3 is sound (seasons, rows, rollover, leaderboard, archive, events). Only the formula, the tier table and the feedback UI change. |
| 2. Rollback | Not viable | Reverting 13.1–13.3 throws away working persistence and UI that the new ladder reuses. |
| 3. PRD / MVP review | Not needed | This is Phase 4 work. The MVP is unaffected, and the product goal ("rank = who is best") is reinforced, not changed. |

- **Effort:** Medium–High. Server engine refactor, tuning script, migration, the division ladder across server and client, UI in four places, i18n in four locales, and about 30 test files.
- **Risk:** Medium, mostly timeline. The target release is **2026-10-01** (Q4 start), 5 days away.
- **Timeline safety net:** the release clears Q4 standings. If it slips past Oct 1, Q4 runs the old formula until release, and the reset then wipes those points, so nothing old-formula survives. There's also no reason to ship before 00:00 UTC Oct 1: a release before the boundary would score the last hours of Q3 with the new formula.

---

## 4. Detailed Change Proposals

All edits below are applied to this branch on approval. Story text uses the repo's Given / When / Then style. **Constants marked *placeholder* are set by the Story 13.4 tuning step.**

### The formula (canonical statement, referenced by the edits below)

```
Per match, for every human seat (both teammates get the same change):

  winners:  + base_win  × margin × 2·(1 − E_winner)
  losers:   − base_loss × margin × 2·E_loser
            then + 5 for each team that made at least one Capot in the match (flat, once)
            rounded; season SP never drops below 0

  margin  = 0.5 + (winner points − loser points) ÷ match target (501 | 1001), clamped 0.5–1.5
            surrender: winners' points count as the target · instant win: 1.5
  E       = 1 / (1 + 10^((opponent avg SP − own avg SP) / S))      expected result, 0–1
            team avg SP = mean of both seats; each bot seat counts as the SP floor of Gold 1
  base_win > base_loss (placeholder +30 / −20), so an average player creeps up over a season

Abandonment (a seat's reconnect window expires):
  abandoner  − 2 × worst possible loss   (worst = base_loss × 1.5 × 2 → placeholder −120)
  teammate   ½ × the loss a surrender at that moment would have cost
  opponents  a normal win, scored as a surrender
  Presence no longer gates SP: every human seat is scored by its team's result, and only
  the abandoning seat gets the abandon penalty. games_completed keeps its presence meaning.
```

**Removed:** +50 for finishing, the flat +100 for winning, game points ÷ 10, and the +50 Capot / instant-win bonus for all four seats.

---

### A. Stories (`epics.md`)

#### A1. Epic 13 overview (both copies: epic list at line 366 and epic body at line 2568)

OLD:
> Players earn Season Points (SP) per match, climb an 8-tier seasonal ladder (Iron → Grandmaster) across 3-month quarterly seasons, and view a seasonal leaderboard. Prior seasons are archived on the profile (zero-game seasons skipped).

NEW:
> Players win and lose Season Points (SP) per match on a competitive seasonal ladder: 8 tiers (Iron → Grandmaster), with Iron through Diamond split into divisions 1–3, where players can climb and drop, across 3-month quarterly seasons. They can view a seasonal leaderboard. Prior seasons are archived on the profile (zero-game seasons skipped). *Reworked 2026-09-26 (sprint-change-proposal-2026-09-26): the original SP ladder only went up (13.1); Stories 13.4–13.5 replace it with a win/loss ladder.*

And the FRs-covered line under the epic list: `**FRs covered:** FR37, FR39, FR40` → `**FRs covered:** FR37, FR39, FR40, FR65, FR66`

*Rationale:* the epic definition is what future agents read first.

#### A2. Story 13.1: add a superseded note under its heading

NEW (inserted directly under `### Story 13.1: Season Points (SP) & Tier Climb`):
> > **Superseded in part (2026-09-26):** the SP formula AC and the flat 8-tier thresholds below describe what was delivered, and are replaced by Story 13.4 (formula) and Story 13.5 (divisions and demotion). The schema AC still holds. See sprint-change-proposal-2026-09-26.md.

*Rationale:* keeps an honest record of what was delivered without leaving a live AC that contradicts 13.4.

#### A3. Story 13.2: add a note

NEW (under `### Story 13.2: Seasonal Leaderboard`):
> > **Amended (2026-09-26):** leaderboard membership becomes "played ≥ 1 match this season" rather than "has any SP" (Story 13.4), so players who lose back down to 0 SP stay on the ladder.

#### A4. Story 13.3: add a note

NEW (under `### Story 13.3: Season Rollover & Prior-Season Archive`):
> > **Amended (2026-09-26):** the archive's "final tier" is the stored end-of-season snapshot, not re-derived from SP with the current tier table. Seasons before 2026 Q4 show a tier without a division (Story 13.5).

#### A5. NEW Story 13.4: Competitive SP Formula

(Inserted after Story 13.3.)

> ### Story 13.4: Competitive SP Formula (Win/Loss Ladder)
>
> As a competitive player,
> I want wins to raise my Season Points and losses to lower them, scaled by the score and by how strong the opponents were,
> So that the seasonal ladder shows who is actually best, not who played the most.
>
> **Acceptance Criteria:**
>
> **Given** a match ends naturally, at the target ("dosta"), by surrender, or by instant win
> **When** SP is calculated for each human seat
> **Then** winners gain `base_win × margin × 2·(1 − E_winner)` and losers lose `base_loss × margin × 2·E_loser`, rounded, with both teammates receiving the same change
> **And** `margin = 0.5 + (winner points − loser points) ÷ match target`, clamped to 0.5–1.5; on surrender the winners' points count as the target; on an instant win the margin is 1.5
> **And** `E = 1 / (1 + 10^((opponent avg SP − own avg SP) / S))`, where a team's average is the mean of its two seats' current-season SP and every bot seat counts as the SP floor of Gold 1
> **And** `base_win > base_loss`, so a player winning half their matches against equal opponents slowly gains SP
> **And** each team that made at least one Capot in the match gains a flat +5 SP, once per match, added after scaling
> **And** a player's season SP never drops below 0
> **And** the old terms are gone: the +50 for finishing, the flat +100 for winning, game points ÷ 10, and the +50 Capot / instant-win bonus for all seats
>
> **Given** a seat's reconnect window expires and the match is abandoned
> **When** SP is calculated
> **Then** the abandoning seat loses a fixed `2 × worst possible loss`, where the worst possible loss is `base_loss × 1.5 × 2`
> **And** the abandoner's teammate loses half of what a surrender at that moment would have cost them, in every room type
> **And** the opponents receive a normal win, scored as a surrender
> **And** every other human seat is scored by its team's result whether or not it was connected at that moment; `games_completed` keeps its presence meaning
>
> **Given** a match includes bot seats (including Quick Play auto-fill)
> **When** SP is calculated
> **Then** bot seats neither gain nor lose SP, the humans at the table are scored normally, and the match counts for rank
>
> **Given** the seasonal leaderboard (Story 13.2)
> **When** membership is evaluated for the page, the total count and the viewer's position
> **Then** a player is on the ladder if they have played at least one match this season, including at 0 SP
>
> **Given** the offline tuning command, which is not shipped in the production image
> **When** it replays every stored 2026 Q3 match in completion order through the new formula
> **Then** it reports each player's final SP, tier and division, their win rate overall, and their win rate on bot-only tables
> **And** the constants (`base_win`, `base_loss`, `S`, the Gold 1 floor, all tier floors) are chosen so that a sustained ~85% win rate on bot-only tables settles at the Grandmaster floor (roughly 50% → Gold, 60% → Platinum, 70% → Diamond, 80% → Master), and a player winning 50% reaches Silver within about 20 matches from Iron 0
> **And** instant wins are recognised from stored data (a 0–0 score and no hands), because the flag isn't persisted
> **And** if time runs out, the release ships the constants derived from the 85% target plus placeholders, to be retuned after a few weeks of Q4
>
> **Given** the release is deployed on or after 2026-10-01 00:00 UTC
> **When** its migration runs
> **Then** any 2026 Q4 `player_seasons` rows already present are cleared, so Q4 standings come only from the new formula
> **And** rows for 2026 Q3 and earlier are untouched
>
> **Technical notes:**
> - Move the calculation into the season service's award transaction. It reads every seated human's current-season row (a missing row means 0 SP) under the existing ascending-user-ID lock, computes the deltas, clamps totals at 0 and writes them.
> - `match` builds a per-match outcome: seat teams, winner, bot seats, final scores, match target, surrender flag, instant-win flag, teams that made a Capot, abandoning seat. `match` must still never import `season`.
> - Keep the DB `CHECK (sp >= 0)`. Fix the first-row INSERT so a first match that is a loss writes 0, not a negative total.
> - Export a match-target helper from `game` (today `matchTarget()` is unexported).
> - Merge or collapse the duplicated `SPAward` / `PlayerSeasonSnapshot` types if that's cheap. Every signature change touches both packages and both test fakes.
> - Tests to update are listed in sprint-change-proposal-2026-09-26.md §2, item 13.

#### A6. NEW Story 13.5: Divisions, Demotion & SP Feedback

(Inserted after Story 13.4.)

> ### Story 13.5: Divisions, Demotion & SP Feedback
>
> As a competitive player,
> I want to see my rank with its division, know when I move up or down, and see what each match did to my SP,
> So that I understand where I stand and why it changed.
>
> **Acceptance Criteria:**
>
> **Given** the season ladder
> **When** a player's SP is placed on it
> **Then** Iron, Bronze, Silver, Gold, Platinum and Diamond each have divisions 1–3, with 1 lowest and 3 highest, splitting that tier's SP band into three equal parts; Master and Grandmaster have no divisions
> **And** the server tier table stays the single source of truth; the client mirror is display-only and changes in the same commit
>
> **Given** any API or wire shape that carries a season tier (current season, leaderboard rows and viewer row, profile `seasonRank`, archive rows, `season_points_awarded`)
> **When** it is served
> **Then** it also carries the division: 1–3, or none for Master, Grandmaster and seasons before 2026 Q4
> **And** the division is stored with the tier snapshot in `player_seasons`
>
> **Given** a player views the prior-season archive
> **When** it renders
> **Then** each season shows its stored final tier and division, not a tier re-derived from SP with the current table
> **And** seasons before 2026 Q4 show their tier without a division
>
> **Given** a match ends and SP is applied
> **When** `event:season_points_awarded` is sent
> **Then** it carries the signed SP change, the new season SP, the tier, the division, the rank change (`promoted` / `demoted` / `none`) and the reason (`normal` / `abandoned` / `partner_abandoned`)
> **And** the Go payload, the golden fixture, the TS type and the zod schema change together
>
> **Given** a player's division or tier goes up
> **When** the event arrives
> **Then** the existing celebratory rank-up toast shows the new tier and division (e.g. "Gold 3")
>
> **Given** a player's division or tier goes down
> **When** the event arrives
> **Then** a subdued notice shows the new rank (e.g. "Dropped to Gold 2"), never the rank-up celebration
>
> **Given** the match result screen
> **When** it renders after a ranked match
> **Then** it shows the player's SP change (+ or −) and their resulting tier and division
> **And** if their partner abandoned, it says so with the reduced loss (e.g. "Partner abandoned: −12 SP (half loss)")
> **And** if the player abandoned, it shows the abandonment penalty
>
> **Given** the RankBanner, the header rank chip, tier badges, leaderboard rows and profile season sections
> **When** they render
> **Then** the rank reads as tier plus division (e.g. "Gold 2"), and the progress bar fills toward the next division (next tier at Diamond 3 / Master; terminal at Grandmaster)
> **And** a negative SP change is displayed as negative, not clamped to 0; totals are still never below 0
>
> **Given** the four locales (en, mk, hr, sr)
> **When** the new strings are added (division label, demotion notice, match-end SP line, partner-abandoned reason)
> **Then** they follow the localization terminology reference: mk in Cyrillic with the SP abbreviation as СП, tier names from the season-tier table, the division as a numeral after the tier name (e.g. „Злато 2"), no em-dashes outside en
> **And** the i18n parity test passes

#### A7. FR inventory (`epics.md` Requirements Inventory)

OLD (line 62):
> FR37: Players can view their current seasonal rank tier (8 tiers: Iron → Bronze → Silver → Gold → Platinum → Diamond → Master → Grandmaster) based on Season Points earned in the current quarterly season

NEW:
> FR37: Players can view their current seasonal rank (8 tiers: Iron → Bronze → Silver → Gold → Platinum → Diamond → Master → Grandmaster; Iron through Diamond each split into divisions 1–3, 3 highest; Master and Grandmaster single) based on their Season Points in the current quarterly season; rank rises and falls with SP

OLD (line 64):
> FR39: Players can view a seasonal leaderboard of top Season-Point earners

NEW:
> FR39: Players can view a seasonal leaderboard of players ranked by current Season Points

NEW (appended after FR64):
> FR65: The system changes each human player's Season Points after every match: winners gain and losers lose SP, scaled by the score margin relative to the match target and by the gap between the two teams' average SP (Elo-style expected result; bot seats count as a fixed SP at the Gold 1 floor); a team that makes a Capot gains a small flat bonus; season SP never drops below 0
> FR66: The system penalizes abandonment in SP: the abandoning player loses a fixed amount equal to twice the largest possible match loss; their teammate loses half of what a surrender at that moment would have cost; the opponents are scored as winning by surrender

FR coverage map: add `FR65: Epic 13 — Competitive SP formula` and `FR66: Epic 13 — SP abandonment penalties`.
Phase scoping line 175: `Phase 4: Seasonal rank + leaderboard (FR37, FR39, FR40)` → `Phase 4: Seasonal rank + leaderboard (FR37, FR39, FR40, FR65, FR66)`.

---

### B. PRD (`prd.md`)

#### B1. Phase 4 bullet (line 168). Required.

OLD:
> - **Seasonal rank system:** 8 tiers (Iron → Bronze → Silver → Gold → Platinum → Diamond → Master → Grandmaster) across 3-month quarterly seasons. Climb via Season Points (SP) earned per match: 50 (completion) + 100 (win) + floor(team_game_points / 10) + 50 (Capot or instant-win). Abandoners earn 0 SP. No decay. Season end → soft reset (all players start next season at Iron). Profile archives every played season; seasons with zero games are skipped on display.

NEW:
> - **Seasonal rank system:** 8 tiers (Iron → Bronze → Silver → Gold → Platinum → Diamond → Master → Grandmaster) across 3-month quarterly seasons; Iron through Diamond each have divisions 1–3 (3 highest), Master and Grandmaster are single. Season Points (SP) rise with wins and fall with losses: each match's change is scaled by the score margin relative to the match target and by the gap between the two teams' average SP (Elo-style expected result; bot seats count as a fixed SP at the Gold 1 floor), and a team that makes a Capot gains a small flat bonus. Rank follows SP in both directions, so players can drop a division or a whole tier. Abandoners lose a fixed penalty of twice the largest possible loss; their teammate takes half of a surrender-scored loss. SP never goes below 0. No matchmaking by rank; rooms stay gated by honor and coin buy-in. No decay. Season end → soft reset (all players start next season at Iron with 0 SP). Profile archives every played season; seasons with zero games are skipped on display. *(Reworked 2026-09-26, see sprint-change-proposal-2026-09-26.md.)*

*MVP impact:* none (Phase 4).

---

### C. Architecture (`architecture.md`)

#### C1. Requirements table (line 36)

OLD: `| Player Progression      | FR33–FR40 | —             | FR33–FR40       | XP/level system, ELO engine, rank tiers, seasons               |`
NEW: `| Player Progression      | FR33–FR40, FR65–FR66 | —  | FR33–FR40, FR65–FR66 | XP/level system, SP engine (win/loss, Elo-style expected result), rank tiers with divisions, seasons |`

#### C2. Requirements-to-structure mapping (line 887)

OLD: `| Progression (FR33–FR40)       | `internal/user/` (extended Phase 2) | `features/lobby/RankBanner.tsx` (Phase 2)        |`
NEW: `| Progression (FR33–FR40, FR65–FR66) | `internal/user/` (XP, honor), `internal/season/` (SP engine, tiers, seasons), `internal/match/sp_award.go` (per-match outcome) | `features/profile/components/RankBanner.tsx`, `shared/lib/seasonTier.ts`, `shared/components/season/` |`

#### C3. Pending-specification list (lines 1003-1004)

OLD:
> - **FR38:** "Scaled ELO penalties by game progress" — scaling formula not specified (brief suggests x0.5 early to x2.0 late but PRD doesn't include it)

NEW:
> - ~~**FR38:** "Scaled ELO penalties by game progress"~~ — resolved: FR38 retired (2026-04-18); SP abandonment penalties are specified by FR66 (2026-09-26).

#### C4. NEW subsection "Seasonal Rank (SP) Engine"

Inserted after `### Backend Architecture`, before `### Infrastructure & Deployment`.

> ### Seasonal Rank (SP) Engine
>
> Added 2026-09-26 (sprint-change-proposal-2026-09-26). Formula: FR65/FR66 and Story 13.4.
>
> | Decision | Choice | Rationale |
> |---|---|---|
> | Where SP is computed | Inside `season.Service`'s award transaction, not in `match` | Opponent scaling needs every seated human's current-season SP. The transaction already locks users in ascending ID order; reading the rows there avoids a read-then-write race. |
> | Package boundary | `match` builds a per-match outcome (teams, winner, bots, scores, target, surrender, instant win, Capot teams, abandoning seat); `match` never imports `season` | Keeps the existing dependency direction (`season` implements `match.SPAwarder`). |
> | Non-negative SP | DB `CHECK (sp >= 0)` kept; the engine clamps totals at 0 before writing | SP can now fall; the floor is enforced in both places. |
> | Tier + division storage | `player_seasons.rank_tier` plus a division (1–3, null for Master/Grandmaster and seasons before 2026 Q4), written with every award | The archive reads the stored snapshot, so retuning the table never rewrites past seasons. |
> | Tier table | `season/tier.go` is the single source of truth; `client/src/shared/lib/seasonTier.ts` is a display-only mirror changed in the same commit | Same manual-sync convention as `level.go` ↔ `xpLevel.ts`. |
> | Leaderboard membership | `games_played ≥ 1` in the season | Players at 0 SP stay on the ladder. |
> | Wire event | `event:season_points_awarded` carries signed SP change, new SP, tier, division, rank change and reason; Go payload, golden fixture, TS type and zod schema change together | Stale tabs drop unknown shapes (strict schema); acceptable at a release boundary. |
> | Tuning | Offline command under `server/cmd/` replays stored matches; not in the production image | Constants are data-driven (Q3 replay), not guessed. |

*Note:* the rest of `architecture.md` (Elo matchmaking wording, `internal/session/` naming) predates the April reshape and the actual package layout. That broader refresh is out of scope here and is listed in deferred work.

---

### D. UX (`ux-design-specification.md`)

#### D1. Direction 5 characteristics (line 410)

OLD: `- Rank banner below the nav — player's current rank tier, LP progress bar, and season countdown visible immediately on entering the lobby`
NEW: `- Rank banner below the nav — player's current rank tier and division (e.g. "Gold 2"), SP progress bar toward the next division, and season countdown visible immediately on entering the lobby`

#### D2. Implementation approach (lines 425, 427)

OLD: `- Rank banner: card component beneath nav, `surface` background, rank badge left + LP bar + season countdown right`
NEW: `- Rank banner: card component beneath nav, `surface` background, rank badge left (tier + division) + SP bar to next division + season countdown right`

OLD: `- Leaderboard rows: rank number, username, tier label, ELO/LP value`
NEW: `- Leaderboard rows: rank number, username, tier label with division, SP value`

#### D3. RankBanner component (lines 753-761)

OLD:
> **Purpose:** Lobby element showing player's current rank, LP progress, and season countdown.
>
> **States:**
>
> - `unranked` — "Unranked" label, empty progress bar, prompt to play placement matches
> - `placement` — "Placement: X/3" during placement matches
> - `ranked` — full rank display with LP and progress bar

NEW:
> **Purpose:** Lobby element showing player's current rank (tier + division), SP, progress to the next division, and season countdown.
>
> **States:**
>
> - `ranked` — tier badge, tier name with division (e.g. "Gold 2"), current SP, progress bar to the next division (to the next tier from Diamond 3 / Master), days left in season. A player at 0 SP is Iron 1; there is no unranked or placement state.
> - `top` — Grandmaster: terminal bar, no "next" target.
>
> **Rank changes:** promotion (division or tier up) gets the celebratory toast. Demotion gets a subdued notice ("Dropped to Gold 2"), with no fanfare and no alarm styling. Earn the theatre: only climbing is theatrical.

#### D4. NEW component entry: match result SP line

Inserted after RankBanner.

> #### Match result: SP line
>
> **Purpose:** Tells the player what the match did to their seasonal rank.
>
> **Anatomy:** Signed SP change (`+24 SP` / `−13 SP`) and the resulting tier and division, shown with the coin settlement on the match result screen. If a partner abandoned: "Partner abandoned: −12 SP (half loss)". If the player abandoned: the penalty, stated plainly.
>
> **Visual tone:** Informational. A gain can use the accent colour; a loss uses a muted colour, never red alarm styling.

---

### E. Tracking and supporting artifacts

#### E1. `sprint-status.yaml`, Epic 13 block

OLD:
```yaml
  epic-13: in-progress
  13-1-season-points-and-tier-climb: review
  13-2-seasonal-leaderboard: review
  13-3-season-rollover-and-prior-season-archive: review
  epic-13-retrospective: optional
```
NEW:
```yaml
  epic-13: in-progress
  13-1-season-points-and-tier-climb: done
  13-2-seasonal-leaderboard: done
  13-3-season-rollover-and-prior-season-archive: done
  # 13-4 / 13-5 added 2026-09-26 by sprint-change-proposal-2026-09-26: win/loss ladder
  # (SP losses, divisions, demotion) replaces 13.1's up-only formula. Target release 2026-10-01.
  13-4-competitive-sp-formula: backlog
  13-5-divisions-demotion-and-sp-feedback: backlog
  epic-13-retrospective: optional
```
*Rationale:* 13-1..13-3 are `done` in their story files and merged to `master`; `review` was stale. `last_updated` is bumped to 2026-09-26.

#### E2. `deferred-work.md`: new section appended

> ## Deferred from: correct-course win/loss rank ladder (2026-09-26)
>
> - **Division shields.** Protect a player from dropping a whole division (e.g. a grace match or two after promotion). Owner-deferred; today players can fall through divisions and tiers freely.
> - **Remove bot-filled matches from rank.** Bot matches count for now because the player base is small (~80 registered, ~10 active). Revisit when enough humans play.
> - **Carry part of last season into the next.** For example, keep the final tier, or start one or two tiers below it, instead of the hard restart at Iron 0.
> - **Per-match SP audit trail.** No per-match SP change is stored (only the running total in `player_seasons`), so "why did I lose X SP?" can't be answered after the fact.
> - **Persist the instant-win flag on the match record.** `GameState.WonByInstantWin` is not stored; the Q3 tuning replay has to infer instant wins from a 0–0 score with no hands.
> - **Boot-reconcile path awards no SP** (`reconcile.go:93-107`). A match cut short by a server restart awards nothing. Pre-existing; unchanged by this change.
> - **architecture.md broader refresh.** Elo-matchmaking wording, `internal/session/` naming and other text that predates the April reshape.

#### E3. `epic-13-context.md`: note at the top

NEW (under the title):
> > **Superseded in part (2026-09-26):** this context describes the original up-only SP ladder (13.1–13.3). The formula, flat tiers and leaderboard membership are replaced by Stories 13.4–13.5; see sprint-change-proposal-2026-09-26.md and `_bmad-output/forge/win-loss-rank-ladder/forged-idea.md`.

---

### F. Existing-drift cleanup (optional; recommended)

These contradict the product as it stands today, independent of this change. They're one-liners, so fixing them now stops a future agent from following stale text.

- **F1. `prd.md` FR34–FR38 (lines 372-376):** align them to `epics.md`. FR34 lifetime level with no gating; FR35, FR36 and FR38 marked retired as in `epics.md`; FR37 takes the new wording from A7. Add under the heading: "*`epics.md` Requirements Inventory is the FR list of record; this list is frozen at FR52 except for these alignments.*"
- **F2. `prd.md` executive summary (line 41):** "ELO-based ranked play, an 8-tier ranking system, quarterly seasons, and leaderboards" → "a win/loss seasonal ladder (8 tiers with divisions), quarterly seasons, and leaderboards".
- **F3. `prd.md` user success (line 63):** "placement matches, tier promotions, and seasonal resets create a compelling loop" → "division and tier promotions (and the risk of dropping), plus seasonal resets, create a compelling loop".
- **F4. `prd.md` Journey 2 (Marko) and Journey 5 Scenario A:** add one line at the top of each: "*Historical narrative (pre-2026-04-18): the Level 5 gate, ranked queue, placement matches and Elo penalties were retired; see FR37, FR65, FR66.*"
- **F5. `ux-design-specification.md` onboarding (lines 445, 455):** "Rank banner: Unranked · Level 0" → "Rank banner: Iron 1 · Level 0"; remove "Play 5 games to unlock Ranked mode".

---

## 5. Implementation Handoff

**Scope: Moderate.** It needs backlog changes (two new stories and doc updates), then developer implementation. It carries one architecture decision (where SP is computed), which is recorded in C4. No PM/Architect replan is needed.

| Role | Responsibility |
|---|---|
| **Owner / PO (Emilijan)** | Approve this proposal. Approve the tuned constants from the Q3 replay before release. Decide go / no-go for 2026-10-01. |
| **Developer (`bmad-build`)** | Implement 13.4, then 13.5, on `feat/E13-win-loss-rank-ladder`, with the Q3 tuning replay as the **first** task of 13.4. Input: this proposal plus `forged-idea.md`. No commits until the owner says so. |

**Sequencing:**
1. The Q3 tuning replay picks the constants.
2. 13.4: the engine, abandonment rules, leaderboard membership and the Q4-reset migration.
3. 13.5: divisions, the wire contract, and UI plus i18n.
4. Release on or after 2026-10-01 00:00 UTC.

**Success criteria:**
- A loss lowers SP. A player can drop a division and a whole tier.
- A player at 0 SP who loses stays at 0 and stays on the leaderboard.
- The Q3 replay shows a strong bot-only player (~85%) near the Grandmaster floor, and a 50% player reaching Silver within about 20 matches.
- A demotion never shows the rank-up celebration. The match result screen shows the signed SP change and the partner-abandoned reason.
- Q3 archive rows keep their original tier, with no division. Q4 starts clean.
- Server and client test suites pass, including the ws contract and i18n parity tests.

---

## Appendix: Change Navigation Checklist

| Item | Status | Note |
|---|---|---|
| 1.1 Triggering story | [x] | No failing story; owner observation. Root cause is 13.1's formula (d98da50c). |
| 1.2 Core problem | [x] | Misreading of 2026-04-18, plus a new requirement (divisions). |
| 1.3 Evidence | [x] | Code refs in §1; forge session. |
| 2.1 Current epic | [x] | Epic 13 can finish with two added stories. |
| 2.2 Epic-level changes | [x] | Epic 13 scope and FRs amended (A1, A7). |
| 2.3 Remaining epics | [x] | 9, 11, 14, 16: no change; 11's profile covered by 13.5. |
| 2.4 Obsolete / new epics | [N/A] | None. |
| 2.5 Order / priority | [x] | 13.4 → 13.5 before 2026-10-01; no other resequencing. |
| 3.1 PRD | [!] | B1 required; F1–F4 existing drift (optional). |
| 3.2 Architecture | [!] | C1–C4; engine placement decision recorded. |
| 3.3 UI/UX | [!] | D1–D4 (+F5). |
| 3.4 Other artifacts | [!] | sprint-status, deferred-work, epic-13-context, ws golden fixture, about 30 test files. |
| 4.1 Direct adjustment | Viable | Selected. Effort Medium–High, risk Medium (timeline). |
| 4.2 Rollback | Not viable | Infrastructure is reused. |
| 4.3 MVP review | Not viable / not needed | Phase 4; MVP unaffected. |
| 4.4 Path selected | [x] | Option 1. |
| 5.1–5.5 Proposal components | [x] | §1–§5. |
| 6.1–6.2 Checklist / accuracy | [x] | |
| 6.3 User approval | [x] | Approved 2026-09-26 ("yes"); `bmad-build` to run in a separate session. |
| 6.4 sprint-status update | [x] | E1 applied; YAML re-parsed clean. |
| 6.5 Handoff confirmed | [x] | Developer via `bmad-build` in a fresh session on this worktree/branch; owner approves tuned constants and go/no-go. |
