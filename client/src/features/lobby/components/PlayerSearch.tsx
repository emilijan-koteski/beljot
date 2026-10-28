import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { Avatar } from "@/shared/components/ui/avatar";
import { SearchInput } from "@/shared/components/ui/search-input";
import { useUserSearch } from "@/shared/hooks/queries/useUserSearch";
import { useDebounce } from "@/shared/hooks/useDebounce";

type PlayerSearchProps = {
  onClose?: () => void;
};

/**
 * Debounced live player search, rendered as a panel inside the Friends card.
 * Each result is a button that opens the player's public profile.
 */
export function PlayerSearch({ onClose }: PlayerSearchProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [value, setValue] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const debounced = useDebounce(value);
  const trimmed = debounced.trim();
  const isActive = trimmed.length >= 2;

  const query = useUserSearch(debounced);
  const results = query.data ?? [];

  const showLoading = isActive && query.isFetching && results.length === 0;
  const showEmpty = isActive && query.isSuccess && !query.isFetching && results.length === 0;

  return (
    <div
      className="border-border mb-2 flex flex-col border-b pb-2 motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-top-1 motion-safe:duration-150"
      data-testid="player-search-panel"
    >
      <SearchInput
        ref={inputRef}
        value={value}
        onValueChange={setValue}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            setValue("");
            onClose?.();
          }
        }}
        placeholder={t("lobby.playerSearch.placeholder")}
        aria-label={t("lobby.playerSearch.label")}
        data-testid="player-search"
      />

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
                <Avatar
                  name={player.username}
                  avatarUrl={player.avatarUrl}
                  size={28}
                  className="shrink-0"
                />
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
