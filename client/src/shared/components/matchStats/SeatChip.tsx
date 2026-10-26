import { Bot } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Avatar } from "@/shared/components/ui/avatar";

type SeatChipProps = {
  name: string;
  team: "A" | "B";
  you?: boolean;
  bot?: boolean;
  avatarUrl?: string | null;
  userId?: number;
};

/**
 * Team-tinted player pill with an optional "YOU" badge.
 * With a `userId` it links to the player's public profile without toggling the surrounding row.
 */
export function SeatChip({
  name,
  team,
  you = false,
  bot = false,
  avatarUrl,
  userId,
}: SeatChipProps) {
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
        avatarUrl={bot ? null : avatarUrl}
        team={team}
        size={20}
        icon={bot ? <Bot aria-hidden="true" /> : undefined}
      />
      <span className="truncate">{name}</span>
      {you && (
        <span
          className="bg-ink ml-0.5 rounded px-1.5 py-px text-2xs font-bold tracking-wider uppercase"
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
        className="text-ink focus-visible:ring-ring/70 inline-flex h-7 items-center gap-1.5 rounded-lg pr-2 pl-1 text-xs font-medium underline-offset-2 hover:underline focus-visible:ring-3 focus-visible:outline-none"
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
