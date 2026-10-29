import { Search, X } from "lucide-react";
import { type ReactNode, useState } from "react";

import { controlVariants } from "@/shared/components/ui/control";
import { SearchInput } from "@/shared/components/ui/search-input";

interface FilterBarProps {
  search: string;
  onSearchChange: (next: string) => void;
  searchPlaceholder: string;
  openSearchLabel: string;
  closeSearchLabel: string;
  collapsed: boolean;
  filters: ReactNode;
  trailing?: ReactNode;
  searchTestId?: string;
}

const SCROLLING_FILTERS =
  "min-w-0 flex-1 overflow-x-auto pr-4 [mask-image:linear-gradient(to_right,black_calc(100%-1rem),transparent)] [scrollbar-width:none]";

/**
 * Search, a row of filter chips and a trailing control (usually a SortMenu), 32px tall.
 * `collapsed` (phones and tablets) folds the search behind a button and lets the chips scroll;
 * otherwise the search box stays open at 240px.
 */
export function FilterBar({
  search,
  onSearchChange,
  searchPlaceholder,
  openSearchLabel,
  closeSearchLabel,
  collapsed,
  filters,
  trailing,
  searchTestId = "filter-search",
}: FilterBarProps) {
  const [searchOpen, setSearchOpen] = useState(search.length > 0);

  if (!collapsed) {
    return (
      <div className="flex items-center gap-2">
        <SearchInput
          value={search}
          onValueChange={onSearchChange}
          placeholder={searchPlaceholder}
          aria-label={searchPlaceholder}
          clearLabel={closeSearchLabel}
          data-testid={searchTestId}
          wrapperClassName="h-8 w-60 shrink-0 py-0"
        />
        <div className={SCROLLING_FILTERS}>{filters}</div>
        {trailing}
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2">
      {searchOpen ? (
        <SearchInput
          autoFocus
          value={search}
          onValueChange={onSearchChange}
          placeholder={searchPlaceholder}
          aria-label={searchPlaceholder}
          data-testid={searchTestId}
          wrapperClassName="h-8 min-w-0 flex-1 py-0"
        />
      ) : (
        <>
          <button
            type="button"
            aria-label={openSearchLabel}
            onClick={() => setSearchOpen(true)}
            className={controlVariants()}
            data-testid={`${searchTestId}-open`}
          >
            <Search aria-hidden="true" />
          </button>
          <div className={SCROLLING_FILTERS}>{filters}</div>
        </>
      )}
      {searchOpen && (
        <button
          type="button"
          aria-label={closeSearchLabel}
          onClick={() => {
            onSearchChange("");
            setSearchOpen(false);
          }}
          className={controlVariants()}
          data-testid={`${searchTestId}-close`}
        >
          <X aria-hidden="true" />
        </button>
      )}
      {trailing}
    </div>
  );
}
