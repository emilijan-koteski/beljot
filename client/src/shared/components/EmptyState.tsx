import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

interface EmptyStateProps {
  icon?: ReactNode;
  title?: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
  compact?: boolean;
  testId?: string;
  className?: string;
}

/**
 * The one "nothing here yet" state: a dashed card with an optional icon, title and action for page
 * sections, or a single muted line (`compact`) inside lists, menus and dialogs.
 */
export function EmptyState({
  icon,
  title,
  children,
  action,
  compact = false,
  testId,
  className,
}: EmptyStateProps) {
  if (compact) {
    return (
      <p className={cn("text-ink-dim m-0 px-1 py-2 text-xs", className)} data-testid={testId}>
        {children}
      </p>
    );
  }

  return (
    <div
      className={cn(
        "bg-surface border-border flex flex-col items-center gap-3 rounded-lg border border-dashed px-5 py-10 text-center",
        className,
      )}
      data-testid={testId}
    >
      {icon && (
        <span className="bg-accent-soft text-accent flex size-14 items-center justify-center rounded-full [&_svg]:size-6">
          {icon}
        </span>
      )}
      {title && <p className="text-ink m-0 text-lg font-semibold">{title}</p>}
      {children && <p className="text-ink-dim m-0 max-w-90 text-sm">{children}</p>}
      {action}
    </div>
  );
}
