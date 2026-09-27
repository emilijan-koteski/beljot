# Epic 13 Context: Seasonal Rank & Leaderboard

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Make the seasonal ladder the real list of who is best. Season Points (SP) rise with wins and fall with losses. Each change is scaled by the score margin and by the opponents' strength. Players climb and drop through 8 tiers (Iron to Grandmaster), and Iron through Diamond each have divisions 1–3. Seasons are 3-month quarters with a soft reset, there is a seasonal leaderboard, and past seasons are archived on the profile. The original ladder (13.1) only went up, so a player could lose and still gain rank. Stories 13.4–13.5 replace it with a win/loss ladder. Matchmaking by rank stays rejected: rooms are gated only by honor and coin buy-in. The target release is 2026-10-01 00:00 UTC, the start of Q4.

## Stories

- Story 13.1: Season Points (SP) & Tier Climb
- Story 13.2: Seasonal Leaderboard
- Story 13.3: Season Rollover & Prior-Season Archive
- Story 13.4: Competitive SP Formula (Win/Loss Ladder)
- Story 13.5: Divisions, Demotion & SP Feedback

## Requirements & Constraints

- **Ladder.** Iron, Bronze, Silver, Gold, Platinum and Diamond each have divisions 1–3 (1 lowest, 3 highest), splitting the tier's SP band into equal thirds. Master and Grandmaster are single. Players can drop divisions and tiers freely. SP never goes below 0, and 0 SP is Iron 1.
- **Per-match SP.** Both teammates get the same change.
  - Winners gain `base_win × margin × 2·(1 − E_winner)` and losers lose `base_loss × margin × 2·E_loser`, rounded.
  - `margin = 0.5 + (winner pts − loser pts) ÷ target` (501 or 1001), clamped to 0.5–1.5. On surrender the winners' points count as the target; an instant win gives 1.5.
  - `E = 1 / (1 + 10^((opp avg SP − own avg SP) / S))`. Team average is the mean of its two seats' current-season SP, and a bot seat counts as the Gold 1 floor.
  - `base_win > base_loss` (placeholder +30 / −20), so an average player creeps up instead of being stuck in Iron.
  - A team that made at least one Capot gets a flat +5, once per match, added after scaling.
  - Removed: +50 for finishing, the flat +100 for a win, game points ÷ 10, and the all-seat +50 Capot / instant-win bonus.
