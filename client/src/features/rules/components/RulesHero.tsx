import { useRules } from "@/features/rules/RulesContext";
import { Eyebrow } from "@/shared/components/ui/eyebrow";

/** Page intro: eyebrow, title, one line of intro and the four quick facts in one strip. */
export function RulesHero() {
  const { ui } = useRules();
  return (
    <header className="flex flex-col gap-3">
      <Eyebrow>{ui.heroEyebrow}</Eyebrow>
      <h1 className="text-ink m-0 text-3xl font-semibold tracking-tight md:text-4xl">
        {ui.heroTitle}
      </h1>
      <p className="text-ink-dim m-0 max-w-140 text-sm leading-relaxed md:text-base">
        {ui.heroIntro}
      </p>
      <dl
        className="bg-surface border-border divide-border m-0 mt-1 grid grid-cols-4 divide-x rounded-xl border shadow-xs"
        data-testid="rules-facts"
      >
        {ui.facts.map((f) => (
          <div key={f.label} className="flex flex-col-reverse items-center gap-0.5 py-2.5">
            <dt className="text-ink-mute text-2xs">{f.label}</dt>
            <dd className="text-ink m-0 text-xl leading-none font-bold tabular-nums">{f.value}</dd>
          </div>
        ))}
      </dl>
    </header>
  );
}
