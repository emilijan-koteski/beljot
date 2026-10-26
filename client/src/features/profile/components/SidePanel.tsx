import type { ReactNode } from "react";

import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { cn } from "@/shared/lib/utils";

type SidePanelProps = {
  eyebrow: string;
  title: string;
  children: ReactNode;
  className?: string;
  testId?: string;
};

/**
 * Generic parchment card used by every profile sidebar panel (Partner
 * Spotlight, Rivalries, Milestones): brass eyebrow + display title + body, on
 * a hairline-bordered surface.
 */
export function SidePanel({ eyebrow, title, children, className, testId }: SidePanelProps) {
  return (
    <section
      className={cn("bg-surface border-border flex flex-col rounded-lg border", className)}
      data-testid={testId}
    >
      <div className="px-4 pt-3.5 pb-2.5">
        <Eyebrow className="mb-1">{eyebrow}</Eyebrow>
        <h3 className="text-ink m-0 text-base font-semibold">{title}</h3>
      </div>
      <div className="px-4 pt-1 pb-3.5">{children}</div>
    </section>
  );
}
