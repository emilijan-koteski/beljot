import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import {
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  seasonRankLabel,
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
};

/**
 * One prior-season row in a profile's archive, showing the stored final tier and division.
 * A single sr-only sentence carries every value; visible cells are aria-hidden.
 */
export function SeasonArchiveRow({ seasonName, sp, tier, division, gamesPlayed }: Props) {
  const { t } = useTranslation();

  const total = finiteOrZero(sp);
  const safeTier = normalizeSeasonTier(tier, total);
  const rankName = seasonRankLabel(t, safeTier, normalizeSeasonDivision(safeTier, division));
  const games = finiteOrZero(gamesPlayed);

  return (
    <li
      data-testid="season-archive-row"
      data-season={seasonName}
      data-tier={safeTier}
      className="border-border bg-surface-elevated flex items-center gap-2.5 rounded-md border px-2.5 py-1.5"
    >
      <span className="sr-only" data-testid="season-archive-row-summary">
        {t("season.archive.rowAria", {
          season: seasonName,
          tier: rankName,
          sp: total.toLocaleString(),
          games,
        })}
      </span>

      <TierBadge tier={safeTier} size="sm" data-testid="season-archive-tier-badge" />

      <span
        data-testid="season-archive-name"
        aria-hidden="true"
        className="text-ink min-w-0 flex-1 truncate text-sm font-medium tabular-nums"
      >
        {seasonName}
      </span>

      <span
        data-testid="season-archive-tier"
        aria-hidden="true"
        className="shrink-0 text-xs font-semibold"
        style={{ color: SEASON_TIER_COLOR[safeTier] }}
      >
        {rankName}
      </span>

      <span
        data-testid="season-archive-sp"
        aria-hidden="true"
        title={t("season.leaderboard.columns.sp")}
        className="text-ink w-20 shrink-0 text-right text-xs font-semibold tabular-nums"
      >
        {t("season.leaderboard.spValue", { sp: total.toLocaleString() })}
      </span>

      <span
        data-testid="season-archive-games"
        aria-hidden="true"
        title={t("season.leaderboard.columns.games")}
        className="text-ink-mute hidden w-10 shrink-0 text-right text-xs tabular-nums sm:inline"
      >
        {games.toLocaleString()}
      </span>
    </li>
  );
}
