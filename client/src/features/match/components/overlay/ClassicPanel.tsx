import type { CSSProperties, ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

/** Panel widths: `notice` for a reveal card, `dialog` for most dialogs, `wide` for score-heavy ones, `reference` for the rules. */
export type ClassicPanelSize = "notice" | "dialog" | "wide" | "reference";

const PANEL_WIDTH: Record<ClassicPanelSize, number> = {
  notice: 264,
  dialog: 480,
  wide: 520,
  reference: 860,
};

interface ClassicPanelProps {
  size?: ClassicPanelSize;
  title?: ReactNode;
  subtitle?: ReactNode;
  glowColor?: string;
  bodyPadding?: number | string;
  className?: string;
  style?: CSSProperties;
  children: ReactNode;
}

const PANEL_BG = "linear-gradient(180deg, rgba(30,60,40,0.98) 0%, rgba(14,40,24,0.98) 100%)";

/**
 * Felt-gradient shell for every in-match dialog and reveal, with an optional team-tinted `glowColor` ring.
 * Caps at the viewport in both axes and scrolls its body, so bottom controls always stay reachable.
 */
export function ClassicPanel({
  size = "dialog",
  title,
  subtitle,
  glowColor,
  bodyPadding,
  className,
  style,
  children,
}: ClassicPanelProps) {
  const baseShadow =
    "var(--shadow-felt-xl), 0 0 0 4px var(--brass-10), inset 0 1px 0 var(--brass-25)";
  const glowShadow = glowColor
    ? `0 0 0 2px color-mix(in srgb, ${glowColor} 53%, transparent), 0 0 24px color-mix(in srgb, ${glowColor} 47%, transparent), `
    : "";

  return (
    <div
      className={cn(
        "flex max-h-[calc(100vh-2rem)] max-w-[calc(100vw-2rem)] flex-col overflow-hidden rounded-xl",
        className,
      )}
      style={{
        width: PANEL_WIDTH[size],
        background: PANEL_BG,
        border: "1px solid var(--brass-55)",
        boxShadow: `${glowShadow}${baseShadow}`,
        color: "var(--ink-light)",
        ...style,
      }}
    >
      {title && (
        <div
          className="shrink-0 px-4 pt-3.5 pb-2.5 sm:px-6 sm:pt-4.5"
          style={{
            borderBottom: "1px solid var(--brass-25)",
          }}
        >
          <div
            className="text-lg sm:text-xl"
            style={{
              fontWeight: 600,
              color: "var(--ink-light)",
            }}
          >
            {title}
          </div>
          {subtitle && (
            <div className="text-xs sm:text-xs" style={{ opacity: 0.65, marginTop: 2 }}>
              {subtitle}
            </div>
          )}
        </div>
      )}
      <div
        className={cn(
          "min-h-0 flex-auto overflow-y-auto",
          bodyPadding === undefined && "p-4 sm:p-6",
        )}
        style={bodyPadding === undefined ? undefined : { padding: bodyPadding }}
      >
        {children}
      </div>
    </div>
  );
}
