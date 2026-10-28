import { useRules } from "@/features/rules/RulesContext";
import { cn } from "@/shared/lib/utils";

type Props = {
  activeId: string;
  onJump: (id: string) => void;
  title?: string;
  testIdPrefix?: string;
  className?: string;
};

/**
 * Table-of-contents sidebar with scroll-spy highlighting, shared by the rules page (sticky, from `lg`)
 * and the in-match rules dialog (from `md`).
 */
export function ChapterIndex({
  activeId,
  onJump,
  title,
  testIdPrefix = "toc",
  className = "sticky top-21 hidden w-55 shrink-0 self-start py-1 lg:flex",
}: Props) {
  const { sections, ui } = useRules();
  return (
    <aside data-testid="rules-chapter-index" className={cn("flex-col gap-1", className)}>
      <div className="text-brass-deep px-3 pb-2 font-mono text-2xs font-semibold tracking-caps uppercase">
        {title ?? ui.tocTitle}
      </div>

      {sections.map((s, i) => {
        const active = activeId === s.id;
        return (
          <button
            key={s.id}
            type="button"
            onClick={() => onJump(s.id)}
            data-testid={`${testIdPrefix}-${s.id}`}
            className={cn(
              "grid cursor-pointer grid-cols-[30px_1fr] items-baseline rounded-lg px-3 py-2 text-left transition-colors",
              active ? "bg-accent-soft" : "hover:bg-surface-sunken",
            )}
          >
            <span
              className={cn(
                "font-mono text-xs font-semibold tracking-widest",
                active ? "text-accent" : "text-ink-mute",
              )}
            >
              {String(i + 1).padStart(2, "0")}
            </span>
            <span
              className={cn(
                "text-sm",
                active ? "text-accent font-semibold" : "text-ink font-medium",
              )}
            >
              {s.label}
            </span>
          </button>
        );
      })}
    </aside>
  );
}
