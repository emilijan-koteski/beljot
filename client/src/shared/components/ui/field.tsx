import * as React from "react";

import { InfoPopover } from "@/shared/components/ui/info-popover";
import { cn } from "@/shared/lib/utils";

type FieldProps = {
  label: string;
  htmlFor?: string;
  hint?: React.ReactNode;
  /** A "(?)" popover next to the label — hover-on-desktop / tap-to-pin, for
   *  an explanation that would otherwise have to sit as a permanent hint. */
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
 * Brass-deep micro-caps label + optional (?) info popover + optional
 * right-aligned hint slot + the input + an inline error message. The label
 * flips to `--danger` when the field carries an error. Promoted from
 * features/auth/components/AuthCard.tsx — now the third consumer (Create
 * Room) needs it too.
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
              hasError ? "text-destructive" : "text-brass-deep",
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
        <p className="text-destructive text-xs leading-[1.4] font-medium" data-testid={errorTestId}>
          {error}
        </p>
      )}
    </div>
  );
}
