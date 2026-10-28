import { cn } from "@/shared/lib/utils";

/** The one loading placeholder: a sunken block that pulses (still under reduced motion), sized by the caller. */
export function Skeleton({ className, testId }: { className?: string; testId?: string }) {
  return (
    <div
      aria-hidden="true"
      data-slot="skeleton"
      data-testid={testId}
      className={cn("bg-surface-sunken rounded-lg motion-safe:animate-pulse", className)}
    />
  );
}
