import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import type { MatchFilter, MatchListItem, MatchSort } from "@/shared/api/matches";
import { EmptyState } from "@/shared/components/EmptyState";
import { MatchStatsCard } from "@/shared/components/matchStats/MatchStatsCard";
import { Button, buttonVariants } from "@/shared/components/ui/button";
import { Skeleton } from "@/shared/components/ui/skeleton";
import { useUserMatchesInfiniteQuery } from "@/shared/hooks/queries/useMatches";

import { HistoryFilters } from "./components/HistoryFilters";

interface MatchHistoryProps {
  userId: number | undefined;
  counts: Record<MatchFilter, number>;
  subjectIsSelf?: boolean;
}

export function MatchHistory({ userId, counts, subjectIsSelf = true }: MatchHistoryProps) {
  const { t } = useTranslation();
  const [filter, setFilter] = useState<MatchFilter>("all");
  const [sort, setSort] = useState<MatchSort>("new");
  const [openIds, setOpenIds] = useState<Set<number>>(new Set());

  const query = useUserMatchesInfiniteQuery(userId, { outcome: filter, sort });

  const items = useMemo<MatchListItem[]>(() => {
    if (!query.data) return [];
    return query.data.pages.flatMap((p) => p.items);
  }, [query.data]);

  const toggleOpen = (id: number) => {
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const header = (
    <h2 className="text-ink m-0 text-lg font-semibold">{t("profile.matchHistory.title")}</h2>
  );

  const hasNoGamesAtAll = counts.all === 0 && !query.isPending && !query.isError;
  if (hasNoGamesAtAll) {
    return (
      <section data-testid="match-history" className="flex flex-col gap-3">
        {header}
        <EmptyState
          testId="match-history-empty"
          action={
            subjectIsSelf && (
              <Link
                to="/lobby"
                className={buttonVariants({ variant: "link", size: "inline" })}
                data-testid="match-history-empty-cta"
              >
                {t("profile.matchHistory.emptyCta")}
              </Link>
            )
          }
        >
          {t(subjectIsSelf ? "profile.matchHistory.empty" : "profile.matchHistory.emptyPublic")}
        </EmptyState>
      </section>
    );
  }

  const filters = (
    <HistoryFilters
      filter={filter}
      onFilterChange={setFilter}
      counts={counts}
      sort={sort}
      onSortChange={setSort}
    />
  );

  let body;
  if (query.isPending) {
    body = (
      <div className="space-y-2.5" data-testid="match-history-loading">
        <Skeleton className="h-20 rounded-lg" />
        <Skeleton className="h-20 rounded-lg" />
        <Skeleton className="h-20 rounded-lg" />
      </div>
    );
  } else if (query.isError) {
    body = (
      <p className="text-danger text-sm" data-testid="match-history-error">
        {t("profile.matchHistory.error")}
      </p>
    );
  } else if (items.length === 0) {
    body = (
      <EmptyState testId="match-history-empty-filtered">
        {t("profile.matchHistory.emptyFiltered")}
      </EmptyState>
    );
  } else {
    const total = query.data?.pages[0]?.total ?? 0;
    const showLoadMore = items.length < total;
    body = (
      <>
        <ul className="m-0 flex list-none flex-col gap-2.5 p-0" data-testid="match-history-list">
          {items.map((match) => (
            <MatchStatsCard
              key={match.id}
              match={match}
              subjectIsSelf={subjectIsSelf}
              isOpen={openIds.has(match.id)}
              onToggle={() => toggleOpen(match.id)}
            />
          ))}
        </ul>
        {showLoadMore && (
          <Button
            variant="outline"
            onClick={() => query.fetchNextPage()}
            disabled={query.isFetchingNextPage}
            className="h-9 w-full"
            data-testid="match-history-load-more"
          >
            {query.isFetchingNextPage
              ? t("profile.matchHistory.loading")
              : t("profile.matchHistory.loadMore")}
          </Button>
        )}
        <p className="text-ink-mute m-0 text-center text-xs" data-testid="match-history-count">
          {t("profile.matchHistory.showing", { shown: items.length, total })}
        </p>
      </>
    );
  }

  return (
    <section data-testid="match-history" className="flex flex-col gap-3">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        {header}
        {filters}
      </div>
      {body}
    </section>
  );
}
