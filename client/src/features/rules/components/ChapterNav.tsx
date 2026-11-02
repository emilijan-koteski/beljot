import { CHAPTER_ICON } from "@/features/rules/chapterIcons";
import { useRules } from "@/features/rules/RulesContext";
import { cn } from "@/shared/lib/utils";

type Props = {
  activeId: string;
  onJump: (id: string) => void;
  testIdPrefix?: string;
  className?: string;
};

/**
 * The chapter list with icons and scroll-spy highlighting, shared by the desktop rules page and the in-match
 * rules dialog.
 */
export function ChapterNav({ activeId, onJump, testIdPrefix = "toc", className }: Props) {
  const { sections, ui } = useRules();
  return (
    <nav
      aria-label={ui.chaptersLabel}
      data-testid="rules-chapter-index"
      className={cn("flex flex-col gap-1", className)}
    >
      {sections.map((s) => {
        const Icon = CHAPTER_ICON[s.id];
        const active = activeId === s.id;
        return (
          <button
            key={s.id}
            type="button"
            onClick={() => onJump(s.id)}
            aria-current={active ? "true" : undefined}
            data-testid={`${testIdPrefix}-${s.id}`}
            className={cn(
              "flex cursor-pointer items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors",
              active
                ? "bg-accent-soft text-ink"
                : "text-ink-dim hover:bg-surface-sunken hover:text-ink",
            )}
          >
            <Icon className="size-4 shrink-0" aria-hidden="true" />
            {s.label}
          </button>
        );
      })}
    </nav>
  );
}
