import type { CSSProperties, ReactNode } from "react";

interface ClassicPanelProps {
  width?: number | string;
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
 * Felt-gradient dialog shell for every classic overlay, with an optional team-tinted `glowColor` ring.
 * Caps at the viewport in both axes and scrolls its body, so bottom controls always stay reachable.
 */
export function ClassicPanel({
  width = 480,
  title,
  subtitle,
  glowColor,
  bodyPadding,
  className = "",
  style,
  children,
}: ClassicPanelProps) {
  const baseShadow =
    "0 20px 60px rgba(0,0,0,0.7), 0 0 0 4px rgba(201,168,118,0.12), inset 0 1px 0 rgba(201,168,118,0.22)";
  const glowShadow = glowColor ? `0 0 0 2px ${glowColor}88, 0 0 24px ${glowColor}77, ` : "";

  return (
    <div
      className={`flex flex-col overflow-hidden ${className}`}
      style={{
        width,
        maxWidth: "calc(100vw - 2rem)",
        maxHeight: "calc(100vh - 2rem)",
        borderRadius: 14,
        background: PANEL_BG,
        border: "1px solid rgba(201,168,118,0.55)",
        boxShadow: `${glowShadow}${baseShadow}`,
        color: "var(--ink-light)",
        fontFamily: "var(--font-body)",
        ...style,
      }}
    >
      {title && (
        <div
          className="shrink-0 px-4 pt-3.5 pb-2.5 sm:px-6 sm:pt-4.5"
          style={{
            borderBottom: "1px solid rgba(201,168,118,0.22)",
          }}
        >
          <div
            className="text-[17px] sm:text-[20px]"
            style={{
              fontFamily: "var(--font-body)",
              fontWeight: 600,
              letterSpacing: 0.3,
              color: "var(--ink-light)",
            }}
          >
            {title}
          </div>
          {subtitle && (
            <div className="text-[11px] sm:text-[12px]" style={{ opacity: 0.65, marginTop: 2 }}>
              {subtitle}
            </div>
          )}
        </div>
      )}
      <div
        className={`min-h-0 overflow-y-auto ${bodyPadding === undefined ? "p-4 sm:p-6" : ""}`}
        style={bodyPadding === undefined ? undefined : { padding: bodyPadding }}
      >
        {children}
      </div>
    </div>
  );
}
