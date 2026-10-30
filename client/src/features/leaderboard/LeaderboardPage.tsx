import { useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";

import { RankLadderPanel, SeasonPanel } from "@/features/leaderboard/components/LeaderboardPanels";
import { LeaderboardPodium } from "@/features/leaderboard/components/LeaderboardPodium";
import { queryKeys } from "@/shared/api/queryKeys";
import type { SeasonSelector } from "@/shared/api/season";
import { EmptyState } from "@/shared/components/EmptyState";
import { PageShell } from "@/shared/components/layout/PageShell";
import { LeaderboardRow, LeaderboardTableHeader } from "@/shared/components/season/LeaderboardRow";
import { SeasonStandingCard } from "@/shared/components/season/SeasonStandingCard";
import { Button } from "@/shared/components/ui/button";
import { Chips } from "@/shared/components/ui/chips";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { Skeleton } from "@/shared/components/ui/skeleton";
import { useCurrentSeasonQuery } from "@/shared/hooks/queries/useCurrentSeason";
import {
  useSeasonLeaderboardInfiniteQuery,
  useSeasonsQuery,
} from "@/shared/hooks/queries/useSeasonLeaderboard";
import { useLayoutBand } from "@/shared/hooks/useLayoutBand";
import { normalizeSeasonTier, seasonDaysRemaining } from "@/shared/lib/seasonTier";
import { getTimeTick, subscribeTimeTick } from "@/shared/lib/timeTick";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";
import type { LeaderboardRow as LeaderboardRowData } from "@/shared/types/apiTypes";

const PAGE_SIZE = 25;

const SEASON_BOUNDARY_RETRY_MS = 5 * 60 * 1000;

/** Whether the observed element has scrolled up past the top of the viewport. */
function useScrolledPast(enabled: boolean) {
  const [node, setNode] = useState<HTMLElement | null>(null);
  const [past, setPast] = useState(false);
  useEffect(() => {
    if (!enabled || node === null || typeof IntersectionObserver === "undefined") {
      setPast(false);
      return;
    }
    const observer = new IntersectionObserver(([entry]) => {
      setPast(entry !== undefined && !entry.isIntersecting && entry.boundingClientRect.top < 0);
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [enabled, node]);
  return [setNode, past] as const;
}

/**
 * The season leaderboard: your standing, the podium, then the list (a table on wider screens) with load-more
 * paging; your own row closes the list when it is off-page and pins to a phone's bottom edge once you scroll.
 */
export function LeaderboardPage() {
  const { t } = useTranslation();

  const [selectedId, setSelectedId] = useState<number | null>(null);
  const band = useLayoutBand();
  const phone = band === "phone";
  const desktop = band === "desktop";
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

  const currentSeason = useCurrentSeasonQuery(!isPastSeason);
  const shownSeason = selectedSeason ?? seasons.find((s) => s.id === currentId);
  const seasonName = shownSeason?.name ?? currentSeason.data?.seasonName ?? "";
  const daysLeft = seasonDaysRemaining(shownSeason?.endsAt ?? currentSeason.data?.endsAt, nowMs);
  const seasonLine = isPastSeason
    ? t("season.leaderboard.seasonEnded", { season: seasonName })
    : t("season.leaderboard.seasonLine", { season: seasonName, days: daysLeft });

  const podium = items[0]?.position === 1 ? items.filter((row) => row.position <= 3) : [];
  const listed = podium.length > 0 ? items.filter((row) => row.position > 3) : items;
  const viewerId = standing?.userId ?? null;

  const [watchStanding, scrolledPast] = useScrolledPast(phone && standing !== null);
  const showStickySelf = phone && standing !== null && scrolledPast;

  const selfRow = (layout: "list" | "table") =>
    standing && (
      <LeaderboardRow
        position={standing.position}
        userId={standing.userId}
        username={viewer?.username || t("season.leaderboard.you")}
        avatarUrl={viewer?.avatarUrl ?? null}
        sp={standing.sp}
        tier={standing.tier}
        division={standing.division}
        gamesPlayed={standing.gamesPlayed}
        gamesWon={standing.gamesWon}
        layout={layout}
        showRank={desktop}
        isSelf
      />
    );

  const standingCard = (layout: "stack" | "row") => (
    <SeasonStandingCard
      standing={standing}
      total={total}
      progress={isPastSeason ? undefined : currentSeason.data}
      past={isPastSeason}
      layout={layout}
    />
  );

  const loadMore = query.hasNextPage && (
    <Button
      variant="outline"
      size={phone ? "default" : "sm"}
      onClick={() => query.fetchNextPage()}
      disabled={query.isFetchingNextPage}
      className={cn(phone && "h-10 w-full")}
      data-testid="leaderboard-load-more"
    >
      {query.isFetchingNextPage
        ? t("season.leaderboard.loading")
        : t("season.leaderboard.loadMore")}
    </Button>
  );

  const count = (
    <p
      className="text-ink-mute m-0 text-center text-xs"
      data-testid="leaderboard-count"
      aria-live="polite"
      aria-atomic="true"
    >
      {t("season.leaderboard.showing", { shown: items.length, total })}
    </p>
  );

  const pageError = query.isError && (
    <div className="flex items-center gap-3" data-testid="leaderboard-page-error">
      <p className="text-danger m-0 text-xs" role="status">
        {t("season.leaderboard.error")}
      </p>
      <Button
        variant="link"
        size="inline"
        onClick={() => query.fetchNextPage()}
        className="text-xs"
        data-testid="leaderboard-retry"
      >
        {t("season.leaderboard.retry")}
      </Button>
    </div>
  );

  const gap = (
    <li aria-hidden="true" className="text-ink-mute py-1 text-center text-xs tracking-[0.3em]">
      ···
    </li>
  );

  let body: ReactNode;
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
          <Skeleton key={i} className="h-12 rounded-md" />
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
      <EmptyState testId="leaderboard-empty">
        {t(isPastSeason ? "season.leaderboard.emptyPast" : "season.leaderboard.empty")}
      </EmptyState>
    );
  } else {
    const rows = listed.map((row) => (
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
        gamesWon={row.gamesWon}
        layout={phone ? "list" : "table"}
        showRank={desktop}
        isSelf={row.userId === viewerId}
      />
    ));
    const pinnedRows = pinned && (
      <ul className="m-0 list-none p-0" data-testid="leaderboard-pinned">
        {gap}
        {selfRow(phone ? "list" : "table")}
      </ul>
    );
    body = (
      <>
        {podium.length > 0 &&
          (phone ? (
            <LeaderboardPodium top={podium} viewerId={viewerId} size="sm" />
          ) : (
            <div className="bg-surface border-border rounded-xl border px-6 pt-8 shadow-xs">
              <LeaderboardPodium top={podium} viewerId={viewerId} size="lg" />
            </div>
          ))}
        {phone ? (
          <>
            {(listed.length > 0 || pinned) && (
              <div className="bg-surface border-border rounded-xl border shadow-xs">
                <ul
                  className="divide-border m-0 flex list-none flex-col divide-y p-0"
                  data-testid="leaderboard-list"
                  aria-label={t("season.leaderboard.title")}
                >
                  {rows}
                </ul>
                {pinnedRows}
              </div>
            )}
            {pageError}
            {loadMore}
            {count}
          </>
        ) : (
          <section className="bg-surface border-border overflow-hidden rounded-xl border shadow-xs">
            <LeaderboardTableHeader showRank={desktop} />
            <ul
              className="divide-border m-0 list-none divide-y p-0"
              data-testid="leaderboard-list"
              aria-label={t("season.leaderboard.title")}
            >
              {rows}
            </ul>
            {pinnedRows}
            <div className="border-border flex flex-wrap items-center justify-between gap-3 border-t px-4 py-2.5">
              {count}
              {pageError}
              {loadMore}
            </div>
          </section>
        )}
      </>
    );
  }

  const stickySelf = showStickySelf && (
    <ul
      className="bg-surface border-accent/40 m-0 list-none rounded-xl border p-0 shadow-lg"
      data-testid="leaderboard-sticky-self"
    >
      {selfRow("list")}
    </ul>
  );

  return (
    <PageShell
      testId="leaderboard-page"
      sidebar={
        desktop ? (
          <>
            {standingCard("stack")}
            <SeasonPanel line={seasonLine} />
            <RankLadderPanel
              yours={standing ? normalizeSeasonTier(standing.tier, standing.sp) : null}
            />
          </>
        ) : undefined
      }
      bottomBar={stickySelf || undefined}
    >
      <div className={cn("flex gap-3", phone ? "flex-col" : "items-end justify-between")}>
        <div className="flex min-w-0 flex-col gap-1">
          <Eyebrow>{seasonLine}</Eyebrow>
          <h1 className="text-ink m-0 text-2xl font-semibold tracking-tight md:text-3xl">
            {t("season.leaderboard.title")}
          </h1>
        </div>
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
          />
        )}
      </div>

      {!desktop && !query.isPending && !showError && (
        <div ref={watchStanding}>{standingCard(phone ? "stack" : "row")}</div>
      )}

      {body}
    </PageShell>
  );
}
