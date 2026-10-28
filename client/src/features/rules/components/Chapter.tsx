import { RuleBlock } from "@/features/rules/components/RuleBlock";
import type { RuleSection } from "@/features/rules/content/types";
import { useRulesDense } from "@/features/rules/RulesContext";
import { cn } from "@/shared/lib/utils";

type Props = {
  idx: number;
  section: RuleSection;
  registerRef: (id: string, el: HTMLElement | null) => void;
};

/** One numbered chapter (§01…§06): header + content blocks. */
export function Chapter({ idx, section, registerRef }: Props) {
  const dense = useRulesDense();
  return (
    <section
      ref={(el) => registerRef(section.id, el)}
      id={section.id}
      className={cn(
        "flex flex-col",
        dense ? "scroll-mt-2 gap-3.5 pt-5.5 pb-5" : "scroll-mt-21 gap-4.5 pt-9 pb-8",
        idx > 0 && "border-border border-t",
      )}
    >
      <header className={cn("flex flex-col", dense ? "gap-1" : "gap-1.5")}>
        <div className="text-brass-deep font-mono text-2xs font-semibold tracking-caps uppercase">
          {String(idx + 1).padStart(2, "0")} &nbsp; {section.label}
        </div>
        <h2
          className={cn(
            "text-ink m-0 leading-tight tracking-tight",
            dense ? "text-2xl font-semibold" : "text-3xl font-bold",
          )}
        >
          {section.title}
        </h2>
        <p
          className={cn(
            "text-ink-dim mt-1 mb-0 max-w-160 leading-relaxed",
            dense ? "text-sm" : "text-base",
          )}
        >
          {section.lede}
        </p>
      </header>

      <div className={cn("flex flex-col", dense ? "gap-3" : "gap-3.5")}>
        {section.blocks.map((b, i) => (
          <RuleBlock key={i} block={b} />
        ))}
      </div>
    </section>
  );
}
