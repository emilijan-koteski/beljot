import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

interface EmptyStateProps {
  icon?: ReactNode;
  title?: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
  compact?: boolean;
  size?: "md" | "sm";
  testId?: string;
  className?: string;
}

/**
 * The one "nothing here yet" state: a dashed card with an optional icon, title and action for page sections
 * (`sm` on the compact mobile type scale), or a single muted line (`compact`) inside lists, menus and dialogs.
 */
export function EmptyState({
  icon,
  title,
  children,
  action,
  compact = false,
  size = "md",
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

  const sm = size === "sm";
  return (
    <div
      className={cn(
        "bg-surface border-border flex flex-col items-center rounded-lg border border-dashed text-center",
        sm ? "gap-2 px-4 py-8" : "gap-3 px-5 py-10",
        className,
      )}
      data-testid={testId}
    >
      {icon && (
        <span
          className={cn(
            "bg-accent-soft text-accent flex items-center justify-center rounded-full",
            sm ? "size-10 [&_svg]:size-5" : "size-14 [&_svg]:size-6",
          )}
        >
          {icon}
        </span>
      )}
      {title && (
        <p className={cn("text-ink m-0 font-semibold", sm ? "text-sm" : "text-lg")}>{title}</p>
      )}
      {children && (
        <p className={cn("text-ink-dim m-0 max-w-90", sm ? "text-xs" : "text-sm")}>{children}</p>
      )}
      {action}
    </div>
  );
}
