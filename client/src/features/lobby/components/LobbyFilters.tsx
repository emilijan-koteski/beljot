import { Clock3, UsersRound } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  type FilterCounts,
  LOBBY_FILTERS,
  type LobbyFilter,
  type LobbySort,
} from "@/features/lobby/lib/lobbyRooms";
import { FilterBar } from "@/shared/components/FilterBar";
import { SortMenu, type SortOption } from "@/shared/components/SortMenu";
import { Chip } from "@/shared/components/ui/chips";

interface LobbyFiltersProps {
  search: string;
  setSearch: (next: string) => void;
  filter: LobbyFilter;
  setFilter: (next: LobbyFilter) => void;
  sort: LobbySort;
  setSort: (next: LobbySort) => void;
  counts: FilterCounts;
  collapsed: boolean;
}

/** Room search, the five filters with their counts, and the sort order; `collapsed` folds search away on smaller screens. */
export function LobbyFilters({
  search,
  setSearch,
  filter,
  setFilter,
  sort,
  setSort,
  counts,
  collapsed,
}: LobbyFiltersProps) {
  const { t } = useTranslation();
  const sortOptions: SortOption<LobbySort>[] = [
    { value: "filling", label: t("lobby.sort.filling"), icon: UsersRound },
    { value: "newest", label: t("lobby.sort.newest"), icon: Clock3 },
  ];
  const activeSort = sortOptions.find((o) => o.value === sort) ?? sortOptions[0]!;

  return (
    <FilterBar
      search={search}
      onSearchChange={setSearch}
      searchPlaceholder={t("lobby.search.placeholder")}
      openSearchLabel={t("lobby.search.open")}
      closeSearchLabel={t("lobby.search.close")}
      collapsed={collapsed}
      searchTestId="room-list-search"
      filters={
        <div className="flex gap-1.5" role="group" aria-label={t("lobby.filters.label")}>
          {LOBBY_FILTERS.map((f) => (
            <Chip
              key={f}
              active={filter === f}
              count={counts[f]}
              onClick={() => setFilter(f)}
              aria-pressed={filter === f}
              data-testid={`filter-chip-${f}`}
            >
              {t(`lobby.filters.${f}`)}
            </Chip>
          ))}
        </div>
      }
      trailing={
        <SortMenu
          value={sort}
          options={sortOptions}
          onValueChange={setSort}
          ariaLabel={t("lobby.sort.ariaLabel", { order: activeSort.label })}
          labelled={!collapsed}
          testId="sort-toggle"
        />
      }
    />
  );
}
