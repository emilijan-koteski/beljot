import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Avatar } from "@/shared/components/ui/avatar";

type PersonRowProps = {
  userId: number;
  username: string;
  avatarUrl: string | null;
  team: "A" | "B";
  sub: string;
  children: ReactNode;
};

/**
 * One partner or rival: avatar and name linking to their profile, a short line under the name, and a figure on the right.
 * Soft-deleted players arrive with an empty username, so they are shown without a link that would 404.
 */
export function PersonRow({ userId, username, avatarUrl, team, sub, children }: PersonRowProps) {
  const { t } = useTranslation();
  const who = (
    <>
      <Avatar name={username} avatarUrl={avatarUrl} team={team} size={32} />
      <span className="flex min-w-0 flex-col leading-tight">
        <span className="text-ink truncate text-sm font-semibold">{username}</span>
        <span className="text-ink-mute text-2xs">{sub}</span>
      </span>
    </>
  );
  return (
    <li className="flex items-center gap-2.5">
      {username === "" ? (
        <div className="flex min-w-0 flex-1 items-center gap-2.5">{who}</div>
      ) : (
        <Link
          to={`/players/${userId}`}
          aria-label={t("friends.viewProfileAria", { username })}
          className="focus-visible:ring-ring/70 flex min-w-0 flex-1 items-center gap-2.5 rounded-md underline-offset-2 hover:underline focus-visible:ring-3 focus-visible:outline-none"
        >
          {who}
        </Link>
      )}
      <span className="shrink-0">{children}</span>
    </li>
  );
}
