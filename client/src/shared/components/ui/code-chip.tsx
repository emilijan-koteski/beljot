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
          "inline-flex items-center gap-2 rounded-[10px] border font-mono font-bold transition-[background-color,border-color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/70",
          dense ? "px-2.5 py-1 text-[13px]" : "px-3 py-2 text-[13px]",
          copied
            ? "border-accent bg-accent-soft text-accent-deep"
            : "border-brass bg-surface-elevated text-ink hover:border-accent/50",
          interactive && "cursor-pointer",
          className,
        )}
        style={{ letterSpacing: 2 }}
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
        "inline-flex items-stretch overflow-hidden rounded-[10px] border transition-[background-color,border-color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/70",
        copied
          ? "border-accent bg-accent-soft"
          : "border-brass bg-surface-elevated hover:border-accent/50",
        interactive && "cursor-pointer",
        className,
      )}
    >
      <span
        className={cn(
          "inline-flex items-center font-mono font-semibold uppercase",
          dense ? "px-2 py-1 text-[9.5px]" : "px-2.5 py-2 text-[10px]",
          copied
            ? "bg-transparent text-accent-deep"
            : "bg-brass-soft text-brass-deep border-r border-border",
        )}
        style={{ letterSpacing: 1.6 }}
      >
        {label}
      </span>
      <span
        data-testid={codeTestId}
        className={cn(
          "inline-flex items-center gap-2 font-mono font-bold",
          dense ? "px-2.5 py-1 text-[13px]" : "px-3 py-2 text-[15px]",
          copied ? "text-accent-deep" : "text-ink",
        )}
        style={{ letterSpacing: 2.2 }}
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
