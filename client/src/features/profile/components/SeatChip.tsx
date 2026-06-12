import { Bot } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Avatar } from "@/shared/components/ui/avatar";

type SeatChipProps = {
  name: string;
  team: "A" | "B";
  /** Marks this chip as the viewer with a "YOU" badge. */
  you?: boolean;
  /** Bot seat — the avatar shows the bot glyph instead of a name initial. */
  bot?: boolean;
};

/**
 * Team-tinted player pill used in match-history rows: a small team avatar +
 * username, plus an optional "YOU" badge for the viewer. Tint + edge follow the
 * viewer-relative team palette (Us = gold, Them = silver).
 */
export function SeatChip({ name, team, you = false, bot = false }: SeatChipProps) {
  const { t } = useTranslation();
  const isA = team === "A";
  return (
    <span
      className="text-ink inline-flex h-7 items-center gap-1.5 rounded-lg pr-2 pl-1 text-xs font-medium"
      style={{
        border: `1px solid ${isA ? "var(--team-a-edge)" : "var(--team-b-edge)"}`,
        background: isA ? "var(--team-a-tint)" : "var(--team-b-tint)",
      }}
      data-testid="match-seat-chip"
    >
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
    </span>
  );
}
