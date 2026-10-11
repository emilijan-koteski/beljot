import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { queryKeys } from "@/shared/api/queryKeys";
import { MOTION } from "@/shared/lib/motion";
import { getTimeTick, subscribeTimeTick } from "@/shared/lib/timeTick";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

const SEASON_BOUNDARY_RETRY_MS = 5 * 60 * 1000;

/**
 * Watches the active season window and, when it ends, refetches the season queries and toasts
 * the new window once. Call exactly once per app, since each mounted copy fires its own toast.
 */
export function useSeasonWindowWatch(season: CurrentSeasonResponse | undefined): void {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const tick = useSyncExternalStore(subscribeTimeTick, getTimeTick, getTimeTick);

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
