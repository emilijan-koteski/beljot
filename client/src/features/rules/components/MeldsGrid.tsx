import type { Declaration } from "@/features/rules/content/types";
import { useRules } from "@/features/rules/RulesContext";
import { cn } from "@/shared/lib/utils";

const TIER: Record<0 | 1 | 2 | 3, { eyebrow: string; card: string; pts: string; text: string }> = {
  0: {
    eyebrow: "text-brass-deep",
    card: "border-border bg-surface",
    pts: "text-ink",
    text: "text-ink",
  },
  1: {
    eyebrow: "text-brass-deep",
    card: "border-brass bg-surface",
    pts: "text-ink",
    text: "text-ink",
  },
  2: {
    eyebrow: "text-accent",
    card: "border-accent bg-accent/5",
    pts: "text-accent",
    text: "text-ink",
  },
  3: {
    eyebrow: "text-danger",
    card: "border-danger bg-danger/5",
    pts: "text-danger",
    text: "text-danger",
  },
};

function MeldCard({ meld }: { meld: Declaration }) {
  const { ui } = useRules();
  const tier = TIER[meld.tier];
  return (
    <div
      data-testid={`meld-${meld.id}`}
      className={cn("flex flex-col gap-2 rounded-lg border px-4 py-3.5", tier.card)}
    >
      <div className="flex items-center gap-2.5">
        <span
          className={cn(
            "min-w-0 flex-auto text-base leading-tight font-semibold tracking-tight",
            tier.text,
          )}
        >
          {meld.name}
        </span>
        <span className="inline-flex shrink-0 items-baseline gap-1">
          <span className={cn("text-2xl font-bold tracking-tight tabular-nums", tier.pts)}>
            +{meld.pts}
          </span>
          <span className="text-ink-mute text-xs">{ui.pts}</span>
        </span>
      </div>
      <div className={cn("font-mono text-2xs font-semibold tracking-caps uppercase", tier.eyebrow)}>
        {ui.meldKinds[meld.kind]}
      </div>
      <div className={cn("text-sm leading-normal", tier.text)}>{meld.summary}</div>
      <div className="text-ink-dim text-xs leading-normal">{meld.detail}</div>
    </div>
  );
}

/** Grid of every declaration (one column on phones, two on tablet+). */
export function MeldsGrid() {
  const { declarations } = useRules();
  return (
    <div data-testid="melds-grid" className="grid grid-cols-1 gap-3 md:grid-cols-2">
      {declarations.map((d) => (
        <MeldCard key={d.id} meld={d} />
      ))}
    </div>
  );
}
