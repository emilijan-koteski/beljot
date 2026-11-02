import { DiffMarker } from "@/features/rules/components/DiffMarker";
import type { StepItem } from "@/features/rules/content/types";
import { useRules, useRulesDense } from "@/features/rules/RulesContext";

/** Short rules in one card split by hairlines: a bold line and one sentence each, optionally numbered. */
export function RuleList({ items, numbered = false }: { items: StepItem[]; numbered?: boolean }) {
  const { ui } = useRules();
  const dense = useRulesDense();
  return (
    <ol
      className="bg-surface border-border divide-border m-0 flex list-none flex-col divide-y rounded-xl border p-0 shadow-xs"
      data-testid="rules-list"
    >
      {items.map((it, i) => (
        <li key={it.t} className="flex gap-3 px-4 py-3">
          {numbered && (
            <span
              className="text-brass-deep w-3 shrink-0 font-mono text-xs leading-5 font-bold"
              data-testid="rules-step-number"
            >
              {i + 1}
            </span>
          )}
          <div className="flex min-w-0 flex-col gap-0.5">
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
