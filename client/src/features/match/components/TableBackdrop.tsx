/**
 * Static, pointer-transparent table chrome: wood rim, felt, brass oval and filigree.
 * Expects a `.game-table` ancestor for its CSS tokens.
 */
export function TableBackdrop({ compact = false }: { compact?: boolean }) {
  const outerRx = compact ? 560 : 470;
  const outerRy = compact ? 220 : 310;
  const innerRx = compact ? 552 : 462.95;
  const innerRy = compact ? 217 : 305.35;
  return (
    <div className="pointer-events-none absolute inset-0" data-testid="table-backdrop" aria-hidden>
      <div
        className="absolute hidden rounded-3xl md:block"
        style={{
          inset: 14,
          background:
            "linear-gradient(135deg, var(--wood-base) 0%, var(--wood-light) 20%, var(--wood-dark) 50%, var(--wood-light) 80%, var(--wood-base) 100%)",
          boxShadow:
            "inset 0 0 0 1px var(--brass-55), inset 0 0 30px var(--shade-60), var(--shadow-felt-xl)",
        }}
      />
      <div
        className="absolute inset-0 rounded-none md:inset-9 md:rounded-2xl"
        style={{
          background: `
            radial-gradient(ellipse at 50% 50%, var(--felt-light) 0%, var(--felt-dark) 70%, var(--felt-deep) 100%),
            repeating-linear-gradient(45deg, transparent 0 3px, var(--shade-5) 3px 4px),
            repeating-linear-gradient(-45deg, transparent 0 3px, var(--sheen-2) 3px 4px)
          `,
          backgroundBlendMode: "normal, overlay, overlay",
          boxShadow: "inset 0 0 60px var(--shade-60), inset 0 0 200px var(--shade-25)",
        }}
      />
      <div className="absolute inset-0 md:inset-9">
        <svg
          className="absolute inset-0 h-full w-full"
          viewBox="0 0 1368 828"
          preserveAspectRatio="none"
        >
          <ellipse
            cx="684"
            cy="414"
            rx={outerRx}
            ry={outerRy}
            fill="none"
            stroke="var(--brass-25)"
            strokeWidth="2"
            strokeDasharray="3 6"
          />
          <ellipse
            cx="684"
            cy="414"
            rx={innerRx}
            ry={innerRy}
            fill="none"
            stroke="var(--brass-soft)"
            strokeWidth="1"
          />
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
