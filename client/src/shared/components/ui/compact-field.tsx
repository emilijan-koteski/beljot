import type { ReactNode } from "react";

import { InfoPopover } from "@/shared/components/ui/info-popover";
import { cn } from "@/shared/lib/utils";

type CompactFieldProps = {
  label: string;
  htmlFor?: string;
  info?: ReactNode;
  infoTestId?: string;
  changed?: boolean;
  meta?: ReactNode;
  error?: string;
  errorTestId?: string;
  className?: string;
  testId?: string;
  children: ReactNode;
};

/**
 * A compact form field: a sentence-case label with an optional brass dot (changed from the standard), "?" note
 * and right-hand meta, then the control and an inline error.
 */
export function CompactField({
  label,
  htmlFor,
  info,
  infoTestId,
  changed = false,
  meta,
  error,
  errorTestId,
  className,
  testId,
  children,
}: CompactFieldProps) {
  return (
    <div
      className={cn("flex min-w-0 flex-col gap-1.5", className)}
      data-testid={testId}
      data-changed={changed ? "true" : undefined}
    >
      <div className="flex min-h-4 items-center gap-1.5">
        <label
          htmlFor={htmlFor}
          className={cn("text-sm font-semibold", error ? "text-danger" : "text-ink")}
        >
          {label}
        </label>
        {changed && <span className="bg-brass size-1.5 shrink-0 rounded-full" aria-hidden="true" />}
        {info && <InfoPopover text={info} ariaLabel={label} testId={infoTestId} />}
        {meta && <span className="text-ink-mute ml-auto text-xs tabular-nums">{meta}</span>}
      </div>
      {children}
      {error && (
        <p className="text-danger m-0 text-xs leading-snug font-medium" data-testid={errorTestId}>
          {error}
        </p>
      )}
    </div>
  );
}
