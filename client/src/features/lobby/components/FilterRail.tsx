import { ArrowDownWideNarrow, Search, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
import { Chip } from "@/shared/components/ui/chips";

export type LobbyFilter = "all" | "qualify" | "open" | "relaxed" | "timed";
export type LobbySort = "newest" | "filling";

export type FilterCounts = Record<LobbyFilter, number>;

type Props = {
  search: string;
  setSearch: (next: string) => void;
  filter: LobbyFilter;
  setFilter: (next: LobbyFilter) => void;
  sort: LobbySort;
  setSort: (next: LobbySort) => void;
  counts: FilterCounts;
};

const FILTERS: LobbyFilter[] = ["all", "qualify", "open", "relaxed", "timed"];

export function FilterRail({ search, setSearch, filter, setFilter, sort, setSort, counts }: Props) {
  const { t } = useTranslation();
  return (
    <div className="bg-surface mb-3.5 flex flex-wrap items-center gap-3.5 rounded-lg border border-border p-3.5">
      <div className="bg-surface-elevated flex min-w-50 flex-1 items-center gap-2 rounded-md border border-border px-2.5 py-1.5 transition-[border-color,box-shadow] has-[input:focus-visible]:border-ring has-[input:focus-visible]:ring-3 has-[input:focus-visible]:ring-ring/70">
        <Search className="text-ink-mute size-3.5" />
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("lobby.search.placeholder")}
          data-testid="room-list-search"
          className="text-ink min-w-0 flex-1 truncate bg-transparent text-sm outline-none placeholder:text-ink-mute"
        />
        {search && (
          <button
            onClick={() => setSearch("")}
            data-testid="room-list-clear-search"
            aria-label="Clear search"
            className="text-ink-mute flex p-0.5"
          >
            <X className="size-3.5" />
          </button>
        )}
      </div>

      <div className="flex flex-wrap gap-1.5">
        {FILTERS.map((f) => {
          const active = filter === f;
          return (
            <Chip
              key={f}
              active={active}
              count={counts[f]}
              onClick={() => setFilter(f)}
              data-testid={`filter-chip-${f}`}
              aria-pressed={active}
            >
              {t(`lobby.filters.${f}`)}
            </Chip>
          );
        })}
      </div>

      <div className="ml-auto flex items-center gap-1.5">
        <span className="text-ink-mute text-xs">{t("lobby.sort.label")}</span>
        <Button
          variant="outline"
          onClick={() => setSort(sort === "newest" ? "filling" : "newest")}
          data-testid="sort-toggle"
        >
          {sort === "newest" ? t("lobby.sort.newest") : t("lobby.sort.filling")}
          <ArrowDownWideNarrow />
        </Button>
      </div>
    </div>
  );
}
