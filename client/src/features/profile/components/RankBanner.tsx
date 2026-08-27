import { useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import {
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  SEASON_TIER_LINE,
  seasonBarFill,
  seasonDaysRemaining,
  seasonSpOrZero,
} from "@/shared/lib/seasonTier";
import { getTimeTick, subscribeTimeTick } from "@/shared/lib/timeTick";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

type Props = {
  /** The viewer's season standing. `undefined` while the query is in flight. */
  season: CurrentSeasonResponse | undefined;
};

/**
 * The seasonal rank banner (Story 13.1 AC3). Renders exactly the five elements
 * the AC names: tier badge (tier colour + glow), tier name, current SP, a
 * progress bar to the next tier, and days remaining in the season.
 *
 * IT LIVES ON THE PROFILE, not the lobby it shipped in. The rank's IDENTITY is
 * always on screen now — the header's HeaderRankChip carries the badge and the
 * tier name on every authed route — so the lobby no longer needs a second, much
 * larger copy of the same fact above the room grid. What the chip deliberately
 * omits is the arithmetic (the SP total, the band decomposition, the countdown),
 * and this is the surface for reading that: the profile, where every other
 * progression figure already lives. It replaced the SeasonSection's small
 * current-rank chip rather than joining it — two rank readouts on one page is
 * the contradiction that chip's own section header warned about.
 *
 * PRESENTATIONAL ONLY, and now literally so. Every number arrives decided by the
 * server — the SP total, the tier token, and the spIntoTier / spForNextTier
 * decomposition. Nothing here computes a rank and nothing gates on one (no
 * feature in this product unlocks on tier or SP). The one non-visual job it used
 * to carry, the season-rollover watch, moved out with the move: it now hangs off
 * the header chip (see `useSeasonWindowWatch`), which is mounted everywhere this
 * page is and everywhere it is not. Do NOT re-add it here — the hook fires a
 * toast, and a second caller would double it.
 *
 * The one subscription that stays is the shared 30s tick, which is what ages the
 * days-remaining figure instead of freezing it at mount.
 *
 * There is NO unranked state and NO placement state. The older UX spec describes
 * unranked / "Placement: X/3" / LP states from the retired ELO model; a player at
 * 0 SP is IRON, a real tier, and renders normally (Story 13.1 D6).
 */
export function RankBanner({ season }: Props) {
  const { t } = useTranslation();
  // The shared 30s tick, so the countdown below ages without this component
  // owning an interval of its own.
  useSyncExternalStore(subscribeTimeTick, getTimeTick, getTimeTick);

  // Nothing to show until the query resolves. Deliberately null rather than a
  // skeleton: the banner sits in a single-column stack, so a placeholder would
  // shift the whole page down and then back up again on a fast response.
  if (!season) return null;

  const sp = seasonSpOrZero(season.sp);
  // Guarded, not trusted: an unrecognised token from a newer server falls back to
  // the SP's own bucket rather than rendering a missing i18n key.
  const tier = normalizeSeasonTier(season.rankTier, sp);
  const fill = seasonBarFill(season.spIntoTier, season.spForNextTier);
  const pct = Math.round(fill * 100);
  const atTop = seasonSpOrZero(season.spForNextTier) <= 0;
  const daysLeft = seasonDaysRemaining(season.endsAt, Date.now());

  const tierName = t(`season.tier.${tier}`);
  const color = SEASON_TIER_COLOR[tier];

  return (
    <section
      data-testid="rank-banner"
      data-tier={tier}
      className="border-border bg-surface mb-5 flex flex-wrap items-center gap-4 rounded-xl border px-4 py-3.5"
      style={{ borderColor: SEASON_TIER_LINE[tier] }}
    >
      {/* Tier badge — colour AND glow, per AC3. The glow is a coloured drop
          shadow off the same token, so it re-roots on felt with the ramp.
          Story 13.2 lifted the markup into TierBadge so the leaderboard's rows
          show the identical treatment; the test id is unchanged. */}
      <TierBadge tier={tier} size="md" data-testid="rank-badge" />

      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex flex-wrap items-baseline gap-x-2.5 gap-y-1">
          <span
            data-testid="rank-tier-name"
            className="font-display text-base font-semibold"
            style={{ color }}
          >
            {tierName}
          </span>
          <span data-testid="rank-sp" className="text-ink-dim text-xs tabular-nums">
            {t("season.banner.sp", { sp: sp.toLocaleString() })}
          </span>
          <span
            data-testid="rank-season-days"
            className="text-ink-mute ml-auto text-xs tabular-nums"
            /* The machine-stable window token is rendered VERBATIM — it is an
               identifier, not translated copy — and sits in the tooltip so the
               visible line stays a plain countdown. */
            title={season.seasonName}
          >
            {t("season.banner.daysLeft", { days: daysLeft })}
          </span>
        </div>

        {/* Progress to the next tier. A sibling of XpBar rather than XpBar
            itself: that component hardcodes the accent fill (`bg-accent`) and
            this one must take the tier's own colour. Same a11y contract. */}
        <div
          data-testid="rank-progress"
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={pct}
          aria-label={
            atTop
              ? t("season.banner.progressLabelTop", { tier: tierName, sp: sp.toLocaleString() })
              : t("season.banner.progressLabel", {
                  tier: tierName,
                  current: seasonSpOrZero(season.spIntoTier).toLocaleString(),
                  next: seasonSpOrZero(season.spForNextTier).toLocaleString(),
                })
          }
          className="bg-surface-sunken h-1.5 overflow-hidden rounded-full"
        >
          <div
            className="h-full rounded-full transition-[width] duration-500 ease-out"
            style={{ width: `${pct}%`, background: color }}
          />
        </div>

        <span
          data-testid="rank-progress-caption"
          className="text-ink-mute text-[11px] tabular-nums"
        >
          {atTop
            ? t("season.banner.atTop")
            : t("season.banner.progress", {
                current: seasonSpOrZero(season.spIntoTier).toLocaleString(),
                next: seasonSpOrZero(season.spForNextTier).toLocaleString(),
              })}
        </span>
      </div>
    </section>
  );
}
