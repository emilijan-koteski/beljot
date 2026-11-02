import { MiniCard } from "@/features/rules/components/MiniCard";
import { PLAIN_SUIT, TRUMP_SUIT } from "@/features/rules/content/shared";
import type { CardRow, Suit } from "@/features/rules/content/types";
import { useRules } from "@/features/rules/RulesContext";
import { SUIT_GLYPH, SUIT_IS_RED } from "@/features/rules/suits";
import { cn } from "@/shared/lib/utils";

type ColumnProps = {
  title: string;
  suit: Suit;
  rows: CardRow[];
  testId: string;
};

function Column({ title, suit, rows, testId }: ColumnProps) {
  return (
    <div className="flex min-w-0 flex-col" data-testid={testId}>
      <div className="text-ink border-border flex items-center gap-1.5 border-b px-3 py-2 text-xs font-semibold">
        <span className={SUIT_IS_RED[suit] ? "text-(--suit-red)" : "text-ink"} aria-hidden="true">
          {SUIT_GLYPH[suit]}
        </span>
        {title}
      </div>
      <ol className="m-0 flex list-none flex-col p-0">
        {rows.map((r) => {
          const top = r.strength === 1;
          return (
            <li
              key={r.rank}
              className={cn("flex items-center gap-2 px-3 py-1.5", top && "bg-brass/10")}
            >
              <MiniCard rank={r.rank} suit={suit} />
              <span
                className={cn("text-ink min-w-0 flex-1 truncate text-sm", top && "font-semibold")}
              >
                {r.name}
              </span>
              <span
                className={cn(
                  "font-mono text-sm tabular-nums",
                  r.pts > 0 ? "text-ink font-semibold" : "text-ink-mute",
                )}
              >
                {r.pts}
              </span>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

/** Card values for trump and the other suits side by side, strongest first, with the deck total under them. */
export function CardTable() {
  const { cardsTrump, cardsPlain, ui } = useRules();
  return (
    <div className="flex flex-col gap-2">
      <div className="bg-surface border-border divide-border grid grid-cols-2 divide-x overflow-hidden rounded-xl border shadow-xs">
        <Column title={ui.cardsTrump} suit={TRUMP_SUIT} rows={cardsTrump} testId="ladder-trump" />
        <Column title={ui.cardsOther} suit={PLAIN_SUIT} rows={cardsPlain} testId="ladder-plain" />
      </div>
      <p className="text-ink-mute m-0 px-1 text-xs">{ui.cardsFoot}</p>
    </div>
  );
}
