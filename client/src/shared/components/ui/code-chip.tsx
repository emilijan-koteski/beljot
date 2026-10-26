import { Check, Copy, KeyRound } from "lucide-react";

import { cn } from "@/shared/lib/utils";

type CodeChipProps = {
  code: string;
  label?: string;
  variant?: "labeled" | "compact";
  dense?: boolean;
  copied?: boolean;
  onCopy?: () => void;
  ariaLabel?: string;
  className?: string;
  testId?: string;
  codeTestId?: string;
};

/**
 * Room-code chip with a label half and a monospace value half, or a single compact pill.
 * Click-to-copy when onCopy is supplied; `copied` switches it to the accent tone.
 */
export function CodeChip({
  code,
  label = "Code",
  variant = "labeled",
  dense,
  copied = false,
  onCopy,
  ariaLabel,
  className,
  testId,
  codeTestId,
}: CodeChipProps) {
  const interactive = Boolean(onCopy);
  const Comp = interactive ? "button" : "div";

  if (variant === "compact") {
    return (
      <Comp
        {...(interactive
          ? { type: "button" as const, onClick: onCopy, "aria-label": ariaLabel }
          : {})}
        data-testid={testId}
        className={cn(
          "inline-flex items-center gap-2 rounded-md border font-mono font-bold tracking-caps transition-[background-color,border-color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/70",
          dense ? "px-2.5 py-1 text-sm" : "px-3 py-2 text-sm",
          copied
            ? "border-accent bg-accent-soft text-accent-deep"
            : "border-brass bg-surface-elevated text-ink hover:border-accent/50",
          interactive && "cursor-pointer",
          className,
        )}
      >
        <KeyRound className={cn("size-3.5", copied ? "text-accent" : "text-brass-deep")} />
        <span data-testid={codeTestId}>{code}</span>
        {interactive && (
          <span className={cn("inline-flex", copied ? "text-accent" : "text-ink-mute")}>
            {copied ? (
              <Check className="size-3.5" strokeWidth={2.4} />
            ) : (
              <Copy className="size-3.5" />
            )}
          </span>
        )}
      </Comp>
    );
  }

  return (
    <Comp
      {...(interactive
        ? { type: "button" as const, onClick: onCopy, "aria-label": ariaLabel }
        : {})}
      data-testid={testId}
      className={cn(
        "inline-flex items-stretch overflow-hidden rounded-md border transition-[background-color,border-color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/70",
        copied
          ? "border-accent bg-accent-soft"
          : "border-brass bg-surface-elevated hover:border-accent/50",
        interactive && "cursor-pointer",
        className,
      )}
    >
      <span
        className={cn(
          "inline-flex items-center font-mono font-semibold tracking-caps uppercase",
          dense ? "px-2 py-1 text-2xs" : "px-2.5 py-2 text-2xs",
          copied
            ? "bg-transparent text-accent-deep"
            : "bg-brass-soft text-brass-deep border-r border-border",
        )}
      >
        {label}
      </span>
      <span
        data-testid={codeTestId}
        className={cn(
          "inline-flex items-center gap-2 font-mono font-bold tracking-caps",
          dense ? "px-2.5 py-1 text-sm" : "px-3 py-2 text-base",
          copied ? "text-accent-deep" : "text-ink",
        )}
      >
        {code}
        {interactive && (
          <span className={cn("inline-flex", copied ? "text-accent" : "text-ink-mute")}>
            {copied ? (
              <Check className="size-3.5" strokeWidth={2.4} />
            ) : (
              <Copy className="size-3.5" />
            )}
          </span>
        )}
      </span>
    </Comp>
  );
}
