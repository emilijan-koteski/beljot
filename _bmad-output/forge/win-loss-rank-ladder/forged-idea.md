# Forged: win/loss rank ladder

HARDENED 2026-09-26. Input for `bmad-correct-course`.

Reverses the flat Season Points ladder from sprint-change-proposal-2026-04-18. That proposal rejected **matchmaking by rank**; dropping ranks was never meant to go. The Correct Course proposal must say this explicitly, because the April text lists "scaled penalties" as retired.

## Purpose
- Rank is the true list of who's best. Players who lose sit below players who win.
- No matchmaking by rank. Rooms are still gated only by honor and coin buy-in.

## Ladder
- Iron through Diamond: 3 divisions each, with 3 highest (Gold 1 < Gold 2 < Gold 3). Master and Grandmaster are single.
- Players can drop through divisions and whole ranks freely. SP never goes below 0.
- Everyone restarts at Iron 0 each quarter (unchanged).

## Formula
`SP change = base × margin × opponent`, plus +5 for a Capot. Winners gain it and losers lose it. Both teammates get the same change.
- **base**: wins pay more than losses cost (placeholder +30 / −20), so an average player creeps up.
- **margin**: 0.5 + (winner points − loser points) ÷ target (501 or 1001), capped to 0.5–1.5. Surrender is scored as if the winners reached the target. An instant win counts as 1.5.
- **opponent**: Elo-style expected result from the gap between the two teams' average SP. Each bot seat counts as a fixed SP value at the start of Gold.
- **Capot**: flat +5, only for the team that made it.
- **Removed**: +50 for finishing, flat +100 for winning, game points ÷ 10 (the term that made SP look like XP), +50 Capot bonus to all four seats.

## Abandonment
- **Abandoner** (the seat whose reconnect window ran out): a fixed loss of 2× the worst possible loss.
- **Teammate**: half of what a surrender at that moment would have cost them, in every room type. The match-end screen shows the reason.
- **Opponents**: a normal win, scored as a surrender.

## Bots
- Bot matches count for rank (the player base is small).
- Tune so that winning about 85% of matches against bots over time settles at the Grandmaster floor. Roughly: 50% → Gold, 60% → Platinum, 70% → Diamond, 80% → Master.

## Tuning
The first build step is an offline script (never shipped) that runs Q3 2026's stored matches through the new formula.
- **Targets**: 85% against bots settles at the Grandmaster floor; a player winning 50% reaches Silver within about 20 matches.
- **Curve steepness**: calculated from the 85% target and the bot value.
- **Division borders**: split evenly within each rank.
- **Constraint**: the reduction on wins against weaker teams must go well below 0.5×. Otherwise anyone who beats bots more than 2/3 of the time climbs forever.
- **Fallback**: ship the calculated values plus placeholders, and retune after a few weeks of Q4.

## Cutover
- Release on 2026-10-01, when Q4 starts at 00:00 UTC. The release resets Q4 SP to 0, which also covers a late deploy.
- Q3 and earlier stay archived under the old flat ranks, with no division.

## Deferred
- Shields that protect against dropping a whole division.
- Removing bot-filled matches from rank.
- Carrying part of last season's SP into the next one (keep the rank, or drop one or two ranks).

## Rejected
- **Zero-sum ladder starting at Iron 0**: the typical player gets stuck in Iron. **Starting mid-ladder**: players get a rank they didn't earn.
- **Linear opponent gap**: it can't make Grandmaster reachable through bots without rewarding sheer volume.
- **Cancelling the teammate's loss on abandon**: lets a second account take losses for a main account.
- **Full teammate loss in private rooms, half in Quick Play**: rejected in favour of one rule everywhere.
- **Showing each player's share of matches played with bots**: not wanted.
- **Replaying Q4, mapping old SP onto divisions, or waiting until 2027 Q1**: none needed with the boundary release plus reset.

## Weak points that survived
- Quick Play has no honor gate, so the teammate penalty teaches less there.
- The 0 floor quietly adds SP to the system.
- The bot value (start of Gold) was accepted with low conviction. It's one constant, to retune with data.

## Touches
- Server: `season/tier.go`, `match/sp_award.go`.
- Client: `seasonTier.ts`.
- WS payload: `SeasonPointsAwardedPayload` only has `TieredUp` today; it needs division and demotion.
- DB: the `player_seasons.rank_tier` token format.
- i18n: rank and division labels in mk/en/hr/sr.
- Planning docs: PRD FR37/FR40, Epic 13, story 13.1 (in review).
