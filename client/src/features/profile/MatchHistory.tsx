import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import type { MatchFilter, MatchListItem, MatchSort } from "@/shared/api/matches";
import { EmptyState } from "@/shared/components/EmptyState";
import { MatchStatsCard } from "@/shared/components/matchStats/MatchStatsCard";
import { useUserMatchesInfiniteQuery } from "@/shared/hooks/queries/useMatches";

import { HistoryFilters } from "./components/HistoryFilters";
import { SectionHeader } from "./components/SectionHeader";

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
    <SectionHeader
      eyebrow={t("profile.matchHistory.eyebrow")}
      title={t("profile.matchHistory.title")}
      sub={t("profile.matchHistory.sub")}
    />
  );

  const hasNoGamesAtAll = counts.all === 0 && !query.isPending && !query.isError;
  if (hasNoGamesAtAll) {
    return (
      <section data-testid="match-history">
        {header}
        <EmptyState
          testId="match-history-empty"
          action={
            subjectIsSelf && (
              <Link
                to="/lobby"
                className="text-accent inline-flex items-center text-sm underline-offset-2 hover:underline"
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
        <div className="bg-surface h-20 animate-pulse rounded-lg" />
        <div className="bg-surface h-20 animate-pulse rounded-lg" />
        <div className="bg-surface h-20 animate-pulse rounded-lg" />
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
          <button
            type="button"
            onClick={() => query.fetchNextPage()}
            disabled={query.isFetchingNextPage}
            className="bg-surface border-border text-ink hover:bg-surface-elevated mt-2.5 w-full rounded-lg border px-4 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-60"
            data-testid="match-history-load-more"
          >
            {query.isFetchingNextPage
              ? t("profile.matchHistory.loading")
              : t("profile.matchHistory.loadMore")}
          </button>
        )}
        <p className="text-ink-mute mt-4 text-center text-xs" data-testid="match-history-count">
          {t("profile.matchHistory.showing", { shown: items.length, total })}
        </p>
      </>
    );
  }

  return (
    <section data-testid="match-history">
      {header}
      {filters}
      {body}
    </section>
  );
}
