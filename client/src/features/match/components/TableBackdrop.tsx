/**
 * Static table chrome — wood rim, felt surface, dashed brass oval, filigree
 * corners. Pointer-events disabled so HUD/seats/chat layered above receive
 * clicks. Kept stateless and CSS-only so it costs nothing per frame.
 *
 * Tokens come from `.game-table` in index.css; this layer assumes the parent
 * already opted into the scope.
 */
export function TableBackdrop({ compact = false }: { compact?: boolean }) {
  // The oval is stretched to fill the felt (preserveAspectRatio="none"). On a
  // tall phone felt that turns the desktop landscape ellipse into an egg, so
  // widen rx / shorten ry on mobile to pull the top & bottom toward the centre
  // for a rounder boundary. Filigree corners are dropped on phones (they'd
  // float off the re-shaped oval).
  const outerRx = compact ? 560 : 470;
  const outerRy = compact ? 220 : 310;
  const innerRx = compact ? 552 : 462.95;
  const innerRy = compact ? 217 : 305.35;
  return (
    <div className="pointer-events-none absolute inset-0" data-testid="table-backdrop" aria-hidden>
      {/* Wood rim — 5-stop diagonal. Hidden on phones: the "table frame" look
          isn't wanted there, where the felt runs edge-to-edge instead. */}
      <div
        className="absolute hidden rounded-3xl md:block"
        style={{
          inset: 14,
          background:
            "linear-gradient(135deg, var(--wood-base) 0%, var(--wood-light) 20%, var(--wood-dark) 50%, var(--wood-light) 80%, var(--wood-base) 100%)",
          boxShadow:
            "inset 0 0 0 1px rgba(201,168,118,0.5), inset 0 0 30px rgba(0,0,0,0.6), 0 20px 60px rgba(0,0,0,0.7)",
        }}
      />
      {/* Felt surface — radial gradient + cross-hatch overlays. Fills the whole
          screen on phones (no wood rim); insets to sit inside the rim on md+. */}
      <div
        className="absolute inset-0 rounded-none md:inset-9 md:rounded-2xl"
        style={{
          background: `
            radial-gradient(ellipse at 50% 50%, var(--felt-light) 0%, var(--felt-dark) 70%, var(--felt-deep) 100%),
            repeating-linear-gradient(45deg, transparent 0 3px, rgba(0,0,0,0.04) 3px 4px),
            repeating-linear-gradient(-45deg, transparent 0 3px, rgba(255,255,255,0.015) 3px 4px)
          `,
          backgroundBlendMode: "normal, overlay, overlay",
          boxShadow: "inset 0 0 60px rgba(0,0,0,0.55), inset 0 0 200px rgba(0,0,0,0.25)",
        }}
      />
      {/* Brass oval boundary + filigree corners. Wrapped in a sized div
          because <svg> with viewBox preserves its intrinsic aspect ratio
          even under `inset: 0`, which would push the oval off-center. */}
      <div className="absolute inset-0 md:inset-9">
        <svg
          className="absolute inset-0 h-full w-full"
          viewBox="0 0 1368 828"
          preserveAspectRatio="none"
        >
          {/* Outer dashed oval — rx reduced from 540 to 470 so the oval
              tracks closer to the felt's true visual centerline rather than
              hugging the wood rim left/right. */}
          <ellipse
            cx="684"
            cy="414"
            rx={outerRx}
            ry={outerRy}
            fill="none"
            stroke="rgba(201,168,118,0.28)"
            strokeWidth="2"
            strokeDasharray="3 6"
          />
          {/* Inner thin oval — inset 1.5% to read as a double rule */}
          <ellipse
            cx="684"
            cy="414"
            rx={innerRx}
            ry={innerRy}
            fill="none"
            stroke="rgba(201,168,118,0.18)"
            strokeWidth="1"
          />
          {/* Filigree corner glyphs at the four 'corners' of the oval (desktop
              only — they'd float off the re-shaped mobile oval). */}
          {!compact &&
            (
              [
                [150, 120, 0],
                [1218, 120, 90],
                [150, 708, 270],
                [1218, 708, 180],
              ] as const
            ).map(([x, y, rot], i) => (
              <g key={i} transform={`translate(${x}, ${y}) rotate(${rot})`} opacity="0.32">
                <path d="M0 0 Q 18 4, 22 22 Q 4 18, 0 0" fill="var(--brass)" />
              </g>
            ))}
        </svg>
      </div>
    </div>
  );
}
