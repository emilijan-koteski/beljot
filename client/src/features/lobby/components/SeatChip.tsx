import { Bot } from "lucide-react";
import { useTranslation } from "react-i18next";

import { AvatarImage } from "@/shared/components/ui/avatar";
import { cn } from "@/shared/lib/utils";

type SeatTeam = "A" | "B";

type Props = {
  username: string | null;
  team: SeatTeam;
  bot?: boolean;
  avatarUrl?: string | null;
  testId?: string;
};

const TOKENS = {
  A: {
    edge: "border-[var(--team-a-edge)]",
    edgeSoft: "border-[var(--team-a-edge-soft)]",
    tint: "bg-[var(--team-a-tint)]",
    line: "bg-[var(--team-a-line)]",
    fill: "bg-[var(--team-a-fill)]",
  },
  B: {
    edge: "border-[var(--team-b-edge)]",
    edgeSoft: "border-[var(--team-b-edge-soft)]",
    tint: "bg-[var(--team-b-tint)]",
    line: "bg-[var(--team-b-line)]",
    fill: "bg-[var(--team-b-fill)]",
  },
} as const;

/**
 * One seat in the room card's 2×2 team grid: a dashed outline when empty.
 * A filled seat shows the avatar, bot glyph, or initial in a team-coloured disc beside the name.
 */
export function SeatChip({ username, team, bot = false, avatarUrl, testId }: Props) {
  const { t } = useTranslation();
  const tone = TOKENS[team];

  if (!username) {
    return (
      <span
        data-testid={testId}
        className={cn(
          "text-ink-off inline-flex h-7.5 items-center justify-center rounded-lg border border-dashed px-2 text-[11px] tracking-[0.5px]",
          tone.edgeSoft,
          tone.line,
        )}
      >
        {t("lobby.card.empty", { defaultValue: "empty" })}
      </span>
    );
  }
  return (
    <span
      data-testid={testId}
      className={cn(
        "text-ink inline-flex h-7.5 items-center gap-1.5 overflow-hidden rounded-lg border pr-2 pl-1 text-xs",
        tone.edge,
        tone.tint,
      )}
    >
      <span
        className={cn(
          "inline-flex size-4.5 shrink-0 items-center justify-center overflow-hidden rounded-full text-[10px] font-bold text-brass-ink",
          tone.fill,
        )}
        data-testid={bot ? "seat-chip-bot-icon" : undefined}
      >
        {bot ? (
          <Bot className="size-3 -translate-y-[5%]" aria-hidden="true" />
        ) : (
          <AvatarImage src={avatarUrl} size={18} fallback={username.charAt(0).toUpperCase()} />
        )}
      </span>
      <span className="truncate">{username}</span>
    </span>
  );
}
