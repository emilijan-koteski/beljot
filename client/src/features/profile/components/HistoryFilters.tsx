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
 * Filter chips with server-accurate counts, scrolling sideways when they run out of room, and a newest/oldest toggle.
 * Selecting a chip or toggling the sort re-queries from page one via the infinite query's key.
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
    <div className="flex min-w-0 items-center gap-2" data-testid="match-history-filters">
      <div className="flex min-w-0 flex-1 gap-1.5 overflow-x-auto [scrollbar-width:none]">
        {FILTERS.map((f) => {
          const active = filter === f.id;
          return (
            <Chip
              key={f.id}
              active={active}
              count={counts[f.id]}
              onClick={() => onFilterChange(f.id)}
              aria-pressed={active}
              className="shrink-0"
              data-testid={`match-history-filter-${f.id}`}
              data-active={active}
            >
              {t(f.labelKey)}
            </Chip>
          );
        })}
      </div>

      <div className="flex shrink-0 items-center gap-1.5">
        <Button
          variant="outline"
          size="sm"
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
