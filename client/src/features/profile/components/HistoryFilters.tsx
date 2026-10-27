import { ArrowDownWideNarrow, ArrowUpNarrowWide } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { MatchFilter, MatchSort } from "@/shared/api/matches";
import { Button } from "@/shared/components/ui/button";
import { Chip } from "@/shared/components/ui/chips";

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
            <Chip
              key={f.id}
              active={active}
              count={counts[f.id]}
              onClick={() => onFilterChange(f.id)}
              aria-pressed={active}
              data-testid={`match-history-filter-${f.id}`}
              data-active={active}
            >
              {t(f.labelKey)}
            </Chip>
          );
        })}
      </div>

      <div className="ml-auto flex items-center gap-1.5">
        <span className="text-ink-mute text-xs">{t("profile.matchHistory.sort.label")}</span>
        <Button
          variant="outline"
          onClick={() => onSortChange(sort === "new" ? "old" : "new")}
          data-testid="match-history-sort"
          data-sort={sort}
        >
          {sort === "new"
            ? t("profile.matchHistory.sort.newest")
            : t("profile.matchHistory.sort.oldest")}
          {sort === "new" ? <ArrowDownWideNarrow /> : <ArrowUpNarrowWide />}
        </Button>
      </div>
    </div>
  );
}
