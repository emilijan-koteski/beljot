import { ArrowDownWideNarrow, Search, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/shared/lib/utils";

export type LobbyFilter = "all" | "open" | "relaxed" | "timed";
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

const FILTERS: LobbyFilter[] = ["all", "open", "relaxed", "timed"];

export function FilterRail({ search, setSearch, filter, setFilter, sort, setSort, counts }: Props) {
  const { t } = useTranslation();
  return (
    <div className="bg-surface mb-3.5 flex flex-wrap items-center gap-3.5 rounded-lg border border-border p-3.5">
      <div className="bg-surface-elevated flex min-w-50 flex-1 items-center gap-2 rounded-[10px] border border-border px-2.5 py-1.5">
        <Search className="text-ink-mute size-3.5" />
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("lobby.search.placeholder")}
          data-testid="room-list-search"
          // `truncate` (overflow-hidden + nowrap + text-ellipsis) makes the
          // placeholder ellipsize instead of hard-clipping when the field is
          // too narrow for it — width-driven, so it adapts to any viewport.
          className="text-ink min-w-0 flex-1 truncate bg-transparent text-sm outline-none placeholder:text-ink-off"
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
            <button
              key={f}
              onClick={() => setFilter(f)}
              data-testid={`filter-chip-${f}`}
              aria-pressed={active}
              className={cn(
                "inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors",
                active
                  ? "border-accent/50 bg-accent-soft text-accent"
                  : "border-border bg-transparent text-ink-dim hover:bg-surface-sunken",
              )}
            >
              {t(`lobby.filters.${f}`)}
              <span
                className={cn(
                  "tabular-nums rounded-md px-1.5 py-px text-[10.5px] font-semibold",
                  active ? "bg-accent/15 text-accent" : "bg-surface-elevated text-ink-mute",
                )}
              >
                {counts[f]}
              </span>
            </button>
          );
        })}
      </div>

      <div className="ml-auto flex items-center gap-1.5">
        <span className="text-ink-mute text-xs">{t("lobby.sort.label")}</span>
        <button
          onClick={() => setSort(sort === "newest" ? "filling" : "newest")}
          data-testid="sort-toggle"
          className="bg-surface-elevated text-ink inline-flex items-center gap-1.5 rounded-[10px] border border-border px-3 py-1.5 text-xs font-medium"
        >
          {sort === "newest" ? t("lobby.sort.newest") : t("lobby.sort.filling")}
          <ArrowDownWideNarrow className="size-3.5" />
        </button>
      </div>
    </div>
  );
}
