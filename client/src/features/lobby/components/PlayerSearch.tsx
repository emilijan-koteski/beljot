import { Search, UserRound, X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { useUserSearch } from "@/shared/hooks/queries/useUserSearch";
import { useDebounce } from "@/shared/hooks/useDebounce";

/**
 * Live player search (Story 11.1, FR5). Owns the raw input value, debounces it,
 * and feeds the debounced term to useUserSearch. Results are keyboard-accessible
 * buttons that navigate to the player's public profile (/players/:id — the route
 * and page were delivered by Story 11.3).
 */
export function PlayerSearch() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [value, setValue] = useState("");

  const debounced = useDebounce(value);
  const trimmed = debounced.trim();
  // Mirrors the hook's own `enabled` gate: below 2 trimmed chars no request
  // fires, and neither the loading nor the empty affordance should show.
  const isActive = trimmed.length >= 2;

  const query = useUserSearch(debounced);
  const results = query.data ?? [];

  // Only a settled, non-fetching, successful empty result is a real "no matches"
  // — while the first request is in flight we show the loading affordance, and
  // keepPreviousData means a stale non-empty list never flashes to empty.
  const showLoading = isActive && query.isFetching && results.length === 0;
  const showEmpty = isActive && query.isSuccess && !query.isFetching && results.length === 0;

  return (
    <div className="bg-surface mb-3.5 rounded-lg border border-border p-3.5">
      <label
        htmlFor="player-search-input"
        className="text-ink-dim mb-2 block text-xs font-semibold"
      >
        {t("lobby.playerSearch.label")}
      </label>

      <div className="bg-surface-elevated flex items-center gap-2 rounded-[10px] border border-border px-2.5 py-1.5">
        <Search className="text-ink-mute size-3.5" />
        <input
          id="player-search-input"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={t("lobby.playerSearch.placeholder")}
          data-testid="player-search"
          className="text-ink min-w-0 flex-1 truncate bg-transparent text-sm outline-none placeholder:text-ink-off"
        />
        {value && (
          <button
            type="button"
            onClick={() => setValue("")}
            data-testid="player-search-clear"
            aria-label={t("lobby.playerSearch.clear")}
            className="text-ink-mute flex p-0.5"
          >
            <X className="size-3.5" />
          </button>
        )}
      </div>

      {isActive && results.length > 0 && (
        <ul className="mt-2 flex flex-col gap-1">
          {results.map((player) => (
            <li key={player.id}>
              <button
                type="button"
                onClick={() => navigate(`/players/${player.id}`)}
                data-testid="player-search-result"
                data-user-id={player.id}
                aria-label={t("lobby.playerSearch.resultAria", { username: player.username })}
                className="hover:bg-surface-sunken flex w-full items-center gap-2 rounded-lg border border-transparent px-2.5 py-2 text-left text-sm transition-colors hover:border-border"
              >
                <span className="bg-accent-soft text-accent flex size-7 items-center justify-center rounded-full">
                  <UserRound className="size-3.5" />
                </span>
                <span className="text-ink truncate font-medium">{player.username}</span>
              </button>
            </li>
          ))}
        </ul>
      )}

      {showLoading && (
        <p data-testid="player-search-loading" className="text-ink-mute mt-2 px-2.5 py-2 text-xs">
          {t("lobby.playerSearch.loading")}
        </p>
      )}

      {showEmpty && (
        <p data-testid="player-search-empty" className="text-ink-dim mt-2 px-2.5 py-2 text-xs">
          {t("lobby.playerSearch.empty", { query: trimmed })}
        </p>
      )}
    </div>
  );
}
