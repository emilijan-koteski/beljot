import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { useCurrentSeasonQuery } from "@/shared/hooks/queries/useCurrentSeason";
import { useSeasonWindowWatch } from "@/shared/hooks/useSeasonWindowWatch";
import { normalizeSeasonTier, SEASON_TIER_COLOR, seasonSpOrZero } from "@/shared/lib/seasonTier";

/**
 * The header's seasonal rank indicator: tier badge + tier name, and the badge
 * ALONE below `sm`.
 *
 * NO SP FIGURE, ANYWHERE. The header carries the identity of the rank, not its
 * arithmetic — the exact total, the band decomposition and the countdown all
 * live on the profile's RankBanner, which is the surface for reading progress.
 * A number here would compete with the two figures already in this row (level
 * and coin balance) for the row's scarcest resource, and on a phone there is no
 * room for a third at all — which is why even the tier NAME drops below `sm`.
 *
 * Bare rather than pilled, matching the `xp-indicator` it sits beside: TierBadge
 * already carries a ring, a tinted ground and a coloured glow, so wrapping it in
 * a second bordered pill (the coin/user idiom) would double the chrome on the
 * one item in this row that supplies its own.
 *
 * It also hosts {@link useSeasonWindowWatch} — the app's ONLY copy. This chip is
 * the one season surface that renders on every authed route, which makes it the
 * right owner for the rollover watch (see the hook's own header).
 */
export function HeaderRankChip() {
  const { t } = useTranslation();
  const { data: season } = useCurrentSeasonQuery();
  useSeasonWindowWatch(season);

  // Nothing until the query resolves. The hooks above still ran, so the
  // rollover watch is armed from the moment the first response lands.
  if (!season) return null;

  const sp = seasonSpOrZero(season.sp);
  // Guarded, not trusted: an unrecognised token from a newer server falls back
  // to the SP's own bucket rather than rendering a missing i18n key.
  const tier = normalizeSeasonTier(season.rankTier, sp);
  const tierName = t(`season.tier.${tier}`);

  return (
    <div className="flex shrink-0 items-center gap-1.5" data-testid="header-rank" data-tier={tier}>
      {/* The chip's whole accessible name. The badge is decorative and the
          visible name is hidden on phones, so without this the rank would be
          announced as nothing at all below `sm`. */}
      <span className="sr-only">{t("season.banner.rankAria", { tier: tierName })}</span>

      <TierBadge tier={tier} size="sm" data-testid="header-rank-badge" />

      <span
        aria-hidden="true"
        data-testid="header-rank-tier"
        className="hidden text-xs font-semibold whitespace-nowrap sm:inline"
        style={{ color: SEASON_TIER_COLOR[tier] }}
      >
        {tierName}
      </span>
    </div>
  );
}
