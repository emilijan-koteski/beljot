import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

interface PanelProps {
  icon?: ReactNode;
  title: ReactNode;
  aside?: ReactNode;
  children: ReactNode;
  className?: string;
  testId?: string;
}

/** A titled surface card for sidebars and page sections: icon, title and an optional trailing note over its content. */
export function Panel({ icon, title, aside, children, className, testId }: PanelProps) {
  return (
    <section
      data-testid={testId}
      className={cn(
        "bg-surface border-border flex flex-col gap-3 rounded-xl border p-4 shadow-xs",
        className,
      )}
    >
      <header className="text-ink flex items-center gap-2 text-sm font-semibold [&>svg]:text-ink-dim [&>svg]:size-4">
        {icon}
        <h2 className="m-0 text-sm font-semibold">{title}</h2>
        {aside !== undefined && <span className="ml-auto text-xs font-semibold">{aside}</span>}
      </header>
      {children}
    </section>
  );
}
