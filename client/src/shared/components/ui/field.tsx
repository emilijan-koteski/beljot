import * as React from "react";

import { InfoPopover } from "@/shared/components/ui/info-popover";
import { cn } from "@/shared/lib/utils";

type FieldProps = {
  label: string;
  htmlFor?: string;
  hint?: React.ReactNode;
  info?: React.ReactNode;
  infoAriaLabel?: string;
  infoTestId?: string;
  error?: string;
  errorTestId?: string;
  required?: boolean;
  className?: string;
  children: React.ReactNode;
};

/**
 * Form field with a micro-caps label, optional info popover and hint, the input, and an inline error.
 * The label switches to the danger colour when the field has an error.
 */
export function Field({
  label,
  htmlFor,
  hint,
  info,
  infoAriaLabel,
  infoTestId,
  error,
  errorTestId,
  required,
  className,
  children,
}: FieldProps) {
  const hasError = Boolean(error);
  return (
    <div className={cn("flex flex-col gap-1.75", className)}>
      <div className="flex items-baseline justify-between gap-2">
        <span className="inline-flex items-center gap-1.5">
          <label
            htmlFor={htmlFor}
            className={cn(
              "font-mono text-[11px] font-semibold tracking-[1.8px] uppercase transition-colors",
              hasError ? "text-danger" : "text-brass-deep",
            )}
          >
            {label}
            {required && <span className="text-accent ml-1">*</span>}
          </label>
          {info && (
            <InfoPopover text={info} ariaLabel={infoAriaLabel ?? label} testId={infoTestId} />
          )}
        </span>
        {hint && (
          <span className="text-ink-mute text-[11.5px] font-medium tracking-[0.5px] normal-case">
            {hint}
          </span>
        )}
      </div>
      {children}
      {error && (
        <p className="text-danger text-xs leading-[1.4] font-medium" data-testid={errorTestId}>
          {error}
        </p>
      )}
    </div>
  );
}
