import { ArrowDownWideNarrow, ArrowUpNarrowWide } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { MatchFilter, MatchSort } from "@/shared/api/matches";

type HistoryFiltersProps = {
  filter: MatchFilter;
  onFilterChange: (next: MatchFilter) => void;
  counts: Record<MatchFilter, number>;
  sort: MatchSort;
  onSortChange: (next: MatchSort) => void;
};

const FILTERS: { id: MatchFilter; labelKey: string }[] = [
  { id: "all", labelKey: "profile.matchHistory.filter.all" },
  { id: "win", labelKey: "profile.matchHistory.filter.win" },
  { id: "loss", labelKey: "profile.matchHistory.filter.loss" },
  { id: "abandoned", labelKey: "profile.matchHistory.filter.abandoned" },
];

/**
 * Filter chips (with server-accurate counts) + a newest/oldest sort toggle
 * above the match list. Selecting a chip / toggling sort re-queries from page
 * one via the infinite query's key.
 */
export function HistoryFilters({
  filter,
  onFilterChange,
  counts,
  sort,
  onSortChange,
}: HistoryFiltersProps) {
  const { t } = useTranslation();

  return (
    <div
      className="bg-surface border-border mb-3.5 flex flex-wrap items-center gap-2.5 rounded-lg border px-4 py-3"
      data-testid="match-history-filters"
    >
      <div className="flex flex-wrap gap-1.5">
        {FILTERS.map((f) => {
          const active = filter === f.id;
          return (
            <button
              key={f.id}
              type="button"
              onClick={() => onFilterChange(f.id)}
              aria-pressed={active}
              data-testid={`match-history-filter-${f.id}`}
              data-active={active}
              className="inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-3 py-1.5 text-sm font-medium transition-colors"
              style={
                active
                  ? {
                      background: "var(--accent-soft)",
                      borderColor: "rgba(25,101,54,0.33)",
                      color: "var(--accent)",
                    }
                  : {
                      background: "transparent",
                      borderColor: "var(--border)",
                      color: "var(--ink-dim)",
                    }
              }
            >
              {t(f.labelKey)}
              <span
                className="rounded-md px-1.5 text-xs tabular-nums"
                style={{
                  background: active ? "rgba(25,101,54,0.14)" : "var(--surface-2)",
                }}
              >
                {counts[f.id]}
              </span>
            </button>
          );
        })}
      </div>

      <div className="ml-auto flex items-center gap-1.5">
        <span className="text-ink-mute text-xs">{t("profile.matchHistory.sort.label")}</span>
        <button
          type="button"
          onClick={() => onSortChange(sort === "new" ? "old" : "new")}
          data-testid="match-history-sort"
          data-sort={sort}
          className="bg-surface-elevated border-border text-ink inline-flex cursor-pointer items-center gap-1.5 rounded-md border px-3 py-1.5 text-sm font-medium"
        >
          {sort === "new"
            ? t("profile.matchHistory.sort.newest")
            : t("profile.matchHistory.sort.oldest")}
          {sort === "new" ? (
            <ArrowDownWideNarrow className="size-3.5" />
          ) : (
            <ArrowUpNarrowWide className="size-3.5" />
          )}
        </button>
      </div>
    </div>
  );
}
