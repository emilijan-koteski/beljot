import type { Suit } from "@/features/rules/content/types";
import { SUIT_GLYPH, SUIT_IS_RED } from "@/features/rules/suits";
import { cn } from "@/shared/lib/utils";

type Props = {
  suit: Suit;
  rank?: string;
  className?: string;
};

/** A tiny playing card, the rank over its suit pip, in the card-face colours whatever the page theme. */
export function MiniCard({ suit, rank, className }: Props) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "bg-(--card-face) border-(--card-face-edge) inline-flex h-6.5 min-w-4.5 shrink-0 flex-col items-center justify-center rounded-xs border px-0.5 text-2xs leading-none font-bold shadow-xs",
        SUIT_IS_RED[suit] ? "text-(--suit-red)" : "text-(--ink-on-card)",
        className,
      )}
    >
      {rank !== undefined && <span>{rank}</span>}
      <span>{SUIT_GLYPH[suit]}</span>
    </span>
  );
}
