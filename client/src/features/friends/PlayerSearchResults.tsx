import { ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { EmptyState } from "@/shared/components/EmptyState";
import { LoadingLine } from "@/shared/components/LoadingLine";
import { Avatar } from "@/shared/components/ui/avatar";
import { useUserSearch } from "@/shared/hooks/queries/useUserSearch";
import { useDebounce } from "@/shared/hooks/useDebounce";

export const PLAYER_SEARCH_MIN_LENGTH = 2;

/** Live results for a player search, each opening the player's profile; quiet until two characters are typed. */
export function PlayerSearchResults({ query }: { query: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const debounced = useDebounce(query);
  const trimmed = debounced.trim();
  const search = useUserSearch(debounced);
  const results = search.data ?? [];

  if (trimmed.length < PLAYER_SEARCH_MIN_LENGTH) return null;

  if (results.length === 0) {
    return search.isFetching ? (
      <LoadingLine testId="player-search-loading">{t("lobby.playerSearch.loading")}</LoadingLine>
    ) : (
      <EmptyState compact testId="player-search-empty">
        {t("lobby.playerSearch.empty", { query: trimmed })}
      </EmptyState>
    );
  }

  return (
    <ul className="divide-border m-0 list-none divide-y p-0" data-testid="player-search-results">
      {results.map((player) => (
        <li key={player.id}>
          <button
            type="button"
            onClick={() => navigate(`/players/${player.id}`)}
            data-testid="player-search-result"
            data-user-id={player.id}
            aria-label={t("lobby.playerSearch.resultAria", { username: player.username })}
            className="hover:bg-surface-sunken focus-visible:ring-ring/70 flex w-full cursor-pointer items-center gap-2.5 rounded-md py-1.5 text-left outline-none focus-visible:ring-3"
          >
            <Avatar name={player.username} avatarUrl={player.avatarUrl} size={32} />
            <span className="text-ink min-w-0 flex-1 truncate text-sm font-semibold">
              {player.username}
            </span>
            <ChevronRight className="text-ink-mute size-4" aria-hidden="true" />
          </button>
        </li>
      ))}
    </ul>
  );
}
