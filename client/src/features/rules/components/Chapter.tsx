import { CHAPTER_ICON } from "@/features/rules/chapterIcons";
import { RuleBlock } from "@/features/rules/components/RuleBlock";
import type { RuleSection } from "@/features/rules/content/types";
import { useRulesDense } from "@/features/rules/RulesContext";
import { cn } from "@/shared/lib/utils";

type Props = {
  section: RuleSection;
  registerRef: (id: string, el: HTMLElement | null) => void;
};

/** One chapter: its icon, title and one-line lede over its content blocks. */
export function Chapter({ section, registerRef }: Props) {
  const dense = useRulesDense();
  const Icon = CHAPTER_ICON[section.id];
  return (
    <section
      ref={(el) => registerRef(section.id, el)}
      id={section.id}
      aria-labelledby={`rules-chapter-${section.id}`}
      className={cn(
        "flex flex-col gap-3",
        dense ? "scroll-mt-2 pt-5 pb-2" : "scroll-mt-32 lg:scroll-mt-24",
      )}
    >
      <header className="flex flex-col gap-1">
        <h2
          id={`rules-chapter-${section.id}`}
          className={cn(
            "text-ink m-0 flex items-center gap-2.5 font-semibold tracking-tight",
            dense ? "text-lg" : "text-xl md:text-2xl",
          )}
        >
          <span className="bg-brass-soft text-brass-deep flex size-8 shrink-0 items-center justify-center rounded-lg">
            <Icon className="size-4" aria-hidden="true" />
          </span>
          {section.title}
        </h2>
        <p className="text-ink-dim m-0 text-sm leading-relaxed">{section.lede}</p>
      </header>

      {section.blocks.map((b, i) => (
        <RuleBlock key={i} block={b} />
      ))}
    </section>
  );
}
