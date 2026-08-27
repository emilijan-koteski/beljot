import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { normalizeSeasonTier, SEASON_TIER_COLOR } from "@/shared/lib/seasonTier";

/**
 * Coerce a possibly-absent server integer into a renderable one.
 *
 * `Number.isFinite`, never truthiness: 0 is a real Go value for both fields on
 * this row (a played season can carry 0 SP), and `undefined` arrives when a
 * client bundle is newer than the server. Deliberately a LOCAL copy rather than
 * `seasonSpOrZero`, for the reason LeaderboardRow documents: that helper is SP
 * ladder display math, and this row also sanitizes a MATCH COUNT.
 */
function finiteOrZero(value: number | null | undefined): number {
  return Number.isFinite(value) ? Math.max(0, value as number) : 0;
}

type Props = {
  /** The machine-stable "YYYY QN" token — rendered VERBATIM, never translated. */
  seasonName: string;
  sp: number;
  /** The server's raw tier token — normalized here, never trusted as-is. */
  tier: string;
  gamesPlayed: number;
};

/**
 * One prior-season row in a profile's archive (Story 13.3): season token, tier
 * badge + localized tier label, final SP, games played. NOT LeaderboardRow —
 * there is no position, no username and no self state here — but it copies that
 * row's a11y recipe exactly: ONE sr-only sentence carries every value, and
 * every visible cell (plus the badge) is `aria-hidden`, so nothing is announced
 * twice and the terse cells never read as number soup.
 *
 * The tier is the season's FINAL standing, derived server-side from the frozen
 * SP; `normalizeSeasonTier` is the version-skew guard, the same as everywhere
 * else a tier token crosses the wire.
 */
export function SeasonArchiveRow({ seasonName, sp, tier, gamesPlayed }: Props) {
  const { t } = useTranslation();

  const total = finiteOrZero(sp);
  const safeTier = normalizeSeasonTier(tier, total);
  const tierName = t(`season.tier.${safeTier}`);
  const games = finiteOrZero(gamesPlayed);

  return (
    <li
      data-testid="season-archive-row"
      data-season={seasonName}
      data-tier={safeTier}
      className="border-border bg-surface-elevated flex items-center gap-2.5 rounded-[10px] border px-2.5 py-1.5"
    >
      {/* The row's entire accessible name — everything below is aria-hidden. */}
      <span className="sr-only" data-testid="season-archive-row-summary">
        {t("season.archive.rowAria", {
          season: seasonName,
          tier: tierName,
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
        {tierName}
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
