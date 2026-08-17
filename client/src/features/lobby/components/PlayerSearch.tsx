import { Search } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { Avatar } from "@/shared/components/ui/avatar";
import { useUserSearch } from "@/shared/hooks/queries/useUserSearch";
import { useDebounce } from "@/shared/hooks/useDebounce";

type PlayerSearchProps = {
  /**
   * Collapses the search back to its icon button. Called on Escape — the host
   * (FriendList) owns the open/closed state and renders the toggle, so this
   * component is only ever mounted while the search is open.
   */
  onClose?: () => void;
};

/**
 * Live player search (Story 11.1, FR5). Owns the raw input value, debounces it,
 * and feeds the debounced term to useUserSearch. Results are keyboard-accessible
 * buttons that navigate to the player's public profile (/players/:id — the route
 * and page were delivered by Story 11.3).
 *
 * No card chrome of its own: it is a panel INSIDE the Friends card, opened from
 * that card's header. Finding a player and befriending them are one task, and
 * this used to be a separate full-width card sitting above the friends surfaces
 * for a field that is empty almost all of the time.
 */
export function PlayerSearch({ onClose }: PlayerSearchProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [value, setValue] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  // The search is mounted by the toggle, so the caret belongs in the field the
  // click just revealed — otherwise every open costs a second click. A ref +
  // effect rather than `autoFocus`, which fires before layout and is a lint
  // no-no for its unconditional behaviour in reusable components.
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

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
    <div
      className="border-border mb-2 flex flex-col border-b pb-2 motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-top-1 motion-safe:duration-150"
      data-testid="player-search-panel"
    >
      <div className="bg-surface-elevated border-border flex items-center gap-2 rounded-[10px] border px-2.5 py-1.5">
        <Search className="text-ink-mute size-3.5 shrink-0" aria-hidden="true" />
        <input
          ref={inputRef}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          // Escape is the keyboard twin of the header's close button: it drops the
          // term AND collapses the panel, so the field never stays open holding a
          // query the player has walked away from.
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              setValue("");
              onClose?.();
            }
          }}
          placeholder={t("lobby.playerSearch.placeholder")}
          aria-label={t("lobby.playerSearch.label")}
          data-testid="player-search"
          className="text-ink placeholder:text-ink-off min-w-0 flex-1 truncate bg-transparent text-sm outline-none"
        />
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
                className="hover:bg-surface-sunken hover:border-border flex w-full items-center gap-2 rounded-lg border border-transparent px-2.5 py-2 text-left text-sm transition-colors"
              >
                {/* Same initial-in-circle as the friend and request tiles: one
                    avatar grammar across the whole card, and a letter identifies
                    the player a generic person glyph cannot. */}
                <Avatar name={player.username} size={28} className="shrink-0" />
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
