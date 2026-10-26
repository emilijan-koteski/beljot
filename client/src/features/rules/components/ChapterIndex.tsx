import { useRules } from "@/features/rules/RulesContext";
import { cn } from "@/shared/lib/utils";

type Props = {
  activeId: string;
  onJump: (id: string) => void;
};

/**
 * Sticky table-of-contents sidebar with scroll-spy highlighting. Hidden on
 * narrow screens (see RulesPage's responsive container).
 */
export function ChapterIndex({ activeId, onJump }: Props) {
  const { sections, ui } = useRules();
  return (
    <aside
      data-testid="rules-chapter-index"
      className="sticky top-21 hidden w-55 shrink-0 flex-col gap-1 self-start py-1 lg:flex"
    >
      <div
        className="font-mono px-3 pb-2"
        style={{
          fontSize: 10,
          letterSpacing: "0.18em",
          textTransform: "uppercase",
          color: "var(--brass-deep)",
          fontWeight: 600,
        }}
      >
        {ui.tocTitle}
      </div>

      {sections.map((s, i) => {
        const active = activeId === s.id;
        return (
          <button
            key={s.id}
            type="button"
            onClick={() => onJump(s.id)}
            data-testid={`toc-${s.id}`}
            className={cn(
              "grid cursor-pointer grid-cols-[30px_1fr] items-baseline rounded-lg px-3 py-2 text-left transition-colors",
              active ? "bg-accent-soft" : "hover:bg-surface-sunken",
            )}
          >
            <span
              className="font-mono"
              style={{
                fontSize: 12,
                letterSpacing: "0.1em",
                color: active ? "var(--accent)" : "var(--ink-mute)",
                fontWeight: 600,
              }}
            >
              {String(i + 1).padStart(2, "0")}
            </span>
            <span
              style={{
                fontSize: 14,
                color: active ? "var(--accent)" : "var(--ink)",
                fontWeight: active ? 600 : 500,
              }}
            >
              {s.label}
            </span>
          </button>
        );
      })}
    </aside>
  );
}
