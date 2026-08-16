import { Send, UserRound } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { useFriends } from "@/shared/hooks/queries/useFriends";

/**
 * The viewer's friend-list surface in the lobby (Story 11.2, AC6 + AC8). Each
 * accepted friend shows an online/offline indicator and links to their public
 * profile; each ONLINE friend also shows an "Invite to Room" affordance.
 *
 * Story 11.5 resolved that parked hook. An invite is issued INTO a specific
 * waiting room (POST /rooms/:id/invite) — and a player looking at this list is,
 * by definition, in the lobby and in no room at all. So the delivery surface
 * lives where the room context exists: the waiting room's invite panel
 * (InviteFriendsDialog, opened from RoomPage's action bar).
 *
 * The button stays, because removing it would leave players hunting for a
 * feature they were shown. It now points them at the one place it can work,
 * instead of silently doing nothing.
 */
export function FriendList() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: friends, isPending } = useFriends();

  return (
    <div
      className="bg-surface border-border mb-3.5 rounded-lg border p-3.5"
      data-testid="friend-list"
    >
      <h2 className="text-ink-dim mb-2 text-xs font-semibold">{t("friends.list.heading")}</h2>

      {isPending ? (
        <p className="text-ink-mute px-2.5 py-2 text-xs">{t("friends.list.loading")}</p>
      ) : !friends || friends.length === 0 ? (
        <p className="text-ink-dim px-2.5 py-2 text-xs" data-testid="friend-list-empty">
          {t("friends.list.empty")}
        </p>
      ) : (
        <ul className="flex flex-col gap-1">
          {friends.map((friend) => (
            <li
              key={friend.id}
              className="flex items-center gap-2 rounded-lg px-2.5 py-2"
              data-testid="friend-row"
              data-user-id={friend.id}
            >
              <button
                type="button"
                onClick={() => navigate(`/players/${friend.id}`)}
                aria-label={t("friends.viewProfileAria", { username: friend.username })}
                className="hover:bg-surface-sunken flex min-w-0 flex-1 items-center gap-2 rounded-md text-left"
              >
                <span className="bg-accent-soft text-accent flex size-7 items-center justify-center rounded-full">
                  <UserRound className="size-3.5" />
                </span>
                <span className="text-ink truncate font-medium">{friend.username}</span>
              </button>

              <span className="flex items-center gap-1.5">
                {/* Go bool — compare with === true, never truthiness. */}
                <span
                  className={
                    friend.online === true
                      ? "bg-accent size-2 rounded-full"
                      : "bg-ink-off size-2 rounded-full"
                  }
                  aria-hidden="true"
                />
                <span className="text-ink-mute text-[11px]">
                  {friend.online === true ? t("friends.online") : t("friends.offline")}
                </span>
              </span>

              {friend.online === true && (
                <button
                  type="button"
                  data-testid="friend-invite-room"
                  // Story 11.5: an invite needs a room to invite INTO, and there
                  // is none in the lobby. Rather than a dead no-op, say where the
                  // action lives — the waiting room's invite panel.
                  onClick={() => toast.info(t("roomInvite.needARoom"))}
                  className="text-accent hover:bg-surface-sunken flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium"
                >
                  <Send className="size-3" />
                  {t("friends.inviteToRoom")}
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
