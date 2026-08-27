import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { queryKeys } from "@/shared/api/queryKeys";
import { MOTION } from "@/shared/lib/motion";
import { getTimeTick, subscribeTimeTick } from "@/shared/lib/timeTick";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

/**
 * How long the boundary effect waits before re-firing for a window it has
 * already acted on. Only reachable when the client's clock runs AHEAD of the
 * server's: the early refetch returns the same still-active season, and this is
 * what lets the real boundary still be caught instead of being swallowed by an
 * already-consumed guard.
 */
const SEASON_BOUNDARY_RETRY_MS = 5 * 60 * 1000;

/**
 * Watches the active season window and reacts when it ends: refetch the season
 * queries, then announce the new window once.
 *
 * EXTRACTED FROM RankBanner, which owned both effects while it lived in the
 * lobby. The banner has since moved to the profile, and the always-visible rank
 * surface is now the header chip — so this is mounted THERE instead, which is
 * where it belongs: the chip renders on every authed route, whereas the banner
 * only renders on one page a player may never open. A lobby left open across a
 * quarter boundary would otherwise show a dead season's tier in the header
 * forever, since `season.current` is deliberately unpolled.
 *
 * CALL THIS EXACTLY ONCE PER APP. It fires a toast; two mounted copies would
 * fire two. RankBanner deliberately no longer carries it.
 *
 * `seasonDaysRemaining` floors at 0 forever, so without the first effect below
 * nothing would ever notice the rollover. When the shared 30s tick observes
 * `endsAt` in the past, invalidate `season.current`, every leaderboard entry
 * (page + any widget) and the seasons list (the picker's feed, which gains a
 * window at every boundary). Pull-only stays intact: this is client-side clock
 * observation, not a push.
 */
export function useSeasonWindowWatch(season: CurrentSeasonResponse | undefined): void {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  // Subscribe to the single shared 30s tick so the boundary effect below
  // re-evaluates as the countdown crosses zero, without owning an interval.
  const tick = useSyncExternalStore(subscribeTimeTick, getTimeTick, getTimeTick);

  // ONCE PER BOUNDARY, BUT NOT ONCE ONLY. The ref keys on the endsAt value, so
  // the refetched NEW season re-arms it for the next quarter while re-renders
  // and further ticks of the dead one do nothing. The `at` stamp is the
  // CLOCK-SKEW ESCAPE: if this client's clock runs ahead of the server, the
  // first firing refetches the SAME still-active window, and a ref consumed
  // forever would leave nothing to fire at the real boundary. Re-arming after
  // RETRY_MS turns that dead end into a slow retry that costs one refetch per
  // five minutes of skew at most.
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
}
