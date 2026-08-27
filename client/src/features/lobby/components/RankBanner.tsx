import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { queryKeys } from "@/shared/api/queryKeys";
import { TierBadge } from "@/shared/components/season/TierBadge";
import { MOTION } from "@/shared/lib/motion";
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
 * How long the boundary effect waits before re-firing for a window it has
 * already acted on. Only reachable when the client's clock runs AHEAD of the
 * server's: the early refetch returns the same still-active season, and this is
 * what lets the real boundary still be caught instead of being swallowed by an
 * already-consumed guard.
 */
const SEASON_BOUNDARY_RETRY_MS = 5 * 60 * 1000;

/**
 * The lobby's seasonal rank banner (Story 13.1 AC3). Renders exactly the five
 * elements the AC names: tier badge (tier colour + glow), tier name, current SP,
 * a progress bar to the next tier, and days remaining in the season.
 *
 * PRESENTATIONAL ONLY. Every number arrives decided by the server — the SP total,
 * the tier token, and the spIntoTier / spForNextTier decomposition. Nothing here
 * computes a rank and nothing gates on one (no feature in this product unlocks on
 * tier or SP).
 *
 * There is NO unranked state and NO placement state. The older UX spec describes
 * unranked / "Placement: X/3" / LP states from the retired ELO model; a player at
 * 0 SP is IRON, a real tier, and renders normally (Story 13.1 D6).
 */
export function RankBanner({ season }: Props) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  // Subscribe to the single shared 30s tick so the days-remaining figure ages
  // without this component owning an interval of its own — and so the boundary
  // effect below re-evaluates as the countdown crosses zero.
  const tick = useSyncExternalStore(subscribeTimeTick, getTimeTick, getTimeTick);

  // THE BOUNDARY EFFECT (Story 13.3): `seasonDaysRemaining` floors at 0
  // forever, so without this a lobby tab left open across the quarter boundary
  // renders the dead season indefinitely — the deferred bug from 13.1. When
  // the shared tick observes endsAt in the past, invalidate `season.current`
  // (the banner's own query), every leaderboard entry (widget + full page) and
  // the seasons list (the picker's feed, which gains a window at every
  // boundary). Pull-only stays intact: this is client-side clock observation,
  // not a push.
  //
  // ONCE PER BOUNDARY, BUT NOT ONCE ONLY. The ref keys on the endsAt value, so
  // the refetched NEW season re-arms it for the next quarter while re-renders
  // and further ticks of the dead one do nothing. The `at` stamp is the
  // CLOCK-SKEW ESCAPE: if this client's clock runs ahead of the server, the
  // first firing refetches the SAME still-active window, and a ref consumed
  // forever would leave nothing to fire at the real boundary (season.current is
  // deliberately unpolled). Re-arming after RETRY_MS turns that dead end into a
  // slow retry that costs one refetch per five minutes of skew at most.
  const invalidatedForRef = useRef<{ endsAt: string; at: number } | null>(null);
  const endsAt = season?.endsAt;
  useEffect(() => {
    if (endsAt === undefined) return;
    const end = new Date(endsAt).getTime();
    const now = Date.now();
    if (!Number.isFinite(end) || now < end) return;
    const last = invalidatedForRef.current;
    if (last?.endsAt === endsAt && now - last.at < SEASON_BOUNDARY_RETRY_MS) return;
    invalidatedForRef.current = { endsAt, at: now };
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.current() });
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.leaderboardAll() });
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.list() });
  }, [tick, endsAt, queryClient]);

  // THE TRANSITION TOAST, once per observed season change: the invalidation
  // above refetches, the new window's name arrives, and the flip from a KNOWN
  // previous name fires exactly one toast. Initial mount sets the ref without
  // toasting (loading a season is not a transition), and the same plumbing the
  // tier-up toast uses (sonner + MOTION duration) keeps the two moments
  // consistent. The season token is rendered verbatim inside localized copy.
  const prevSeasonNameRef = useRef<string | null>(null);
  const seasonName = season?.seasonName;
  useEffect(() => {
    if (seasonName === undefined) return;
    const prev = prevSeasonNameRef.current;
    prevSeasonNameRef.current = seasonName;
    if (prev !== null && prev !== seasonName) {
      toast.success(t("season.banner.newSeason", { season: seasonName }), {
        duration: MOTION.TOAST_LONG,
      });
    }
  }, [seasonName, t]);

  // Nothing to show until the query resolves. Deliberately null rather than a
  // skeleton: the banner sits in a single-column stack, so a placeholder would
  // shift the whole lobby down and then back up again on a fast response.
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
