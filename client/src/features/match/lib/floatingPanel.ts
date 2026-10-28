import type { CSSProperties } from "react";

/** Edge of a floating table panel: a brass hairline by default, full brass for seat announcements, danger for errors. */
export type FloatingPanelEdge = "hairline" | "announce" | "danger";

const EDGE_COLOR: Record<FloatingPanelEdge, string> = {
  hairline: "var(--brass-40)",
  announce: "var(--brass)",
  danger: "color-mix(in srgb, var(--danger-light) 40%, transparent)",
};

const PANEL_FILL = "var(--panel-dark)";
const PANEL_BLUR = "blur(8px)";

interface FloatingPanelOptions {
  edge?: FloatingPanelEdge;
  menu?: boolean;
}

/** The one surface for anything that floats over the felt: HUD panels and buttons, seat banners, menus and the error toast. */
export function floatingPanelStyle({
  edge = "hairline",
  menu = false,
}: FloatingPanelOptions = {}): CSSProperties {
  return {
    background: PANEL_FILL,
    border: `1px solid ${EDGE_COLOR[edge]}`,
    boxShadow: menu ? "var(--shadow-felt-lg)" : "var(--shadow-felt-md)",
    backdropFilter: PANEL_BLUR,
    WebkitBackdropFilter: PANEL_BLUR,
  };
}
