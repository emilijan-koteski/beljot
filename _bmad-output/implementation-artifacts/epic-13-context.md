# Epic 13 Context: Seasonal Rank & Leaderboard

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Give competitive players an active, recurring goal that is distinct from lifetime XP/level: every completed match awards Season Points (SP), SP accumulates into an 8-tier seasonal ladder (Iron through Grandmaster), and standings are visible both personally (rank banner in the lobby) and publicly (seasonal leaderboard). Seasons run on 3-month quarterly windows with a soft reset, so newcomers can compete fairly each quarter while a player's past seasons stay on record on their profile. This is the epic that makes "a rank next to your name" real and creates the quarterly re-climb loop that drives return visits.

Note on source coverage: the architecture and UX planning documents were never updated for the season system, so they carry no season-specific detail beyond the lobby layout and rank-banner placement captured below. Treat the epic definition and the notes here as authoritative for season behavior.

## Stories

- Story 13.1: Season Points (SP) & Tier Climb
- Story 13.2: Seasonal Leaderboard
- Story 13.3: Season Rollover & Prior-Season Archive

## Requirements & Constraints

- Players can see their current seasonal tier, derived purely from SP earned in the active quarterly season. Tier names and ordering are fixed: Iron, Bronze, Silver, Gold, Platinum, Diamond, Master, Grandmaster.
- Players can view a leaderboard of top SP earners for a season, and reach a full paginated view from primary navigation.
- Seasons are 3-month quarterly windows with a soft reset: everyone starts the next season at Iron with 0 SP. No decay, no merging or compression of prior SP. Prior-season records are preserved permanently.
- The profile archive lists only seasons in which the player actually played; zero-game seasons are omitted, and the whole archive section is hidden for players with no played seasons rather than shown as an empty state.
- SP calculation is server-side and server-authoritative. The client must never be able to influence SP, tier, or leaderboard position by sending unsanctioned messages.
- Abandoning a match earns 0 SP, consistent with how the project already treats abandonment for progression rewards.
- The seasonal system supersedes the retired ELO/ranked-queue design. Do not implement ELO, placement matches, scaled ELO abandonment penalties, or tier sub-divisions (I/II/III) - all were explicitly retired. Older planning prose and the product brief still describe them; that content is stale.
- Success is measured by seasonal engagement: a majority of participating players should carry a season through to its end, so tier progress must feel reachable and the season countdown must be visible.
- All new player-facing strings need translations for every supported locale (English, Serbian Latin, Macedonian, Croatian), including tier names, season labels, and leaderboard column headings.

## Technical Decisions

- Season state is persisted in Postgres via GORM with explicit versioned SQL migrations (golang-migrate). Two new tables: one describing each season's identity and window, one holding per-player per-season accumulation (SP, current tier, games played, games completed).
- Tier is a stored, derived value on the per-player season record, recomputed at match settlement rather than calculated on every read - this keeps leaderboard queries and profile reads cheap and makes tier-up detection a simple before/after comparison at settlement time.
- SP accrual belongs in the existing match-settlement path alongside the other end-of-match reward systems (XP, coin, honor), not in a separate pass. Abandonment paths must route to the zero-SP outcome.
- Read endpoints are REST JSON under the versioned, plural, kebab-case API namespace used across the project. The leaderboard endpoint takes a season selector and returns ordered rows carrying position, username, tier, SP, and games played; the full-page view is paginated and highlights the requesting player's own position.
- The leaderboard is explicitly pull-based: refreshed on page load or a periodic poll. Do not add a WebSocket push channel for standings.
- Season rollover is a scheduled server-side job running nightly, checking whether the active season's end timestamp has passed and creating the next quarter's row. It must be idempotent - a nightly cadence means it will run many times inside a single season and possibly more than once after a boundary.
- Rank display lives in the lobby feature area as a dedicated banner component; the leaderboard is a lobby panel plus a full page reached from the top nav.

## UX & Interaction Patterns

- The lobby uses the competitive/esports layout: fixed top nav with a Leaderboard tab, a rank banner card directly beneath the nav, then a body split between a narrow play-options column and a wider right-hand panel holding the seasonal leaderboard (default top 10).
- The rank banner reads left to right: tier badge, tier name, current SP, progress bar toward the next tier, days remaining in the season. The season countdown is deliberate urgency signaling, not decoration.
- Tier badges use tier-specific colors with a glow treatment (Iron grey through Bronze `#cd7f32`, Silver `#c0c0c0`, up to the accent color plus glow at the top tier). Rank names and season headers use the large display type scale.
- Tier-up is one of the platform's earned theatrical moments: a toast on the match-end transition. Routine SP gains stay quiet so promotions land harder.
- The leaderboard in the lobby is ambient motivation - visible on every visit without requiring navigation. Keep it scannable: position, username, tier badge, SP.
- The rank banner's older `unranked` / `placement` states from the design spec are obsolete under the SP model. A player with 0 SP is simply Iron with an empty progress bar.

## Cross-Story Dependencies

- Story 13.1 is the foundation: it creates the season schema, the SP accrual hook in match settlement, and the rank banner. Stories 13.2 and 13.3 both read the tables it introduces, so 13.1 lands first. (13.1 has already shipped, including the Master/Grandmaster naming for the top two tiers.)
- Story 13.2's leaderboard depends on 13.1's per-player season records and stored tier value.
- Story 13.3's rollover job depends on 13.1's season window fields; its profile archive depends on prior-season records having been accumulated by 13.1.
- Epic 9 dependency: SP accrual attaches to the same match-settlement pipeline as XP, coin settlement, and honor, and must follow the same abandonment semantics.
- Epic 11 dependency: the public profile was built to tolerate absent season data, with the prior-season archive section omitted from the DOM entirely when the season system was not yet live. Story 13.3 fills that section in, and the current seasonal rank becomes part of the public profile response.
