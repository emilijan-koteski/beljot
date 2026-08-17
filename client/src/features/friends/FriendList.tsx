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
 * The viewer's friend-list surface in the lobby (Story 11.2, AC6 + AC8). Each
 * accepted friend shows an online/offline indicator and links to their public
 * profile; each ONLINE friend also gets a whisper button.
 *
 * That slot used to hold "Invite to Room", which could not do its job from here:
 * an invite goes INTO a specific waiting room (POST /rooms/:id/invite) and a
 * player reading this list is in the lobby, in no room at all, so the button only
 * ever raised a toast pointing at the room's own invite panel. Whisper is the
 * action this surface CAN complete — the friend is right there, the dock is on
 * this page — so the slot now opens a private thread with them (Story 11.4's
 * channel, entered from the friend instead of from a /w command).
 *
 * This card is the lobby's WHOLE friends surface: the pending-requests section
 * (FriendRequests, which draws nothing when the inbox is empty) and player search
 * both live inside it. Three cards for one relationship spent most of their area
 * on chrome — heading, border and padding each — to show a handful of names.
 * Search hides behind one header icon because the field is empty almost always;
 * this component holds its open/closed state, and mounting PlayerSearch only when
 * open is also what puts the caret in the field on the way in.
 *
 * Names TILE into an auto-filling grid instead of one row each: a lobby is wide,
 * a name is narrow, and a stack of full-width rows is mostly empty parchment.
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
        {/* One control, two states — open the search, or close it (which also
            drops the term). Escape inside the field does the same thing. */}
        <button
          type="button"
          onClick={() => setSearchOpen((open) => !open)}
          aria-expanded={searchOpen}
          aria-label={searchOpen ? t("lobby.playerSearch.close") : t("lobby.playerSearch.label")}
          title={searchOpen ? t("lobby.playerSearch.close") : t("lobby.playerSearch.label")}
          data-testid="player-search-toggle"
          className="text-ink-mute hover:text-ink hover:bg-surface-sunken focus-visible:ring-ring/50 ml-auto flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors focus-visible:ring-3 focus-visible:outline-none"
        >
          {searchOpen ? (
            <X className="size-3.5" aria-hidden="true" />
          ) : (
            <Search className="size-3.5" aria-hidden="true" />
          )}
        </button>
      </div>

      {searchOpen && <PlayerSearch onClose={() => setSearchOpen(false)} />}

      {/* Pending requests sit above the roster, under their own eyebrow: they are
          the part of this card that wants an answer. Draws nothing when empty. */}
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
              className="bg-surface-elevated border-border flex items-center gap-2 rounded-[10px] border px-2.5 py-1.5"
              data-testid="friend-row"
              data-user-id={friend.id}
            >
              <button
                type="button"
                onClick={() => navigate(`/players/${friend.id}`)}
                aria-label={t("friends.viewProfileAria", { username: friend.username })}
                className="hover:bg-surface-sunken focus-visible:ring-ring/50 flex min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-md text-left transition-colors focus-visible:ring-3 focus-visible:outline-none"
              >
                <Avatar name={friend.username} size={28} className="shrink-0" />
                <span className="text-ink truncate text-sm font-medium">{friend.username}</span>
              </button>

              {/* The dot alone carries presence in a tile this narrow — green for
                  here, grey for away is the one convention nobody has to learn.
                  The word survives as the hover title and for screen readers, so
                  nothing is lost but the width it was costing. */}
              <span
                className="flex shrink-0 items-center"
                title={friend.online === true ? t("friends.online") : t("friends.offline")}
              >
                {/* Go bool — compare with === true, never truthiness. */}
                <span
                  className={
                    friend.online === true
                      ? "bg-accent size-2 rounded-full"
                      : "bg-ink-off size-2 rounded-full"
                  }
                  aria-hidden="true"
                />
                <span className="sr-only">
                  {friend.online === true ? t("friends.online") : t("friends.offline")}
                </span>
              </span>

              {/* Whisper, as a bare glyph in the pink the whisper thread itself
                  wears (Story 11.4's --whisper-* tokens) — the icon is the colour
                  of the thing it opens, and it takes the solid ink tone rather
                  than the translucent bubble fill, which at this size read as a
                  smudge. Online-only, like the invite button it replaces: the
                  server rejects a whisper to an offline friend (whispers are
                  real-time), so an always-on button would fail by design. */}
              {friend.online === true && (
                <button
                  type="button"
                  data-testid="friend-whisper"
                  onClick={() => openWhisper(friend.username)}
                  aria-label={t("friends.whisperAria", { username: friend.username })}
                  title={t("friends.whisperAria", { username: friend.username })}
                  className="focus-visible:ring-ring/50 flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-md text-(--whisper-name) transition-colors hover:text-(--whisper-ink) focus-visible:ring-3 focus-visible:outline-none"
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
