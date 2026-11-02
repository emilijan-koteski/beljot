import { Megaphone } from "lucide-react";
import type { ReactNode } from "react";

import { useRules } from "@/features/rules/RulesContext";

const CARD_POINTS = 152;
const LAST_TRICK_BONUS = 10;

function Term({ value, label }: { value: ReactNode; label: string }) {
  return (
    <div className="flex flex-1 flex-col items-center gap-0.5 py-2.5">
      <span className="text-ink flex h-5 items-center text-xl leading-none font-bold tabular-nums">
        {value}
      </span>
      <span className="text-ink-mute text-2xs">{label}</span>
    </div>
  );
}

function Plus() {
  return (
    <span className="text-ink-mute self-center text-sm font-semibold" aria-hidden="true">
      +
    </span>
  );
}

/** What one hand is worth: the card points, the last-trick bonus and the declarations, as a sum. */
export function ScoreSum() {
  const { ui } = useRules();
  return (
    <div
      className="bg-surface border-border flex items-stretch rounded-xl border shadow-xs"
      data-testid="rules-score-sum"
    >
      <Term value={CARD_POINTS} label={ui.sumCards} />
      <Plus />
      <Term value={LAST_TRICK_BONUS} label={ui.sumLastTrick} />
      <Plus />
      <Term
        value={<Megaphone className="text-brass-deep size-5" aria-hidden="true" />}
        label={ui.sumDeclarations}
      />
    </div>
  );
}
