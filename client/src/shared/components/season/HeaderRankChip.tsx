import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { useCurrentSeasonQuery } from "@/shared/hooks/queries/useCurrentSeason";
import { useSeasonWindowWatch } from "@/shared/hooks/useSeasonWindowWatch";
import {
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  seasonRankLabel,
  seasonSpOrZero,
} from "@/shared/lib/seasonTier";

/**
 * The header's seasonal rank indicator: tier badge plus rank name, badge only below `sm`.
 * It also hosts the app's only {@link useSeasonWindowWatch}.
 */
export function HeaderRankChip() {
  const { t } = useTranslation();
  const { data: season } = useCurrentSeasonQuery();
  useSeasonWindowWatch(season);

  if (!season) return null;

  const sp = seasonSpOrZero(season.sp);
  const tier = normalizeSeasonTier(season.rankTier, sp);
  const rankName = seasonRankLabel(t, tier, normalizeSeasonDivision(tier, season.rankDivision));

  return (
    <div className="flex shrink-0 items-center gap-1.5" data-testid="header-rank" data-tier={tier}>
      <span className="sr-only">{t("season.banner.rankAria", { tier: rankName })}</span>

      <TierBadge tier={tier} size="sm" data-testid="header-rank-badge" />

      <span
        aria-hidden="true"
        data-testid="header-rank-tier"
        className="hidden text-xs font-semibold whitespace-nowrap sm:inline"
        style={{ color: SEASON_TIER_COLOR[tier] }}
      >
        {rankName}
      </span>
    </div>
  );
}
