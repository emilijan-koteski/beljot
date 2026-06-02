import type { CSSProperties, ReactNode } from "react";

interface ClassicPanelProps {
  width?: number | string;
  /** Optional title displayed in the brass-bordered header. */
  title?: ReactNode;
  /** Optional subtitle below the title. */
  subtitle?: ReactNode;
  /**
   * Optional team-color glow stop (gold or silver). When provided the panel
   * gains a 2 px outer ring + 24 px halo in that color — used for trump-taken,
   * declarations, belot reveals, hand-end + match-end overlays.
   */
  glowColor?: string;
  /** Inner padding override for the body (default 24 px). */
  bodyPadding?: number | string;
  className?: string;
  style?: CSSProperties;
  children: ReactNode;
}

const PANEL_BG = "linear-gradient(180deg, rgba(30,60,40,0.98) 0%, rgba(14,40,24,0.98) 100%)";

/**
 * Felt-gradient dialog shell — the unified chrome for every classic-state
 * overlay (bidding, belot, declarations, pause, score, match end, etc.).
 *
 * Optional `glowColor` adds a team-tinted ring around the panel — Gold for
 * "Us" wins/announcements, Silver for "Them" — pinned to the panel's 14 px
 * radius so the halo traces the edge cleanly.
 */
export function ClassicPanel({
  width = 480,
  title,
  subtitle,
  glowColor,
  bodyPadding = 24,
  className = "",
  style,
  children,
}: ClassicPanelProps) {
  const baseShadow =
    "0 20px 60px rgba(0,0,0,0.7), 0 0 0 4px rgba(201,168,118,0.12), inset 0 1px 0 rgba(201,168,118,0.22)";
  const glowShadow = glowColor ? `0 0 0 2px ${glowColor}88, 0 0 24px ${glowColor}77, ` : "";

  return (
    <div
      className={`overflow-hidden ${className}`}
      style={{
        width,
        // Never exceed the viewport on a narrow (phone) screen — the panel
        // shrinks to fit with a small gutter, then caps at `width` on desktop.
        maxWidth: "calc(100vw - 2rem)",
        borderRadius: 14,
        background: PANEL_BG,
        border: "1px solid rgba(201,168,118,0.55)",
        boxShadow: `${glowShadow}${baseShadow}`,
        color: "var(--ink-light, #f5f2e8)",
        fontFamily: "var(--font-body)",
        ...style,
      }}
    >
      {title && (
        <div
          style={{
            padding: "18px 24px 10px",
            borderBottom: "1px solid rgba(201,168,118,0.22)",
          }}
        >
          <div
            style={{
              fontFamily: "var(--font-body)",
              fontSize: 20,
              fontWeight: 600,
              letterSpacing: 0.3,
              color: "var(--ink-light, #f5f2e8)",
            }}
          >
            {title}
          </div>
          {subtitle && <div style={{ fontSize: 12, opacity: 0.65, marginTop: 2 }}>{subtitle}</div>}
        </div>
      )}
      <div style={{ padding: bodyPadding }}>{children}</div>
    </div>
  );
}