- **Abandonment** (a seat's reconnect window expires):
  - The abandoner loses a fixed `2 × worst loss`, where the worst loss is `base_loss × 1.5 × 2` (placeholder −120).
  - Their teammate loses half of what a surrender at that moment would have cost, in every room type. Cancelling the loss would let a second account absorb losses for a main one.
  - The opponents get a normal win, scored as a surrender.
  - Every other human seat is scored by its team's result, connected or not. `games_completed` keeps its presence meaning.
- **Bots.** Matches with bots count for rank, including Quick Play auto-fill. Bot seats never gain or lose SP.
- **Leaderboard membership** is `games_played ≥ 1` this season, including players at 0 SP. It applies to the page, the total count and the viewer's position.
- **Seasons.** Everyone restarts each quarter at Iron with 0 SP, with no decay and no carry-over. The archive omits zero-game seasons, and the section is hidden if the player has none.
- **Tuning** is an offline command that is never shipped.
  - It replays stored 2026 Q3 matches in completion order and reports each player's final SP, tier and division, and win rate overall and on bot-only tables.
  - Targets: ~85% against bots settles at the Grandmaster floor (roughly 50% → Gold, 60% → Platinum, 70% → Diamond, 80% → Master). A 50% player reaches Silver within ~20 matches from Iron 0.
  - The reduction for beating weaker teams must go well below 0.5×, or anyone beating bots more than two thirds of the time climbs forever.
  - Instant wins are inferred from a 0–0 score with no hands.
  - Fallback: ship the derived values plus placeholders and retune after a few weeks of Q4. The owner approves the constants before release.
- **Cutover.** Deploy on or after 2026-10-01 00:00 UTC, never before, or the end of Q3 is scored with the new formula. The migration clears any Q4 `player_seasons` rows. Q3 and earlier keep their old tier with no division.
- **Server authority.** SP, tier and division are computed on the server only.
- **i18n.** Strings are needed in en, mk, hr and sr. mk is fully Cyrillic, with SP written as СП. Tier names come from the season-tier terminology table, and the division is a numeral after the name („Злато 2"). No em-dashes outside en; the i18n parity test must pass.
- **Out of scope:** division shields, excluding bot matches, season carry-over, a per-match SP audit trail, persisting the instant-win flag, SP for matches cut short by a server restart (unchanged), and placement matches or ranked queues.

## Technical Decisions

- **SP is computed in the season service's award transaction, not in `match`.** It reads each seated human's current-season row (missing = 0 SP) under the existing ascending-user-ID lock, then computes the deltas, clamps at 0 and writes. This avoids a read-then-write race.
- **`match` builds a per-match outcome**: seat teams, winner, bot seats, final scores, match target, surrender and instant-win flags, Capot teams, and the abandoning seat. `match` never imports `season`; `season` implements `match`'s awarder interface.
- **Non-negative SP.** The DB `CHECK (sp >= 0)` stays alongside the engine clamp. A first-ever row written after a loss must be 0, not negative.
- **Stored snapshot.** Tier and division are stored on `player_seasons` with every award. The division is null for Master, Grandmaster and pre-Q4 seasons. The archive reads the snapshot and never re-derives a tier from SP.
- **Tier table.** The server table is the single source of truth. The client copy is display-only and changes in the same commit, the same convention as the XP level table.
- **Division everywhere.** Every shape that carries a tier also carries the division: the current season, leaderboard rows and viewer row, profile `seasonRank`, and archive rows.
- **`event:season_points_awarded`** carries the signed SP change, the new SP, tier, division, rank change (`promoted` / `demoted` / `none`) and reason (`normal` / `abandoned` / `partner_abandoned`).
  - The Go payload, golden fixture, TS type and zod schema change together.
  - Old tabs drop the new shape because the zod schema is strict; this is acceptable at a release boundary.
- **Leaderboard** refresh stays pull-based (page load or poll), with no WS push.
- **Tuning command** lives under `server/cmd/`, excluded from the production image.

## UX & Interaction Patterns

- **Rank display.** Every rank reads as tier plus division ("Gold 2"): the RankBanner, header chip, tier badges, leaderboard rows and profile season sections.
  - The progress bar fills to the next division. From Diamond 3 and Master it fills to the next tier. Grandmaster shows a terminal bar.
  - There are no unranked or placement states.
- **RankBanner** shows the tier badge (tier colour plus glow), the rank in display type, current SP, the progress bar and days left in the season. The countdown is deliberate urgency.
- **Rank changes.** Promotion gets the celebratory toast with the new tier and division. Demotion gets a subdued notice ("Dropped to Gold 2") with no fanfare and no alarm styling.
- **Match result** shows, next to coin settlement, the signed SP change (`+24 SP` / `−13 SP`) and the resulting tier and division.
  - A partner abandon shows "Partner abandoned: −12 SP (half loss)", and the player's own abandon states the penalty plainly.
  - A gain may use the accent colour; a loss is muted, never red.
- **Negative changes** display as negative. Client helpers that turn negatives into 0 must not hide them.
- **Leaderboard.** The lobby's right panel shows the top 10. The top-nav tab opens the full paginated page with the viewer's row highlighted.
- **Stale journeys.** Older ones showing a Level 5 gate, a ranked queue, placement matches or a rank-reveal screen are obsolete.

## Cross-Story Dependencies

- **13.1–13.3 are shipped.** Their schema, rollover job, leaderboard, archive and event are reused. Their text on the formula, flat tier thresholds, `sp > 0` membership and re-derived archive tier is superseded by 13.4–13.5.
- **Order.** The Q3 tuning replay comes first. Then 13.4 (engine, abandonment, membership, Q4-reset migration), then 13.5 (divisions, wire shapes, UI, i18n). Both ship in one release.
- **Epic 9.** SP stays in the match-settlement pipeline beside XP, coins and honor, which are unchanged. Honor still records abandonment on its own terms.
- **Epic 11.** The public profile's season rank and archive rows gain the division.
- **Tests.** About 30 server and client test files pin the old behaviour and must be updated. All suites must pass, including the WS golden-fixture and i18n parity tests.
