/** Dim levels: `prompt` for a decision while the table plays on, `blocker` once play has stopped, `spotlight` for Capot. */
export type OverlayDim = "prompt" | "blocker" | "spotlight";

const DIM_OPACITY: Record<OverlayDim, { center: number; edge: number }> = {
  prompt: { center: 0.4, edge: 0.5 },
  blocker: { center: 0.56, edge: 0.7 },
  spotlight: { center: 0.66, edge: 0.82 },
};

/** Radial black wash for one dim level, lighter at the centre so the panel reads as lit. */
export function overlayDimBackground(level: OverlayDim): string {
  const { center, edge } = DIM_OPACITY[level];
  return `radial-gradient(ellipse at center, rgba(0,0,0,${center}) 0%, rgba(0,0,0,${edge}) 100%)`;
}
