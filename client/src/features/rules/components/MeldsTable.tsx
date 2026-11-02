import { MiniCard } from "@/features/rules/components/MiniCard";
import type { Declaration, Suit } from "@/features/rules/content/types";
import { useRules, useRulesDense } from "@/features/rules/RulesContext";
import { cn } from "@/shared/lib/utils";

const SET_SUITS: Suit[] = ["S", "H", "D", "C"];
const TRUMP_COUNT = 8;

function MeldCards({ meld }: { meld: Declaration }) {
  if (meld.wins) {
    return (
      <span className="flex shrink-0 items-center gap-1">
        <MiniCard suit={meld.suit} />
        <span className="text-brass-deep font-mono text-xs font-bold">×{TRUMP_COUNT}</span>
      </span>
    );
  }
  return (
    <span className="flex shrink-0 gap-0.5">
      {meld.cards.map((rank, i) => (
        <MiniCard key={i} rank={rank} suit={meld.set ? (SET_SUITS[i] ?? meld.suit) : meld.suit} />
      ))}
    </span>
  );
}

/**
 * Every declaration in one table: its points, its name and a picture of its cards, with a few words only
 * where the cards cannot say it. Wide layouts split it into two columns.
 */
export function MeldsTable() {
  const { declarations, ui } = useRules();
  const dense = useRulesDense();
  return (
    <ul
      data-testid="melds-grid"
      className={cn(
        "bg-surface border-border m-0 grid list-none overflow-hidden rounded-xl border p-0 shadow-xs",
        !dense && "md:grid-cols-2",
      )}
    >
      {declarations.map((m, i) => (
        <li
          key={m.id}
          data-testid={`meld-${m.id}`}
          className={cn(
            "border-border flex items-center gap-3 px-3.5 py-2.5",
            i > 0 && "border-t",
            !dense && i === 1 && "md:border-t-0",
            !dense && i % 2 === 1 && "md:border-l",
            m.wins && "bg-brass/10",
          )}
        >
          <span
            className={cn(
              "w-14 shrink-0 text-right font-bold tabular-nums",
              m.wins ? "text-brass-deep text-sm" : "text-ink text-base",
            )}
          >
            {m.wins ? ui.meldWin : `+${m.pts}`}
          </span>
          <span className="flex min-w-0 flex-1 flex-col gap-0.5">
            <span className="text-ink text-sm leading-tight font-semibold">{m.name}</span>
            {m.summary !== undefined && (
              <span className="text-ink-mute text-xs leading-snug">{m.summary}</span>
            )}
          </span>
          <MeldCards meld={m} />
        </li>
      ))}
    </ul>
  );
}
