import { MessageCircle, Search, X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { FriendRequests } from "@/features/friends/FriendRequests";
import { PlayerSearch } from "@/features/lobby/components/PlayerSearch";
import { Avatar } from "@/shared/components/ui/avatar";
import { useFriends } from "@/shared/hooks/queries/useFriends";
import { useChatStore } from "@/shared/stores/chatStore";

/**
 * The lobby's friends card: the friend roster with presence, profile links and whisper buttons.
 * It also hosts the pending-requests section and a toggleable player search.
 */
export function FriendList() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: friends, isPending } = useFriends();
  const openWhisper = useChatStore((s) => s.openWhisper);
  const [searchOpen, setSearchOpen] = useState(false);

  return (
    <div
      className="bg-surface border-border mb-3.5 rounded-lg border p-3.5"
      data-testid="friend-list"
    >
      <div className="mb-2 flex items-center gap-2">
        <h2 className="text-ink-dim text-xs font-semibold">{t("friends.list.heading")}</h2>
        <button
          type="button"
          onClick={() => setSearchOpen((open) => !open)}
          aria-expanded={searchOpen}
          aria-label={searchOpen ? t("lobby.playerSearch.close") : t("lobby.playerSearch.label")}
          title={searchOpen ? t("lobby.playerSearch.close") : t("lobby.playerSearch.label")}
          data-testid="player-search-toggle"
          className="text-ink-mute hover:text-ink hover:bg-surface-sunken focus-visible:ring-ring/70 ml-auto flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors focus-visible:ring-3 focus-visible:outline-none"
        >
          {searchOpen ? (
            <X className="size-3.5" aria-hidden="true" />
          ) : (
            <Search className="size-3.5" aria-hidden="true" />
          )}
        </button>
      </div>

      {searchOpen && <PlayerSearch onClose={() => setSearchOpen(false)} />}

      <FriendRequests />

      {isPending ? (
        <p className="text-ink-mute px-2.5 py-2 text-xs">{t("friends.list.loading")}</p>
      ) : !friends || friends.length === 0 ? (
        <p className="text-ink-dim px-2.5 py-2 text-xs" data-testid="friend-list-empty">
          {t("friends.list.empty")}
        </p>
      ) : (
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] list-none gap-1.5 p-0">
          {friends.map((friend) => (
            <li
              key={friend.id}
              className="bg-surface-elevated border-border flex items-center gap-2 rounded-md border px-2.5 py-1.5"
              data-testid="friend-row"
              data-user-id={friend.id}
            >
              <button
                type="button"
                onClick={() => navigate(`/players/${friend.id}`)}
                aria-label={t("friends.viewProfileAria", { username: friend.username })}
                className="hover:bg-surface-sunken focus-visible:ring-ring/70 flex min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-md text-left transition-colors focus-visible:ring-3 focus-visible:outline-none"
              >
                <Avatar
                  name={friend.username}
                  avatarUrl={friend.avatarUrl}
                  size={28}
                  className="shrink-0"
                />
                <span className="text-ink truncate text-sm font-medium">{friend.username}</span>
              </button>

              <span
                className="flex shrink-0 items-center"
                title={friend.online === true ? t("friends.online") : t("friends.offline")}
              >
                <span
                  className={
                    friend.online === true
                      ? "bg-accent size-2 rounded-full"
                      : "bg-faint size-2 rounded-full"
                  }
                  aria-hidden="true"
                />
                <span className="sr-only">
                  {friend.online === true ? t("friends.online") : t("friends.offline")}
                </span>
              </span>

              {friend.online === true && (
                <button
                  type="button"
                  data-testid="friend-whisper"
                  onClick={() => openWhisper(friend.username)}
                  aria-label={t("friends.whisperAria", { username: friend.username })}
                  title={t("friends.whisperAria", { username: friend.username })}
                  className="focus-visible:ring-ring/70 flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-md text-(--whisper-ink) transition-colors hover:bg-(--whisper-fill) focus-visible:ring-3 focus-visible:outline-none"
                >
                  <MessageCircle className="size-4" aria-hidden="true" />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
