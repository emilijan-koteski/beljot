import { useTranslation } from "react-i18next";

import { SeasonRecord } from "@/shared/components/season/LeaderboardRow";
import { TierBadge } from "@/shared/components/season/TierBadge";
import {
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  seasonRankLabel,
  seasonWinRate,
} from "@/shared/lib/seasonTier";

/** Coerces a possibly absent server integer to a finite number, treating 0 as real. */
function finiteOrZero(value: number | null | undefined): number {
  return Number.isFinite(value) ? Math.max(0, value as number) : 0;
}

type Props = {
  seasonName: string;
  sp: number;
  tier: string;
  division: number | null;
  gamesPlayed: number;
  gamesWon: number;
};

/**
 * One prior-season row in a profile's archive: the stored final rank, SP, and the season's wins, losses and win rate.
 * A single sr-only sentence carries every value; visible cells are aria-hidden.
 */
export function SeasonArchiveRow({ seasonName, sp, tier, division, gamesPlayed, gamesWon }: Props) {
  const { t } = useTranslation();

  const total = finiteOrZero(sp);
  const safeTier = normalizeSeasonTier(tier, total);
  const rankName = seasonRankLabel(t, safeTier, normalizeSeasonDivision(safeTier, division));
  const games = finiteOrZero(gamesPlayed);
  const won = Math.min(finiteOrZero(gamesWon), games);

  return (
    <li
      data-testid="season-archive-row"
      data-season={seasonName}
      data-tier={safeTier}
      className="flex items-center gap-3 py-2.5"
    >
      <span className="sr-only" data-testid="season-archive-row-summary">
        {t("season.archive.rowAria", {
          season: seasonName,
          tier: rankName,
          sp: total.toLocaleString(),
          games,
        })}{" "}
        {t("season.leaderboard.recordAria", { won, lost: games - won })}
      </span>

      <TierBadge tier={safeTier} size="sm" glow={false} data-testid="season-archive-tier-badge" />

      <span aria-hidden="true" className="flex min-w-0 flex-1 flex-col leading-tight">
        <span
          data-testid="season-archive-name"
          className="text-ink truncate text-sm font-semibold tabular-nums"
        >
          {seasonName}
        </span>
        <span
          data-testid="season-archive-tier"
          className="text-2xs font-semibold"
          style={{ color: SEASON_TIER_COLOR[safeTier] }}
        >
          {rankName}
        </span>
      </span>

      <span aria-hidden="true" className="flex shrink-0 flex-col items-end leading-tight">
        <span
          data-testid="season-archive-sp"
          className="text-ink text-sm font-bold whitespace-nowrap tabular-nums"
        >
          {total.toLocaleString()}{" "}
          <span className="text-ink-mute text-2xs font-semibold">
            {t("season.leaderboard.spUnit")}
          </span>
        </span>
        <span data-testid="season-archive-record" className="text-2xs whitespace-nowrap">
          <SeasonRecord won={won} played={games} />
          <span className="text-ink-mute"> · {seasonWinRate(won, games)}%</span>
        </span>
      </span>
    </li>
  );
}
