import { DiffMarker } from "@/features/rules/components/DiffMarker";
import type { StepItem } from "@/features/rules/content/types";
import { useRules, useRulesDense } from "@/features/rules/RulesContext";

/** Numbered steps on a thin rail, so a sequence like the deal reads top to bottom without a card per step. */
export function Timeline({ items }: { items: StepItem[] }) {
  const { ui } = useRules();
  const dense = useRulesDense();
  return (
    <ol className="m-0 flex list-none flex-col p-0" data-testid="rules-timeline">
      {items.map((it, i) => (
        <li
          key={it.t}
          className="relative grid grid-cols-[2rem_minmax(0,1fr)] gap-3 pb-4 last:pb-0"
        >
          {i < items.length - 1 && (
            <span className="bg-border absolute top-8 bottom-0 left-4 w-px" aria-hidden="true" />
          )}
          <span
            className="border-brass text-brass-deep bg-surface relative flex size-8 items-center justify-center rounded-full border font-mono text-xs font-bold"
            data-testid="rules-step-number"
          >
            {i + 1}
          </span>
          <div className="flex min-w-0 flex-col gap-0.5 pt-1.5">
            <span className="text-ink text-sm font-semibold">
              {it.t}
              {it.otherVariantNote !== undefined && (
                <DiffMarker
                  note={it.otherVariantNote}
                  label={ui.diffLabel}
                  tone={dense ? "dark" : "light"}
                />
              )}
            </span>
            <span className="text-ink-dim text-sm leading-relaxed">{it.d}</span>
          </div>
        </li>
      ))}
    </ol>
  );
}
