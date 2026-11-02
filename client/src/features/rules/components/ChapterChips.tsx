import { useEffect, useRef } from "react";

import { CHAPTER_ICON } from "@/features/rules/chapterIcons";
import { useRules } from "@/features/rules/RulesContext";
import { Chip } from "@/shared/components/ui/chips";
import { cn } from "@/shared/lib/utils";

type Props = {
  activeId: string;
  onJump: (id: string) => void;
  innerClassName?: string;
};

const CHIP_GUTTER_PX = 16;

/**
 * Phone and tablet chapter chips, pinned under the top bar; the chip of the chapter on screen is active and
 * kept scrolled into view.
 */
export function ChapterChips({ activeId, onJump, innerClassName }: Props) {
  const { sections, ui } = useRules();
  const rowRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const row = rowRef.current;
    const chip = row?.querySelector<HTMLElement>(`[data-chapter="${activeId}"]`);
    if (!row || !chip) return;
    row.scrollTo?.({ left: chip.offsetLeft - CHIP_GUTTER_PX, behavior: "smooth" });
  }, [activeId]);

  return (
    <nav
      aria-label={ui.chaptersLabel}
      data-testid="rules-chapter-chips"
      className="bg-background/95 border-border sticky top-15 z-10 border-b backdrop-blur"
    >
      <div
        ref={rowRef}
        className={cn(
          "flex gap-1.5 overflow-x-auto py-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
          innerClassName,
        )}
      >
        {sections.map((s) => {
          const Icon = CHAPTER_ICON[s.id];
          const active = activeId === s.id;
          return (
            <Chip
              key={s.id}
              size="sm"
              active={active}
              icon={<Icon aria-hidden="true" />}
              onClick={() => onJump(s.id)}
              aria-current={active ? "true" : undefined}
              data-chapter={s.id}
              data-testid={`rules-chip-${s.id}`}
            >
              {s.label}
            </Chip>
          );
        })}
      </div>
    </nav>
  );
}
