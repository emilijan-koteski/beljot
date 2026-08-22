import { Bot } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Avatar } from "@/shared/components/ui/avatar";

type SeatChipProps = {
  name: string;
  team: "A" | "B";
  /** Marks this chip as the viewer with a "YOU" badge. */
  you?: boolean;
  /** Bot seat — the avatar shows the bot glyph instead of a name initial. */
  bot?: boolean;
  /** Real user's id — when set, the chip becomes a link to their public
   *  profile (`/players/:userId`). Leave unset for bot seats, missing seats
   *  and the profile subject's own chip, which stay non-interactive. */
  userId?: number;
};

/**
 * Team-tinted player pill used in match-history rows: a small team avatar +
 * username, plus an optional "YOU" badge for the viewer. Tint + edge follow the
 * viewer-relative team palette (Us = gold, Them = silver). With a `userId` it
 * renders as a client-side link to that player's public profile; the click is
 * stopped from bubbling so it never toggles the surrounding match row.
 */
export function SeatChip({ name, team, you = false, bot = false, userId }: SeatChipProps) {
  const { t } = useTranslation();
  const isA = team === "A";
  const tint = {
    border: `1px solid ${isA ? "var(--team-a-edge)" : "var(--team-b-edge)"}`,
    background: isA ? "var(--team-a-tint)" : "var(--team-b-tint)",
  };
  const content = (
    <>
      <Avatar
        name={name}
        team={team}
        size={20}
        icon={bot ? <Bot aria-hidden="true" /> : undefined}
      />
      <span className="truncate">{name}</span>
      {you && (
        <span
          className="bg-ink ml-0.5 rounded px-1.5 py-px text-[9px] font-bold tracking-[0.6px] uppercase"
          style={{ color: "var(--bg)" }}
        >
          {t("profile.matchHistory.you")}
        </span>
      )}
    </>
  );

  if (userId !== undefined) {
    return (
      <Link
        to={`/players/${userId}`}
        onClick={(e) => e.stopPropagation()}
        aria-label={t("friends.viewProfileAria", { username: name })}
        className="text-ink focus-visible:ring-ring/50 inline-flex h-7 items-center gap-1.5 rounded-lg pr-2 pl-1 text-xs font-medium underline-offset-2 hover:underline focus-visible:ring-3 focus-visible:outline-none"
        style={tint}
        data-testid="match-seat-chip"
      >
        {content}
      </Link>
    );
  }

  return (
    <span
      className="text-ink inline-flex h-7 items-center gap-1.5 rounded-lg pr-2 pl-1 text-xs font-medium"
      style={tint}
      data-testid="match-seat-chip"
    >
      {content}
    </span>
  );
}
