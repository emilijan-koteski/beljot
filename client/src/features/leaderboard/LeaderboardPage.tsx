import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";

import { SectionHeader } from "@/features/profile/components/SectionHeader";
import { queryKeys } from "@/shared/api/queryKeys";
import type { SeasonSelector } from "@/shared/api/season";
import { LeaderboardRow } from "@/shared/components/season/LeaderboardRow";
import { Chips } from "@/shared/components/ui/chips";
import {
  useSeasonLeaderboardInfiniteQuery,
  useSeasonsQuery,
} from "@/shared/hooks/queries/useSeasonLeaderboard";
import { getTimeTick, subscribeTimeTick } from "@/shared/lib/timeTick";
import { useAuthStore } from "@/shared/stores/authStore";
import type { LeaderboardRow as LeaderboardRowData } from "@/shared/types/apiTypes";

const PAGE_SIZE = 25;

const SEASON_BOUNDARY_RETRY_MS = 5 * 60 * 1000;

/** The full seasonal leaderboard with load-more paging and the viewer's own row pinned when off-page. */
export function LeaderboardPage() {
  const { t } = useTranslation();

  const [selectedId, setSelectedId] = useState<number | null>(null);
  const seasonsQuery = useSeasonsQuery();
  const seasons = seasonsQuery.data?.items ?? [];
  const queryClient = useQueryClient();

  const tick = useSyncExternalStore(subscribeTimeTick, getTimeTick, getTimeTick);

  const nowMs = Date.now();

  const currentId = seasons.find(
    (s) => Date.parse(s.startedAt) <= nowMs && nowMs < Date.parse(s.endsAt),
  )?.id;

  let newestEndsAt: string | undefined;
  for (const s of seasons) {
    if (newestEndsAt === undefined || Date.parse(s.endsAt) > Date.parse(newestEndsAt)) {
      newestEndsAt = s.endsAt;
    }
  }
  const invalidatedForRef = useRef<{ endsAt: string; at: number } | null>(null);
  useEffect(() => {
    if (newestEndsAt === undefined) return;
    const end = Date.parse(newestEndsAt);
    const now = Date.now();
    if (!Number.isFinite(end) || now < end) return;
    const last = invalidatedForRef.current;
    if (last?.endsAt === newestEndsAt && now - last.at < SEASON_BOUNDARY_RETRY_MS) return;
    invalidatedForRef.current = { endsAt: newestEndsAt, at: now };
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.leaderboardAll() });
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.list() });
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.current() });
  }, [tick, newestEndsAt, queryClient]);

  const selectedSeason = selectedId === null ? undefined : seasons.find((s) => s.id === selectedId);
  const isPastSeason = selectedSeason !== undefined && Date.parse(selectedSeason.endsAt) <= nowMs;

  const seasonParam: SeasonSelector = selectedId === null ? "current" : selectedId;
  const query = useSeasonLeaderboardInfiniteQuery(PAGE_SIZE, seasonParam);
  const viewer = useAuthStore((s) => s.user);

  const items = useMemo<LeaderboardRowData[]>(() => {
    if (!query.data) return [];
    const seen = new Set<number>();
    const out: LeaderboardRowData[] = [];
    for (const page of query.data.pages) {
      for (const row of page.items) {
        if (seen.has(row.userId)) continue;
        seen.add(row.userId);
        out.push(row);
      }
    }
    return out;
  }, [query.data]);

  const latest = query.data?.pages.at(-1);
  const standing = latest?.viewer ?? null;
  const total = latest?.total ?? 0;

  const onPage = standing !== null && items.some((row) => row.userId === standing.userId);
  const pinned = standing !== null && !onPage ? standing : null;

  const showError = query.isError && items.length === 0;

  let body;
  if (query.isPending) {
    body = (
      <div
        className="space-y-1.5"
        data-testid="leaderboard-loading"
        role="status"
        aria-busy="true"
        aria-label={t("season.leaderboard.loading")}
      >
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="bg-surface h-10 animate-pulse rounded-md" />
        ))}
      </div>
    );
  } else if (showError) {
    body = (
      <p className="text-danger text-sm" data-testid="leaderboard-error" role="status">
        {t("season.leaderboard.error")}
      </p>
    );
  } else if (items.length === 0) {
    body = (
      <div
        className="bg-surface border-border rounded-lg border border-dashed p-10 text-center text-sm"
        data-testid="leaderboard-empty"
      >
        <p className="text-ink-dim m-0">
          {t(isPastSeason ? "season.leaderboard.emptyPast" : "season.leaderboard.empty")}
        </p>
      </div>
    );
  } else {
    const showLoadMore = query.hasNextPage;
    body = (
      <>
        <ul
          className="m-0 flex list-none flex-col gap-1.5 p-0"
          data-testid="leaderboard-list"
          aria-label={t("season.leaderboard.title")}
        >
          {items.map((row) => (
            <LeaderboardRow
              key={row.userId}
              position={row.position}
              userId={row.userId}
              username={row.username}
              avatarUrl={row.avatarUrl}
              sp={row.sp}
              tier={row.tier}
              division={row.division}
              gamesPlayed={row.gamesPlayed}
              isSelf={standing !== null && row.userId === standing.userId}
            />
          ))}
        </ul>

        {pinned && (
          <div className="mt-2.5" data-testid="leaderboard-pinned">
            <p className="text-ink-mute mb-1.5 text-2xs uppercase">
              {t("season.leaderboard.yourPosition")}
            </p>
            <ul className="m-0 flex list-none flex-col p-0">
              <LeaderboardRow
                position={pinned.position}
                userId={pinned.userId}
                username={viewer?.username || t("season.leaderboard.you")}
                avatarUrl={viewer?.avatarUrl ?? null}
                sp={pinned.sp}
                tier={pinned.tier}
                division={pinned.division}
                gamesPlayed={pinned.gamesPlayed}
                isSelf
              />
            </ul>
          </div>
        )}

        {query.isError && (
          <div className="mt-2.5 flex items-center gap-3" data-testid="leaderboard-page-error">
            <p className="text-danger m-0 text-xs" role="status">
              {t("season.leaderboard.error")}
            </p>
            <button
              type="button"
              onClick={() => query.fetchNextPage()}
              className="text-accent cursor-pointer text-xs underline-offset-2 hover:underline"
              data-testid="leaderboard-retry"
            >
              {t("season.leaderboard.retry")}
            </button>
          </div>
        )}

        {showLoadMore && (
          <button
            type="button"
            onClick={() => query.fetchNextPage()}
            disabled={query.isFetchingNextPage}
            className="bg-surface border-border text-ink hover:bg-surface-elevated mt-2.5 w-full rounded-lg border px-4 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-60"
            data-testid="leaderboard-load-more"
          >
            {query.isFetchingNextPage
              ? t("season.leaderboard.loading")
              : t("season.leaderboard.loadMore")}
          </button>
        )}

        <p
          className="text-ink-mute mt-4 text-center text-xs"
          data-testid="leaderboard-count"
          aria-live="polite"
          aria-atomic="true"
        >
          {t("season.leaderboard.showing", { shown: items.length, total })}
        </p>
      </>
    );
  }

  return (
    <div className="mx-auto max-w-220 px-4 py-8 pb-32 md:px-7" data-testid="leaderboard-page">
      <SectionHeader
        eyebrow={t("season.leaderboard.eyebrow")}
        title={t("season.leaderboard.title")}
        sub={t(isPastSeason ? "season.leaderboard.subPast" : "season.leaderboard.sub", {
          season: selectedSeason?.name ?? "",
        })}
      />

      {seasons.length > 1 && (
        <Chips
          value={selectedId ?? currentId ?? -1}
          onValueChange={(id) =>
            setSelectedId(currentId !== undefined && id === currentId ? null : id)
          }
          options={seasons.map((s) => ({
            value: s.id,
            label:
              s.id === currentId ? (
                <>
                  {s.name}
                  <span className="text-2xs tracking-wider uppercase opacity-70">
                    {t("season.picker.current")}
                  </span>
                </>
              ) : (
                s.name
              ),
          }))}
          ariaLabel={t("season.picker.label")}
          testId="leaderboard-season-picker"
          className="mb-4"
        />
      )}

      {body}
    </div>
  );
}
