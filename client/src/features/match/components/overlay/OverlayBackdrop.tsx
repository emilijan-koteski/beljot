import type { ReactNode } from "react";

import { Z } from "@/shared/lib/zLayers";

import { type OverlayDim, overlayDimBackground } from "../../lib/overlayDim";

interface OverlayBackdropProps {
  dim?: OverlayDim;
  dimZ?: number;
  panelZ?: number;
  children: ReactNode;
}

/**
 * Full-screen radial dim plus a centered panel on separate z-layers, so elements like the bidder's hand can sit between them.
 * The dim ignores pointer events and never dismisses the overlay.
 */
export function OverlayBackdrop({
  dim = "prompt",
  dimZ = Z.PROMPT_DIM,
  panelZ = Z.PROMPT,
  children,
}: OverlayBackdropProps) {
  return (
    <>
      <div
        aria-hidden
        className="absolute inset-0 pointer-events-none"
        style={{
          zIndex: dimZ,
          background: overlayDimBackground(dim),
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
