import type { ReactNode } from "react";

import { Z } from "@/shared/lib/zLayers";

interface OverlayBackdropProps {
  /** 0..1 opacity multiplier for the radial dim layer. Default 0.55. */
  dim?: number;
  /** z-index of the dim layer. Default {@link Z.PROMPT_DIM}. */
  dimZ?: number;
  /** z-index of the centered panel layer. Default {@link Z.PROMPT}. */
  panelZ?: number;
  children: ReactNode;
}

/**
 * Full-screen radial-dim backdrop used by classic-style overlays. The dim and
 * the centered panel render on two separate z-layers so individual game
 * elements can opt to sit between them — e.g. the active bidder's hand elevates
 * to {@link Z.BIDDER_HAND} during the trump prompt so the player can read their
 * cards while everything else is dimmed/blurred.
 *
 *  • dim   → `dimZ` (default {@link Z.PROMPT_DIM}); captures no pointer events
 *            so clicks reach cards that opted to elevate above it.
 *  • panel → `panelZ` (default {@link Z.PROMPT}); pointer-events-auto inner
 *            wrapper so the dialog itself stays interactive.
 *
 * For a dialog whose root establishes its own stacking context (most do — they
 * carry a single `zIndex: Z.X` on their root), these two values only matter
 * relatively (dim below panel). For the trump prompt the backdrop is rendered
 * untrapped, so the values ARE the global prompt tier and the bidder hand can
 * slot between them.
 *
 * Clicking the dim itself does NOT dismiss the overlay (overlays are dismissed
 * via their own buttons / autoclose rings / Escape handlers).
 */
export function OverlayBackdrop({
  dim = 0.55,
  dimZ = Z.PROMPT_DIM,
  panelZ = Z.PROMPT,
  children,
}: OverlayBackdropProps) {
  const center = dim * 0.8;
  return (
    <>
      <div
        aria-hidden
        className="absolute inset-0 pointer-events-none"
        style={{
          zIndex: dimZ,
          background: `radial-gradient(ellipse at center, rgba(0,0,0,${center}) 0%, rgba(0,0,0,${dim}) 100%)`,
          backdropFilter: "blur(2px)",
          WebkitBackdropFilter: "blur(2px)",
        }}
      />
      <div
        className="absolute inset-0 flex items-center justify-center pointer-events-none"
        style={{ zIndex: panelZ }}
      >
        <div className="pointer-events-auto">{children}</div>
      </div>
    </>
  );
}
