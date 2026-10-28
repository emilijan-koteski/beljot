import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

/** Tone of an explainer strip: `brass` says what an action does, `danger` what it costs. */
export type ExplainerTone = "brass" | "danger";

interface ToneStyle {
  edge: string;
  fill: string;
  ink: string;
  dot: string;
  dotGlow: string;
  highlight: string;
}

const TONE_STYLE: Record<ExplainerTone, ToneStyle> = {
  brass: {
    edge: "var(--brass-40)",
    fill: "linear-gradient(90deg, var(--felt-strip-start), var(--felt-strip-end))",
    ink: "var(--felt-cream)",
    dot: "var(--brass)",
    dotGlow: "var(--brass-55)",
    highlight: "var(--brass-10)",
  },
  danger: {
    edge: "color-mix(in srgb, var(--danger-light) 35%, transparent)",
    fill: "linear-gradient(90deg, var(--felt-strip-danger-start), var(--felt-strip-danger-end))",
    ink: "color-mix(in srgb, var(--danger-light) 25%, var(--ink-light))",
    dot: "var(--danger-light)",
    dotGlow: "color-mix(in srgb, var(--danger-light) 60%, transparent)",
    highlight: "color-mix(in srgb, var(--danger-light) 14%, transparent)",
  },
};

interface ExplainerStripProps {
  tone?: ExplainerTone;
  className?: string;
  children: ReactNode;
}

/** One-line note under a dialog's body copy, led by a glowing dot in the strip's tone. */
export function ExplainerStrip({ tone = "brass", className, children }: ExplainerStripProps) {
  const style = TONE_STYLE[tone];
  return (
    <div
      role="note"
      data-tone={tone}
      className={cn("flex items-center gap-2 rounded-lg px-3 py-2 text-xs", className)}
      style={{
        border: `1px solid ${style.edge}`,
        background: style.fill,
        color: style.ink,
        boxShadow: `inset 0 1px 0 ${style.highlight}`,
      }}
    >
      <span
        aria-hidden
        className="size-1.75 shrink-0 rounded-full"
        style={{ background: style.dot, boxShadow: `0 0 6px ${style.dotGlow}` }}
      />
      {children}
    </div>
  );
}
