import { ArrowDownWideNarrow } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
import { Chip } from "@/shared/components/ui/chips";
import { SearchInput } from "@/shared/components/ui/search-input";

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
      <SearchInput
        value={search}
        onValueChange={setSearch}
        placeholder={t("lobby.search.placeholder")}
        aria-label={t("lobby.search.placeholder")}
        data-testid="room-list-search"
        clearLabel={t("lobby.empty.clear")}
        clearTestId="room-list-clear-search"
        wrapperClassName="min-w-50 flex-1"
      />

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
