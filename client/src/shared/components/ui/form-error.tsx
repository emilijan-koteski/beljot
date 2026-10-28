import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

/** The one form-level error: a single danger line announced as an alert. Field errors stay under their field. */
export function FormError({
  children,
  testId,
  className,
}: {
  children: ReactNode;
  testId?: string;
  className?: string;
}) {
  return (
    <p
      role="alert"
      data-testid={testId}
      className={cn("text-danger m-0 text-sm font-medium", className)}
    >
      {children}
    </p>
  );
}
