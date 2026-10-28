import type { Fact as FactData } from "@/features/rules/content/types";
import { useRules } from "@/features/rules/RulesContext";

function Fact({ label, value, caption }: FactData) {
  return (
    <div className="bg-surface border-border flex min-w-35 flex-[1_1_140px] flex-col gap-0.5 rounded-lg border px-3.5 py-3">
      <span className="text-brass-deep font-mono text-2xs font-bold tracking-caps uppercase">
        {label}
      </span>
      <span className="text-ink text-3xl leading-tight font-bold tracking-tight tabular-nums">
        {value}
      </span>
      <span className="text-ink-dim text-xs">{caption}</span>
    </div>
  );
}

/** Page intro: eyebrow, title, lede, and the quick-facts row. */
export function RulesHero() {
  const { ui } = useRules();
  return (
    <header className="flex flex-col gap-4.5 pb-7">
      <div className="text-brass-deep font-mono text-2xs font-semibold tracking-caps uppercase">
        {ui.heroEyebrow}
      </div>

      <h1 className="text-ink m-0 text-5xl leading-none font-bold tracking-tight">
        {ui.heroTitle}
      </h1>

      <p className="text-ink-dim m-0 max-w-160 text-lg leading-relaxed">{ui.heroIntro}</p>

      <div className="flex flex-wrap gap-2.5 pt-1">
        {ui.facts.map((f, i) => (
          <Fact key={i} label={f.label} value={f.value} caption={f.caption} />
        ))}
      </div>
    </header>
  );
}
