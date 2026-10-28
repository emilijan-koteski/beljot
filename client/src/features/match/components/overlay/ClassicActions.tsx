import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

import { ClassicButton } from "./ClassicButton";

/** One action in a felt dialog's action row, with the same shape as an AppDialog action. */
export interface ClassicAction {
  label: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  testId?: string;
}

interface ClassicActionsProps {
  primary?: ClassicAction;
  secondary?: ClassicAction;
  tone?: "default" | "danger";
  testId?: string;
  className?: string;
  children?: ReactNode;
}

/**
 * The in-match action row, AppDialog's footer contract on felt: right-aligned, primary first (brass, or red in
 * the danger tone), then the ghost way out, then any custom controls such as a timer-ringed button.
 */
export function ClassicActions({
  primary,
  secondary,
  tone = "default",
  testId,
  className,
  children,
}: ClassicActionsProps) {
  return (
    <div
      data-slot="classic-actions"
      data-testid={testId}
      className={cn("flex flex-wrap items-center justify-end gap-3.5", className)}
    >
      {primary && (
        <ClassicButton
          variant={tone === "danger" ? "danger" : "primary"}
          onClick={primary.onClick}
          disabled={primary.disabled}
          data-testid={primary.testId}
        >
          {primary.label}
        </ClassicButton>
      )}
      {secondary && (
        <ClassicButton
          onClick={secondary.onClick}
          disabled={secondary.disabled}
          data-testid={secondary.testId}
        >
          {secondary.label}
        </ClassicButton>
      )}
      {children}
    </div>
  );
}
