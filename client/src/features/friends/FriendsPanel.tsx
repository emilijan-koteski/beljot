import { ChevronRight, MessageCircle } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { FriendRequests } from "@/features/friends/FriendRequests";
import {
  PLAYER_SEARCH_MIN_LENGTH,
  PlayerSearchResults,
} from "@/features/friends/PlayerSearchResults";
import { EmptyState } from "@/shared/components/EmptyState";
import { LoadingLine } from "@/shared/components/LoadingLine";
import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { SearchInput } from "@/shared/components/ui/search-input";
import { useFriends } from "@/shared/hooks/queries/useFriends";
import { cn } from "@/shared/lib/utils";
import { useChatStore } from "@/shared/stores/chatStore";
import type { Friend } from "@/shared/types/apiTypes";

function onlineFirst(a: Friend, b: Friend): number {
  return (
    Number(b.online === true) - Number(a.online === true) || a.username.localeCompare(b.username)
  );
}

/** One friend row opening their profile: avatar, name and presence, a whisper button while online, then a chevron. */
function FriendRow({
  friend,
  onWhisper,
}: {
  friend: Friend;
  onWhisper: (username: string) => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const online = friend.online === true;
  return (
    <li
      className="hover:bg-surface-sunken relative flex items-center gap-2.5 py-1.5"
      data-testid="friend-row"
      data-user-id={friend.id}
    >
      <button
        type="button"
        onClick={() => navigate(`/players/${friend.id}`)}
        aria-label={t("friends.viewProfileAria", { username: friend.username })}
        className="focus-visible:ring-ring/70 absolute inset-0 cursor-pointer rounded-md outline-none focus-visible:ring-3"
      />
      <Avatar name={friend.username} avatarUrl={friend.avatarUrl} size={32} />
      <span className="flex min-w-0 flex-1 flex-col leading-tight">
        <span className="text-ink truncate text-sm font-semibold">{friend.username}</span>
        <span className="text-ink-mute inline-flex items-center gap-1.5 text-2xs">
          <span
            className={cn("size-1.5 rounded-full", online ? "bg-accent" : "bg-faint")}
            aria-hidden="true"
          />
          {online ? t("friends.online") : t("friends.offline")}
        </span>
      </span>
      {online && (
        <Button
          variant="ghost"
          size="icon-sm"
          data-testid="friend-whisper"
          onClick={() => onWhisper(friend.username)}
          aria-label={t("friends.whisperAria", { username: friend.username })}
          title={t("friends.whisperAria", { username: friend.username })}
          className="relative z-10 text-(--whisper-ink) hover:bg-(--whisper-fill) hover:text-(--whisper-ink)"
        >
          <MessageCircle aria-hidden="true" />
        </Button>
      )}
      <ChevronRight className="text-ink-mute size-4" aria-hidden="true" />
    </li>
  );
}

/** The viewer's friends, online first, or a line saying there are none yet. */
function FriendRoster({ onWhisper }: { onWhisper: (username: string) => void }) {
  const { t } = useTranslation();
  const { data: friends, isPending } = useFriends();
  if (isPending) return <LoadingLine>{t("friends.list.loading")}</LoadingLine>;
  if (!friends || friends.length === 0) {
    return (
      <EmptyState compact testId="friend-list-empty">
        {t("friends.list.empty")}
      </EmptyState>
    );
  }
  const online = friends.filter((f) => f.online === true).length;
  return (
    <section>
      <Eyebrow tone="mute" className="mb-1.5">
        {t("friends.list.groupLabel", { count: online })}
      </Eyebrow>
      <ul className="divide-border m-0 list-none divide-y p-0" data-testid="friend-list">
        {[...friends].sort(onlineFirst).map((friend) => (
          <FriendRow key={friend.id} friend={friend} onWhisper={onWhisper} />
        ))}
      </ul>
    </section>
  );
}

interface FriendsPanelProps {
  onWhisper?: (username: string) => void;
}

/**
 * The friends content shared by the phone and tablet sheet and the desktop sidebar: find any player,
 * then pending requests and friends online first. Whispering opens the chat dock on that friend's thread.
 */
export function FriendsPanel({ onWhisper }: FriendsPanelProps) {
  const { t } = useTranslation();
  const openWhisper = useChatStore((s) => s.openWhisper);
  const [query, setQuery] = useState("");
  const searching = query.trim().length >= PLAYER_SEARCH_MIN_LENGTH;

  function whisper(username: string) {
    openWhisper(username);
    onWhisper?.(username);
  }

  return (
    <div className="flex flex-col gap-4" data-testid="friends-panel">
      <SearchInput
        value={query}
        onValueChange={setQuery}
        placeholder={t("lobby.playerSearch.placeholder")}
        aria-label={t("lobby.playerSearch.label")}
        clearLabel={t("lobby.playerSearch.close")}
        data-testid="player-search"
      />
      {searching ? (
        <PlayerSearchResults query={query} />
      ) : (
        <>
          <FriendRequests />
          <FriendRoster onWhisper={whisper} />
        </>
      )}
    </div>
  );
}
